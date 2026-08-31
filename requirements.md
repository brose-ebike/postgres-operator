# Requirements

This document captures the functional and non-functional requirements of the Brose E-Bike
Postgres Operator as they exist today. It is the baseline to validate against during the planned
refactoring: any behavior described here that a refactor would change is a **breaking change**
and must be called out explicitly and agreed on before merging.

## 1. Purpose and scope

- REQ-1.1: The operator manages databases, roles/users, and their permissions on
  **already-running** PostgreSQL servers. It does **not** provision, upgrade, back up, or run
  high-availability failover for Postgres itself (unlike Zalando or Crunchy Data). It never
  creates Pods, StatefulSets, or storage for Postgres.
- REQ-1.2: The operator connects to the target instance over the wire protocol (`database/sql` +
  `lib/pq`) and manages state purely by issuing SQL (`CREATE DATABASE`, `ALTER ROLE`, `GRANT`,
  etc.), driven by the declared state of Kubernetes Custom Resources.
- REQ-1.3: A single running operator instance manages any number of distinct target Postgres
  instances/servers concurrently (multi-instance-per-operator), each described by its own
  `PgInstance` resource. This is the key architectural difference from
  `movetokube/postgres-operator`, which assumes one instance per operator deployment. Refactoring
  must preserve the ability to reconcile many `PgInstance` resources — across namespaces — from
  one manager process.
- REQ-1.4: The operator must work against self-hosted PostgreSQL as well as managed offerings from
  Azure, AWS, and GCP. Azure Flexible Server is the actively tested managed target; Azure Single
  Server is explicitly unsupported (deprecated by Microsoft, incompatible username structure) and
  is not a support goal for the refactor.

## 2. Custom Resource Definitions (API group `postgres.brose.bike/v1`)

### 2.1 `PgProperty` (shared value-sourcing primitive)

- REQ-2.1.1: Every configurable connection attribute is expressed as a `PgProperty`, which
  resolves its value from exactly one of: an inline `value` string, a `ConfigMapKeyRef`
  (namespace-local `ConfigMap` key), or a `SecretKeyRef` (namespace-local `Secret` key, supporting
  both `Data` and `StringData`).
- REQ-2.1.2: Resolution order when multiple sources are set on one property: `value` wins first,
  then `ConfigMapKeyRef`, then `SecretKeyRef`. Setting more than one source is permitted but
  discouraged (documented as "better to avoid").
- REQ-2.1.3: If none of the sources yield a value, resolution fails with a typed
  `MissingPropertyValueError`; callers may supply a fallback default instead of failing
  (`GetPropertyValueWithDefault`).
- REQ-2.1.4: If a referenced `ConfigMap`/`Secret` key is missing, resolution fails with a typed
  `MapEntryNotFoundError`. If the referenced object itself doesn't exist, the underlying
  Kubernetes "not found" error is surfaced.

### 2.2 `PgInstance`

- REQ-2.2.1: Fields (`PgInstanceSpec`, all `PgProperty`): `host` (required), `port` (optional,
  default `5432`), `username` (required, the administrator/superuser-equivalent), `password`
  (required), `database` (optional maintenance database, default `postgres`), `sslMode` (optional,
  default `none`).
- REQ-2.2.2: The operator only reads Secrets/ConfigMaps in the `PgInstance`'s own namespace — no
  cross-namespace secret references.
- REQ-2.2.3: `PgDatabase` and `PgUser` resources reference a `PgInstance` by namespace + name
  (`PgInstanceRef`) — the referenced instance may live in a different namespace than the resource
  referencing it.
- REQ-2.2.4: On reconcile, the operator establishes a connection and actively pings it
  (`TestConnection`), recording success/failure as the `postgres.brose.bike/connected` status
  condition (reasons `ConnectionSucceeded` / `ConnectionFailed`). `PgInstance` reconciliation does
  not itself create/alter any database objects.
- REQ-2.2.5: The connecting user must have superuser-equivalent rights, or at minimum
  `CREATEDB` + `CREATEROLE`/`CREATEUSER` role privileges plus `CONNECT` on the maintenance
  database, since ownership transfer (`ALTER DATABASE ... OWNER TO`) and privilege administration
  require it.

### 2.3 `PgDatabase`

- REQ-2.3.1: Fields (`PgDatabaseSpec`): `instance` (`PgInstanceRef`, required), `deletion`
  (`PgDatabaseDeletion`: `drop` bool, `wait` bool), `extensions` ([]string), `defaultPrivileges`
  ([]`PgDatabaseDefaultPrivileges`), `publicPrivileges.revoke` (bool), `publicSchema.drop` (bool).
- REQ-2.3.2: The Kubernetes object `metadata.name` **is** the Postgres database name — there is no
  separate "database name" field, so renaming the CR does not rename the database (a new CR must
  be created).
- REQ-2.3.3: Reconciliation is idempotent: check-then-act. The database is created only if it does
  not already exist (`IsDatabaseExisting` → `CreateDatabase`); creation is never attempted if it
  already exists (no destructive re-create, no schema diffing/migration).
- REQ-2.3.4: Deletion behavior is declarative and controlled per-resource by
  `deletion.drop`/`deletion.wait`, enforced via a finalizer
  (`postgres.brose.bike/pgdatabase`):
  - `drop: true` — on CR deletion, drop the actual Postgres database (if it exists) before
    removing the finalizer.
  - `drop: false` (default) — deleting the CR never drops the database; the finalizer is removed
    immediately, leaving the Postgres database intact.
  - `wait: true` — the finalizer blocks CR deletion, requeuing, until the database has been
    removed from Postgres by some other means (e.g., a DBA); it does not drop the database itself.
  - This is a deliberate safety mechanism: destructive drops require an explicit opt-in.
- REQ-2.3.5: Extensions listed in `spec.extensions` are ensured present via `CREATE EXTENSION`
  (checked first with `IsDatabaseExtensionPresent`) — idempotent, additive only. There is no
  mechanism to drop an extension that is removed from the spec. Failure to create one extension
  sets a `pgdatabase.postgres.brose.bike/extensions` condition to `False` with reason
  `MissingExtension-<name>` and stops further extension processing for that reconcile.
- REQ-2.3.6: `defaultPrivileges` entries name a schema (`schemaName`) that must already exist in
  the database (reconcile fails with an error if not — the operator does not create
  non-`public` schemas implicitly) and a list of `roles`. For each role, the operator:
  1. Ensures the schema is "usable" by the connecting/admin role (`GRANT CONNECT` +
     `GRANT USAGE ON SCHEMA`) if not already.
  2. Grants the declared `schemaPrivileges` (`USAGE`, `CREATE`) on the schema to the role.
  3. Grants the declared privileges (tables/sequences/functions/types) on all *existing* objects
     of that kind in the schema (`GRANT ... ON ALL <TYPE> IN SCHEMA`).
  4. Sets the same privileges as *default* privileges for objects created in the future
     (`ALTER DEFAULT PRIVILEGES IN SCHEMA ... GRANT ... ON <TYPE> TO ...`).
  - Allowed privilege enums are validated: `SchemaPrivilege` (USAGE, CREATE), `TablePrivilege`
    (SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER), `SequencePrivilege` (SELECT,
    UPDATE, USAGE), `FunctionPrivilege` (EXECUTE), `TypePrivilege` (USAGE).
  - Privilege grants are additive (`GRANT`) — the operator never revokes a previously-granted
    default/schema privilege that is later removed from the spec (no privilege drift correction on
    removal).
- REQ-2.3.7: `publicPrivileges.revoke: true` revokes all privileges from the special `PUBLIC`
  pseudo-role on the database itself and, if the `public` schema exists, all privileges on that
  schema — implementing Postgres least-privilege hardening (PUBLIC otherwise has CONNECT/CREATE by
  default on older Postgres versions). Once revoked, there is no code path to re-grant if the flag
  is later set back to `false`.
- REQ-2.3.8: `publicSchema.drop: true` drops the `public` schema entirely if present. This is
  irreversible via the operator (no re-creation logic) and independent of `publicPrivileges`.
- REQ-2.3.9: Status conditions exposed: `pgdatabase.postgres.brose.bike/exists`,
  `.../extensions`, `.../default-privileges`, plus the shared `postgres.brose.bike/connected`.
- REQ-2.3.10: All Postgres object identifiers (database name, schema name, role name) are
  interpolated into SQL as double-quoted identifiers (`formatQueryObj`); the CRD does not
  currently validate identifier legality beyond the Kubernetes resource name (DNS-1123 subdomain)
  rules already imposed on `metadata.name`.

### 2.4 `PgUser`

- REQ-2.4.1: Fields (`PgUserSpec`): `instance` (`PgInstanceRef`, required), `secret`
  (`PgUserSecret{Name}`, names the target credentials Secret in the same namespace), `databases`
  ([]`PgUserDatabase{Name, Owner *bool, Privileges []DatabasePrivilege}`, `DatabasePrivilege` ∈
  {`CONNECT`, `CREATE`}).
- REQ-2.4.2: `metadata.name` is the Postgres login-role name — same rename caveat as
  REQ-2.3.2.
- REQ-2.4.3: The role is created (`CREATE USER`, i.e. a role with `LOGIN`) only if it does not
  already exist, then a password is generated/reused and pushed via `ALTER USER ... WITH PASSWORD
  ... LOGIN` on every reconcile (so password rotation done externally in the Secret is
  overwritten — the Secret's stored password, not the spec, is authoritative; see REQ-2.4.4).
- REQ-2.4.4: Password lifecycle:
  - On first creation, a random password (24–31 chars, alphanumeric) is generated
    (`pkg/security.GeneratePassword`, using `crypto/rand`) and written into the credentials
    Secret.
  - On subsequent reconciles, the existing password is read back out of the Secret (never
    regenerated) and re-applied to the role, so an operator restart or spec change never rotates
    credentials.
  - The Secret is owned by the `PgUser` CR (`OwnerReference`, controller=true,
    blockOwnerDeletion=true) so it is garbage-collected if the CR is force-deleted outside the
    finalizer flow.
- REQ-2.4.5: The generated Secret's data keys: `host`, `port`, `user`, `password`, and per
  referenced database: `database.<name>.uri`, `database.<name>.connection_string`,
  `database.<name>.jdbc_connection_string`. These must remain stable during refactoring since
  applications consume them directly (CI/CD decoupling use case, REQ-6).
- REQ-2.4.6: Reconciliation only proceeds to grant database ownership/privileges once **all**
  referenced databases exist; otherwise it sets the
  `pguser.postgres.brose.bike/databases` condition to `False` (reason `DatabasesMissing`, message
  lists missing DB names) and requeues quickly (1s) rather than erroring — this is the intended
  mechanism for `PgUser` to wait on a co-deployed `PgDatabase`.
- REQ-2.4.7: Per-database privilege reconciliation is state-driven on current vs. desired
  ownership:
  - Desired owner, not current owner → `ALTER DATABASE ... OWNER TO` (via temporary role
    membership grant/revoke pattern, `runAs`).
  - Currently owner, no longer desired as owner → ownership is reset back to the connecting
    admin/instance role (`ResetDatabaseOwner`).
  - If the user is not (and should not be) the owner, `Privileges` (`CONNECT`/`CREATE`) are
    applied via `UpdateDatabasePrivileges`, which first revokes **all** privileges then grants
    exactly the declared set (full replace, not incremental).
  - If the user **is** the owner, per-database `Privileges` are not separately applied (an owner
    implicitly has full rights).
- REQ-2.4.8: Deletion (finalizer `postgres.brose.bike/pgloginrole`): if the role exists,
  `DeleteRole` first reassigns all objects owned by the role to the connecting admin role
  (`REASSIGN OWNED BY ... TO ...`), drops all remaining privileges owned by the role (`DROP OWNED
  BY`), then drops the role (`DROP USER`) — this must not fail merely because the role still owns
  objects. The credentials Secret is then explicitly deleted (not left to Kubernetes GC), and the
  finalizer is removed.
- REQ-2.4.9: Status conditions exposed: `pguser.postgres.brose.bike/exists`,
  `.../databases`, plus the shared `postgres.brose.bike/connected`.

## 3. Reconciliation semantics (cross-cutting)

- REQ-3.1: Every reconciler follows: fetch resource → resolve referenced `PgInstance` → build a
  `pgapi` client → branch on `DeletionTimestamp` (finalizer path) → reconcile desired state →
  update status conditions → ensure finalizer present.
- REQ-3.2: Errors during reconciliation return `ctrl.Result{RequeueAfter: time.Minute}` (fixed
  backoff, not exponential) rather than failing permanently — the operator is expected to keep
  retrying indefinitely against a transient outage (e.g. instance restart, network blip).
- REQ-3.3: Status conditions use standard Kubernetes `metav1.Condition` semantics
  (Type/Status/Reason/Message/ObservedGeneration) and are only written when the condition actually
  changes (`meta.IsStatusConditionPresentAndEqual` short-circuit), to avoid status-update
  reconcile storms.
- REQ-3.4: The three reconcilers (`PgInstanceReconciler`, `PgDatabaseReconciler`,
  `PgUserReconciler`) are independently registered controllers watching their own CRD type only —
  a change to a `PgInstance` does not automatically re-trigger reconciliation of `PgDatabase`/
  `PgUser` resources that reference it (no owning/watches relationship today). This is a known gap
  a refactor may choose to address, but doing so is not required for backwards compatibility.
- REQ-3.5: Each reconciler obtains its Postgres client through an injectable factory function
  (`PgDatabaseAPIFactory` / `PgRoleAPIFactory` / `PgConnectionFactory`), decoupling controller
  logic from the concrete `pgapi`/`services` implementation — this seam must be preserved so
  reconciler logic remains unit-testable without a real Postgres instance.
- REQ-3.6: A fresh `pgapi` client (and thus a fresh set of SQL connections) is built once per
  `Reconcile()` call — connections are not pooled/reused across reconciles, and are explicitly torn
  down when the passed-in context is cancelled.

## 4. Postgres access layer (`pkg/pgapi`)

- REQ-4.1: All connection parameters are assembled into a `lib/pq` DSN string
  (`PgConnectionString.toString()`); `port` is only included in the DSN when it differs from 5432,
  and any other empty field is simply omitted (relying on `lib/pq`/libpq defaults).
- REQ-4.2: Administrative operations (create/drop database, create/drop role, extensions) run
  against the instance's configured maintenance database using the long-lived connection; any
  operation that must act *inside* a specific target database opens a **new** short-lived
  connection scoped to that database (`runIn`/`runInAs`) because Postgres has no cross-database
  queries.
- REQ-4.3: `runAs`/`runInAs` implement a "borrow membership" pattern: if the connecting role is not
  already a member of the target role, it is granted membership for the duration of the callback
  and revoked immediately after (including on panic, via `recover`+re-panic in `runInAs`) — this
  lets a superuser-like admin role act on objects it doesn't directly own, without ever
  permanently altering role membership. This pattern must be preserved for any refactor that
  touches ownership/privilege operations, since it's the mechanism that avoids requiring
  `pg_read_all_data`/blanket superuser workarounds.
- REQ-4.4: All object identifiers (database/schema/role names) passed into SQL statements are
  quoted with double quotes via `formatQueryObj`; string *literals* (e.g. passwords) are escaped by
  naive single-quote doubling/backslash-escaping (`UpdateUserPassword`) rather than parameterized
  queries — this is a known SQL-construction pattern to preserve behaviorally but treat as a
  hardening candidate (see §8).
- REQ-4.5: Privilege and object-type arguments are validated against fixed allow-lists before being
  interpolated into SQL (`validatePrivileges`, `validateTypeName`, `UpdateDatabasePrivileges`'s own
  list) — any refactor must keep rejecting values outside these enums with a typed
  `IllegalArgumentError` rather than passing them through to SQL.
- REQ-4.6: `pgapi` exposes narrow, composable interfaces (`PgConnector`, `PgRoleAPI`,
  `PgDatabaseAPI`, `PgSchemaAPI`) that together form `PgInstanceAPI`; controllers must depend only
  on the narrowest composition they need (see `controllers/factories.go`'s `PgDatabaseAPI` /
  `PgRoleAPI` aliases) so that mocking/testing stays cheap.

## 5. Multi-instance operation (differentiator vs. `movetokube/postgres-operator`)

- REQ-5.1: One operator deployment (one manager process, optionally leader-elected) reconciles an
  arbitrary number of `PgInstance` resources across namespaces, each potentially pointing at a
  different physical Postgres server (different vendor, version, credentials).
- REQ-5.2: `PgDatabase`/`PgUser` resources are bound to a specific instance via `PgInstanceRef`;
  the same database/user *name* may exist independently on multiple instances without collision,
  since Kubernetes resource identity (namespace+name of the CR) is separate from the Postgres
  object it manages.
- REQ-5.3: A failure connecting to one `PgInstance` (e.g., network partition to one managed
  server) must not block reconciliation of `PgDatabase`/`PgUser` resources tied to other,
  healthy instances — each reconcile is scoped to a single resource and its own instance
  connection.
- REQ-5.4: There is no shared connection pool or cache across reconciles/instances today (see
  REQ-3.6) — this is a performance characteristic to keep in mind (not a hard requirement to
  preserve) if the refactor introduces pooling for efficiency.

## 6. Vendor / Azure support

- REQ-6.1: The operator must support self-hosted PostgreSQL and managed PostgreSQL from Azure
  (Flexible Server), AWS, and GCP, connecting purely over the standard Postgres wire protocol —
  no vendor SDKs or cloud-provider APIs are used to manage the database/role objects themselves.
- REQ-6.2: Azure Database for PostgreSQL **Flexible Server** is the actively supported and
  regularly-tested managed offering.
- REQ-6.3: Azure Database for PostgreSQL **Single Server** is explicitly out of scope/unsupported
  because its login username must be suffixed with `@<server-name>` — a structural incompatibility
  with how this operator derives/uses role names (the CR's `metadata.name` *is* the role name, see
  REQ-2.4.2, with no room for a vendor-specific suffix). Single Server is deprecated by Microsoft;
  do not add special-casing for it.
- REQ-6.4: `sslMode` is configurable per `PgInstance` (default `none`) to satisfy managed providers
  that require/enforce TLS (Azure Flexible Server, AWS RDS, GCP Cloud SQL commonly require
  `require`/`verify-full`).
- REQ-6.5: Any refactor must keep at least one automated or documented verification path against
  Azure Flexible Server (today: manual/regular testing per `docs/usage/azure.md`), since it is a
  named production target, not just a theoretical compatibility claim.

## 7. Security model

- REQ-7.1: The operator requires one administrative credential per `PgInstance`, ideally superuser
  or at least `CREATEDB`+`CREATEROLE`/`CREATEUSER`+`CONNECT` (see REQ-2.2.5). It never expects or
  requests per-tenant elevated credentials — all lower-privileged role/database work is done by
  temporarily borrowing role membership (REQ-4.3), not by requiring superuser on every operation.
- REQ-7.2: Generated login-role passwords are cryptographically random
  (`crypto/rand`-backed), 24–31 characters, drawn from `[0-9a-zA-Z]` — no symbols. This should stay
  compatible with the widest range of client drivers/connection-string encodings; if the refactor
  changes the charset it must remain URL-safe (used directly, unescaped, in
  `database.<name>.connection_string`/`.uri` — see REQ-2.4.5).
- REQ-7.3: Credentials are only ever persisted in Kubernetes Secrets (never ConfigMaps, never CR
  spec/status fields, never logs) — `PgInstance` admin credentials are read via `SecretKeyRef`/
  `ConfigMapKeyRef` (ConfigMap is permitted for non-secret fields like host/port but the docs
  recommend Secrets for username/password), and `PgUser` role credentials are written to a
  Secret named by `spec.secret.name`.
- REQ-7.4: RBAC granted to the operator's ServiceAccount is scoped to: full CRUD on the three CRDs
  + their `/status` and `/finalizers` subresources, and `get;list;watch` on core `Secret`/
  `ConfigMap` (plus `create;update;patch;delete` on `Secret`, needed only for `PgUser`'s generated
  credentials Secret). It must not require broader cluster-wide permissions.
- REQ-7.5: `publicPrivileges.revoke` and `publicSchema.drop` (REQ-2.3.7/2.3.8) exist specifically
  to let operators harden a database against the Postgres default of `PUBLIC` having implicit
  `CONNECT`/`CREATE`/schema-usage rights — least-privilege by default is a design goal for new
  databases even though the flags themselves default to `false` for backwards compatibility.

## 8. Concrete use cases (acceptance scenarios)

- REQ-8.1 — **Automated database lifecycle**: Applying a `PgDatabase` CR results in a real
  `CREATE DATABASE` on the target instance; deleting the CR triggers `DROP DATABASE` only when
  `deletion.drop: true` is set, otherwise the Postgres database survives CR deletion (REQ-2.3.4).
- REQ-8.2 — **Role and credential synchronization**: Applying a `PgUser` CR results in a real login
  role and a populated Kubernetes Secret containing the working password and ready-to-use
  connection strings, with no manual DBA step (REQ-2.4.3–2.4.5).
- REQ-8.3 — **Privilege grants and access control**: Declaring `defaultPrivileges` on a
  `PgDatabase` and `databases[].privileges`/`owner` on a `PgUser` results in the corresponding
  `GRANT`/`REVOKE`/`ALTER DEFAULT PRIVILEGES`/ownership-transfer SQL, including role nesting via
  the borrow-membership pattern (REQ-4.3) and schema/table/sequence/function/type-level
  least-privilege mapping (REQ-2.3.6).
- REQ-8.4 — **Extension provisioning**: Declaring `extensions: [uuid-ossp, pgcrypto, postgis, ...]`
  on a `PgDatabase` results in `CREATE EXTENSION IF NOT EXISTS`-equivalent, idempotent creation of
  each named extension inside that database (REQ-2.3.5). Note: the current implementation issues
  a plain `CREATE EXTENSION` guarded by a prior existence check rather than the SQL
  `IF NOT EXISTS` clause — behaviorally idempotent either way; a refactor may switch to
  `IF NOT EXISTS` directly as long as the exists-check short-circuit / condition reporting
  behavior for an invalid/unavailable extension name is preserved.
- REQ-8.5 — **CI/CD application decoupling**: A CI/CD pipeline can apply a `PgDatabase` and a
  `PgUser` manifest (referencing that database) alongside an application `Deployment` in the same
  pipeline run, with no manual coordination — the `PgUser` reconciler's wait-for-database behavior
  (REQ-2.4.6) makes ordering-independent apply safe, and the resulting Secret is mountable
  immediately by the application Pod once both resources report ready conditions.

## 9. Backwards compatibility constraints for the refactor

- REQ-9.1: The API group/version (`postgres.brose.bike/v1`) and Kind names (`PgInstance`,
  `PgDatabase`, `PgUser`) must not change.
- REQ-9.2: All existing spec fields listed in §2 must continue to be accepted with their current
  JSON field names, types, and default values; no required field may become newly required, and no
  currently-optional field may become mandatory.
- REQ-9.3: Existing status condition `Type` strings (`postgres.brose.bike/connected`,
  `pgdatabase.postgres.brose.bike/exists`, `.../extensions`, `.../default-privileges`,
  `pguser.postgres.brose.bike/exists`, `.../databases`) must keep their exact string values, since
  external tooling (dashboards, `kubectl wait --for=condition=...`, ArgoCD health checks) may
  depend on them.
- REQ-9.4: Finalizer string values (`postgres.brose.bike/pgdatabase`,
  `postgres.brose.bike/pgloginrole`) must not change, so upgrades don't strand existing resources
  with an unrecognized finalizer.
- REQ-9.5: Generated Secret key names for `PgUser` (REQ-2.4.5) must not change or be removed;
  new keys may be added additively.
- REQ-9.6: Deletion-safety defaults must not change: a bare-minimum `PgDatabase`/`PgUser` CR
  (all deletion/privilege flags left at zero value) must keep behaving as non-destructively as it
  does today (`drop: false` never drops data; privilege revocation flags never default to `true`).
- REQ-9.7: Where the refactor must break compatibility (e.g. to fix the SQL-escaping pattern in
  REQ-4.4, or to add instance-change-triggered re-reconciliation from REQ-3.4), it must be called
  out as an explicit, reviewed exception to this document rather than an incidental side effect.

## 10. Non-functional / operational requirements

- REQ-10.1: Ships as a single controller-manager binary (Kubebuilder v3 scaffold), packaged as a
  container image, installable via a static `install.yaml`, Kustomize, Helm, or an OLM bundle.
- REQ-10.2: Exposes standard `/healthz` and `/readyz` probes and an authenticated/authorized
  metrics endpoint (`:8443`, TLS, protected via controller-runtime's built-in
  auth/authz metrics filter) for Prometheus-style scraping.
- REQ-10.3: Supports optional leader election (`--leader-elect`) for HA operator deployments
  (multiple operator Pods, one active) — orthogonal to and not to be confused with multi-instance
  support (REQ-5): leader election is about the operator's own HA, not about how many Postgres
  targets it manages.
- REQ-10.4: Every reconciler's Postgres dependency is swappable via a factory function (REQ-3.5),
  and the `pgapi` package is independently tested against real Postgres via testcontainers
  (`pkg/tcpostgres`) — both testing seams must survive the refactor so `controllers`/`api/v1`/
  `pkg/services` tests can keep running under `envtest` without Docker, while `pkg/pgapi` tests
  keep exercising real SQL semantics.
- REQ-10.5: CRD/RBAC/webhook manifests are generated (`controller-gen`) and must stay committed
  and in sync with the Go types (`make manifests`, `make generate`); CI fails the build if
  generated output doesn't match committed output.

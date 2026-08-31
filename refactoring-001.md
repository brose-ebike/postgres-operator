# Refactoring 001: Temporary role assignment and connection usage

**Status:** Proposal for review
**Scope:** `pkg/pgapi/general.go` (`runAs`, `runInAs`, `runIn`), `pkg/pgapi/connection.go`
(`newConnection`), `pkg/pgapi/database.go` (`UpdateDatabaseOwner`)
**Related requirements:** REQ-3.6, REQ-4.2, REQ-4.3, REQ-4.6, REQ-5.4, REQ-7.1, REQ-9.7

This started as a review of the grant/revoke "borrow membership" pattern (§2-§3). Reading the
connection-handling code it sits on top of surfaced two connection leaks that matter more for
connection count than the grant/revoke mechanism itself, so this revision leads with those (§1) and
carries a connection-count lens through the rest of the proposal (§4).

## 1. Connection usage: what we actually open today, and two leaks

Per REQ-3.6, one `pgInstanceAPIImpl` (and its one long-lived `s.instance *sql.DB`, connected to the
instance's maintenance database) is built fresh per `Reconcile()` call and torn down when the
passed-in context is cancelled (`pkg/pgapi/connection.go:71-79`, wired to `ctx.Done()` in
`NewPgInstanceAPI`, `pkg/pgapi/general.go:52-57`). That single long-lived pool is correctly closed.

Everything built *on top of* that pool is not:

### 1.1 `newConnection()` — every call leaks a checked-out connection

```go
// pkg/pgapi/connection.go:99-106
func (s *pgInstanceAPIImpl) newConnection() (*sql.Conn, error) {
    if !s.IsConnected() {
        return nil, errors.New("Missing Connection, unable to execute query")
    }
    return s.instance.Conn(s.ctx)
}
```

`database/sql`'s own docs are explicit: *"Every Conn must be returned to the database pool after
use by calling Conn.Close."* None of the 11 call sites do that —
`IsDatabaseExisting`/`CreateDatabase`/`DeleteDatabase`/`UpdateDatabaseOwner`/
`UpdateDatabasePrivileges`/`GetDatabaseOwner`/`ResetDatabaseOwner` in `database.go`, and
`IsRoleExisting`/`CreateRole`/`DeleteRole`/`UpdateUserPassword` in `role.go`, all call
`conn, err := s.newConnection()` and never call `conn.Close()`. A connection obtained this way and
never closed is never returned to `s.instance`'s pool, so it can never be reused — each call opens
a genuinely new physical connection to the maintenance database (a new TCP connection and a new
Postgres backend process) rather than borrowing an idle one. Since `s.instance.Close()` (called from
`disconnect()`) only closes connections that are currently idle in its pool, connections that were
checked out and never returned aren't closed by it either — they leak for the life of the process,
cleaned up only when Postgres itself times them out server-side (not something we configure) or the
operator restarts.

A single `PgUser` reconcile that checks role existence, creates the role, sets the password, and
grants/revokes ownership on N databases already calls `newConnection()` multiple times — each one a
separate, never-reused, never-closed physical connection.

### 1.2 `runIn`/`runInAs` — every call leaks an entire connection pool

```go
// pkg/pgapi/general.go:113-142 (runIn; runInAs at :144-209 is the same shape)
db, err := sql.Open("postgres", conStr.toString())   // new *sql.DB = new pool, per call
...
conn, err := db.Conn(ctx)
...
err = runner(ctx, conn)
if err := conn.Close(); err != nil {   // returns conn to db's own idle pool
    return err
}
return err
// db itself is never .Close()'d
```

`conn.Close()` here only returns the connection to *that call's own, freshly-created* `*sql.DB`
pool — it does not close the physical connection. Since `db` is a local variable that goes out of
scope right after, the pool (holding one idle connection) becomes unreachable. `database/sql` has
no finalizer that closes abandoned `*sql.DB` pools, so the physical connection stays open,
invisible to the rest of the program, until the process exits or Postgres closes it server-side.

This is called from `IsDatabaseExtensionPresent`, `CreateDatabaseExtension`,
`IsSchemaInDatabase`, `CreateSchema`, `DeleteSchema`, `UpdateSchemaPrivileges`,
`UpdatePrivilegesOnAllObjects`, `UpdateDefaultPrivileges`, `DeleteAllPrivilegesOnSchema`,
`IsSchemaUsable`, `MakeSchemaUseable`, `GetSchemaOwner` — i.e., most of `PgDatabaseAPI`/
`PgSchemaAPI`. A single `PgDatabase` reconcile with a handful of extensions and one
`defaultPrivileges` entry covering a couple of roles across tables/sequences/functions easily makes
8-15 of these calls, every one opening (and leaking) its own physical connection to that database —
even though they're all targeting the *same* database, sequentially, within the same reconcile, and
could share one.

### 1.3 Net effect

Connection count today isn't just "one fresh connection per reconcile" (REQ-3.6's stated model) —
it's one leaked physical connection per `newConnection()`/`runIn`/`runInAs` *call*, most of which
target the same one or two databases per reconcile. On a cluster reconciling many `PgDatabase`/
`PgUser` resources on a fixed interval, this accumulates: connection count against each target
Postgres instance grows roughly linearly with total reconcile calls made since the operator process
last restarted, not with the number of CRs or reconciles in flight. This is a real risk of hitting
`max_connections` on the target instance over time, independent of anything about the grant/revoke
mechanism.

## 2. The grant/revoke pattern itself

Whenever the operator's admin/connecting role needs to act on an object it doesn't own (transfer
database ownership, drop a role's owned objects, grant schema/table privileges on behalf of a
database owner), it temporarily borrows membership in the target role, does the work, then revokes
it — `runAs` (`general.go:86`, used by `DeleteDatabase`, `ResetDatabaseOwner`, `DeleteRole`) and
`runInAs` (`general.go:144`, used by `UpdateSchemaPrivileges`, `UpdatePrivilegesOnAllObjects`,
`MakeSchemaUseable`). This is REQ-4.3's documented mechanism for avoiding a blanket-superuser
requirement, and it's why the same code path works unmodified across self-hosted Postgres and every
managed provider in REQ-1.4/REQ-6.1 without vendor special-casing.

### 2.1 Two problems found while reading this code, independent of §1 and of the movetokube research

- **`UpdateDatabaseOwner` (`pkg/pgapi/database.go:93-115`) reimplements the pattern ad hoc instead
  of calling `runAs`, and leaks the grant on error.** It grants `roleName` to the admin, runs
  `ALTER DATABASE ... OWNER TO`, then revokes — but if the `ALTER DATABASE` statement itself fails
  (line 106-110), the function returns immediately *without revoking the grant issued two lines
  earlier*. The admin role is left permanently a member of `roleName`. This is reachable from the
  real reconcile loop (`internal/controller/pguser_controller.go`, REQ-2.4.7's ownership-transfer
  branch) any time `ALTER DATABASE ... OWNER TO` fails transiently — exactly the kind of failure
  REQ-3.2's "retry indefinitely" model expects to happen routinely. Every other ownership/privilege
  mutation goes through `runAs`/`runInAs`, which always attempt the revoke regardless of the
  runner's result; this one doesn't, silently violating REQ-4.3's "without ever permanently
  altering role membership" guarantee.
- **`runAs` has no panic protection; `runInAs` does.** `runInAs` wraps its revoke in a
  `defer`+`recover`+re-panic (`general.go:177-189`) specifically so a panicking runner still gets
  its membership revoked before the panic propagates. `runAs` has no such guard — a panic inside
  any of its three callers skips the revoke entirely. Same policy, different guarantees.

## 3. How movetokube/postgres-operator handles this

Read `pkg/postgres/{postgres,database,role,aws,azure,gcp}.go` from
[movetokube/postgres-operator](https://github.com/movetokube/postgres-operator) (master branch).

**Short answer: it doesn't have a better mechanism — it mostly doesn't do this at all, and where it
has to, it's less careful than what we already do. It also doesn't do anything different on
connection reuse — its per-database calls (`GetConnection`, `pkg/postgres/postgres.go`) open a new
`*sql.DB` per call the same way our `runIn`/`runInAs` do, just with the `Close()` this project
happens to call correctly, so it doesn't share our leak, but it doesn't reuse connections across
calls either.**

### 3.1 Base implementation: assumes true superuser, no borrowing

The default `pg` struct issues ownership/privilege changes directly on the long-lived admin
connection, with no grant/revoke around them at all:

```go
// database.go
func (c *pg) AlterDatabaseOwner(dbname, owner string) error {
    _, err := c.db.Exec(fmt.Sprintf(ALTER_DB_OWNER, dbname, owner))
    return err
}
```

```go
// database.go — SetSchemaPrivileges also just GRANTs directly, no role-borrowing
_, err = tmpDb.Exec(fmt.Sprintf(GRANT_ALL_TABLES, schemaPrivileges.Privs, schemaPrivileges.Schema, schemaPrivileges.Role))
```

This only works because Postgres superusers bypass ownership/membership checks entirely — there is
no Postgres primitive being used here that we aren't already using (no `SET ROLE`, no
`SET SESSION AUTHORIZATION`, no transaction tricks); it's just skipped, on the assumption the admin
credential is a real superuser. That assumption is exactly what REQ-2.2.5 explicitly does *not*
require of us, and REQ-7.1 states we deliberately avoid requiring superuser on every operation.

### 3.2 Managed providers: ad hoc, provider-specific, and sometimes leaky

Movetokube's admin credential *isn't* superuser on AWS RDS, Azure Flexible Server, or GCP
AlloyDB/Cloud SQL, so those three providers get their own decorator struct
(`awspg`/`azurepg`/`gcppg`) that overrides individual methods to add a grant before the operation
their base implementation can't perform:

```go
// aws.go — granted, never revoked
func (c *awspg) CreateUserRole(role, password string) (string, error) {
    returnedRole, err := c.pg.CreateUserRole(role, password)
    if err != nil {
        return "", err
    }
    // On AWS RDS the postgres user isn't really superuser ... he doesn't have permissions
    // to ALTER DEFAULT PRIVILEGES FOR ROLE unless he belongs to the role
    err = c.GrantRole(role, c.user)   // <- no matching RevokeRole, anywhere
    ...
}
```

```go
// azure.go — same shape, same omission
func (azpg *azurepg) CreateDB(dbname, role string) error {
    err := azpg.GrantRole(role, azpg.user)   // <- granted, never revoked
    if err != nil {
        return err
    }
    return azpg.pg.CreateDB(dbname, role)
}
```

```go
// gcp.go — DropRole is the one place that does grant + defer revoke, matching our pattern
err := c.GrantRole(role, c.user)
...
defer c.RevokeRole(role, c.user)
```

So across the three provider decorators: some paths (`gcp.DropRole`, `aws.DropRole`) grant-then-
revoke like we do; several others (`aws.CreateDB`, `aws.CreateUserRole`, `azure.CreateDB`,
`azure.DropRole`) grant and **never revoke** — the admin ends up permanently a member of every group
role it has ever created or touched. None of the decorators check "am I already a member" first the
way our `isMember` guard does, so the same role can accumulate redundant `GRANT` statements across
reconciles (harmless — `GRANT` is idempotent — but further evidence there's no membership-hygiene
invariant being maintained here).

### 3.3 Conclusion

Movetokube trades our "one mechanism, works uniformly everywhere" design for "assume superuser by
default, patch the exceptions per-cloud, with the patches leaking privilege." Adopting either half
of that as our new mechanism would be a regression against REQ-2.2.5/REQ-7.1/REQ-4.3, and would
reintroduce the per-provider special-casing REQ-6.1 explicitly says we avoid. It also offers nothing
for the connection-count question in §1 — its per-database connection handling has the same
one-pool-per-call shape ours does, just without our `Close()` bug.

## 4. Proposal

Ordered by how essential each change is; §4.1-4.2 fix correctness bugs unrelated to connection
count, §4.3-4.4 are where the connection-reduction work lives.

### 4.1 Fix `UpdateDatabaseOwner` to go through `runAs`

Replace the hand-rolled grant/alter/revoke in `pkg/pgapi/database.go:93-115` with a call to
`s.runAs(conn, roleName, func() error { ...alter database... })`, matching `ResetDatabaseOwner`
right below it. Removes the leak-on-error path from §2.1 with no interface or behavior change
visible to callers.

### 4.2 Deduplicate `runAs`/`runInAs` into one shared helper

Extract the "check membership → grant if needed → run callback → revoke if needed, panic-safe"
logic into a single internal helper, e.g. `withBorrowedRole(conn *sql.Conn, role string, runner
func() error) error`. `runAs` becomes a thin wrapper; `runInAs` becomes "get a connection, then call
`withBorrowedRole`". Gives `runAs`'s three callers the same panic-safe revoke `runInAs` already has.
Pure internal refactor — `PgDatabaseAPI`/`PgRoleAPI`/`PgSchemaAPI` are untouched (REQ-4.6).

### 4.3 Always close what we open (fixes §1.1 and §1.2, mandatory, largest lever on connection count)

- Every `newConnection()` call site in `database.go`/`role.go` (11 places) should `defer
  conn.Close()` immediately after the connection is obtained. This alone lets Go's own pool on
  `s.instance` start reusing idle connections (`MaxIdleConns` defaults to 2) across calls within the
  same reconcile instead of opening a fresh physical connection every time — no architectural change
  needed, just closing what we already open.
- `runIn`/`runInAs` should `defer db.Close()` alongside the existing `conn.Close()`, so a
  one-off call at least doesn't leak the whole pool it created. On its own this stops new leaks but
  doesn't enable reuse (see §4.4) since each call still opens and immediately tears down its own
  pool.

This is the correctness floor: whatever else does or doesn't ship from §4.4, this is a bug fix with
no compatibility impact and should happen regardless.

### 4.4 Cache one connection pool per target database for the lifetime of a reconcile

§4.3 alone still opens a new physical connection to each target database on every `runIn`/`runInAs`
call, even against the same database, because each call creates and tears down its own `*sql.DB`.
To actually reduce the *count* of connections opened per reconcile (not just stop leaking them),
cache the per-database `*sql.DB` on `pgInstanceAPIImpl`, keyed by database name, created lazily on
first use:

```go
type pgInstanceAPIImpl struct {
    ...
    instance    *sql.DB
    databases   map[string]*sql.DB   // new: one pool per target database, lazily created
    databasesMu sync.Mutex
}

func (s *pgInstanceAPIImpl) databaseConn(database string) (*sql.DB, error) {
    s.databasesMu.Lock()
    defer s.databasesMu.Unlock()
    if db, ok := s.databases[database]; ok {
        return db, nil
    }
    conStr := s.connectionString.copy()
    conStr.database = database
    db, err := sql.Open("postgres", conStr.toString())
    if err != nil {
        return nil, err
    }
    s.databases[database] = db
    return db, nil
}
```

`runIn`/`runInAs` call `s.databaseConn(database)` instead of `sql.Open` directly, still get a
`*sql.Conn` via `db.Conn(ctx)` per call and still `defer conn.Close()` per call (returning it to
that database's pool for reuse by the *next* call in the same reconcile), but no longer create or
tear down the pool itself each time. All cached pools are closed in `disconnect()` alongside
`s.instance`, using the same `ctx.Done()` teardown goroutine that already exists in
`NewPgInstanceAPI` (`general.go:52-57`) — so the connection lifetime story stays exactly what REQ-3.6
already documents ("built once per Reconcile(), torn down when context is cancelled"), just scoped
per-database-within-a-reconcile instead of per-call.

Concretely, for a `PgDatabase` reconcile with a few extensions and one `defaultPrivileges` block
covering two roles across tables/sequences/functions, this turns roughly 8-15 physical connections
(today's leaked count, per §1.2) into 1 connection to that database, reused for every call in that
reconcile and cleanly closed at the end.

## 5. Explicitly rejected

- **Assume the admin connection is superuser and drop the grant/revoke dance** — breaks
  REQ-2.2.5/REQ-7.1, and stops working on managed providers whose admin role isn't superuser (Azure
  Flexible Server, RDS, GCP), the actively-tested target per REQ-6.2/REQ-6.5.
- **Grant membership once and never revoke it (movetokube's AWS/Azure decorator behavior)** —
  contradicts REQ-4.3/REQ-7.1's least-privilege guarantee.
- **Per-cloud-provider decorator structs** — unnecessary since our mechanism already works
  uniformly; would cut against REQ-1.4/REQ-6.1's "no vendor-specific code" stance.
- **A single connection pool shared across all `PgInstance`s / across reconciles** (the maximal
  version of "reduce connection count") — would improve connection count further, but breaks
  REQ-5.3's isolation guarantee (a problem with one instance's connection must not affect
  reconciliation of resources on other instances) and REQ-3.6's per-reconcile lifecycle, both of
  which REQ-5.4 explicitly leaves as the caller's choice to preserve rather than something safe to
  discard casually. §4.4's per-reconcile, per-database cache gets most of the reduction (repeat calls
  within one reconcile stop each paying for a new connection) without touching either guarantee —
  going further than that is a separate, larger design decision this proposal doesn't make.
- **Wrap grant→operate→revoke in a single SQL transaction, so a crash mid-sequence rolls back the
  grant instead of leaving it permanent.** Considered as a way to harden the pattern beyond what
  §4.1-4.2 do, but it doesn't work: PostgreSQL explicitly forbids executing certain DDL commands
  inside a transaction block, and `DeleteDatabase` (`pkg/pgapi/database.go:79-91`) is exactly this
  case — its `runner()` issues `DROP DATABASE`, which Postgres rejects with
  *"DROP DATABASE cannot run inside a transaction block"* (SQLSTATE `25001`) the moment it's wrapped
  in an explicit `BEGIN`. Since `runAs` is meant to be one uniform helper used across all its
  callers, and at least one caller executes a statement that categorically cannot participate in a
  transaction, there's no way to apply this uniformly — it would need a carve-out for
  `DeleteDatabase` (and for any future caller that happens to run another transaction-incompatible
  statement, a footgun for whoever adds the next `runAs` call site without knowing this). Not
  pursued in any form.

## 6. Compatibility

§4.1-4.3 are internal to `pkg/pgapi`, change no exported interface, SQL statement text, CRD field,
status condition, or finalizer string, and §4.1/§4.3 only change behavior on previously-leaking
paths — no REQ-9 exception needed. §4.4 changes REQ-4.2's "short-lived" connection wording (a
connection scoped to a database now lives for the reconcile, not for a single call) but preserves
its intent — a connection scoped to one target database, opened because Postgres has no
cross-database queries, torn down predictably — and REQ-5.4 already pre-approves exactly this kind
of change ("not a hard requirement to preserve... if the refactor introduces pooling for
efficiency"); it should still be called out explicitly in review per REQ-9.7 since it's the one
behavioral change in this proposal, even though it needs no CRD/API exception.

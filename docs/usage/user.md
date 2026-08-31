# PgUser
## Resource Definition

The `PgUser` resource manages a role with login (user) on the referenced instance.

```yaml
apiVersion: postgres.oebc.tools/v1
kind: PgUser
metadata:
  name: service_user
spec:
  instance:
    namespace: "default"
    name: "instance-001"
  secret:
    name: "service-credentials"
  databases: 
    - name: "service_db"
      owner: true
      privileges: ["CONNECT", "CREATE"]
```

## Attribute Description

| Attribute            | Description                                                                              | Required |
|-----------------------|-------------------------------------------------------------------------------------------|----------|
| `instance.namespace`/`.name` | Reference to the `PgInstance` on which this role should be managed                 | :white_check_mark: |
| `secret.name`         | Name of the k8s Secret (in the same namespace) the operator creates/manages with this role's credentials | :x: at the schema level, but see note below |
| `databases`           | List of databases this role should be able to access, see below                          | :x: |

!!! warning
    Although `secret` is optional in the CRD schema, the operator currently requires it in
    practice: reconciliation fails with a clear error if `secret` is omitted, since the operator
    has no way to store the generated password otherwise. Always set `secret.name`.

### `databases`

| Attribute    | Description                                                                                     | Required |
|---------------|--------------------------------------------------------------------------------------------------|----------|
| `name`        | Name of the database (must already exist, e.g. managed by a `PgDatabase` resource)               | :x: |
| `owner`       | If `true`, the operator makes this role the owner of the database (`ALTER DATABASE ... OWNER TO`). If `false`/unset and the role is currently the owner, ownership is reset back to the instance's admin user. | :x: |
| `privileges`  | `CONNECT`, `CREATE` — granted on the database via `GRANT ... ON DATABASE`                        | :white_check_mark: |

!!! note
    `privileges` is required by the schema but is only ever applied when `owner` is not `true` —
    an owning role already has full rights on the database, so the operator does not additionally
    sync the `privileges` list for it. If the referenced database doesn't exist yet, reconciliation
    is requeued (fast retry) until it does; ownership/privilege sync only runs once every database
    in the list exists.

## Generated Secret

For every `PgUser`, the operator creates (and keeps up to date) a k8s Secret named `secret.name`
in the same namespace, owned by the `PgUser` resource. Its data keys are:

| Key                                              | Contents                                                                 |
|---------------------------------------------------|---------------------------------------------------------------------------|
| `host`                                            | Hostname resolved from the referenced `PgInstance`                       |
| `port`                                            | Port resolved from the referenced `PgInstance`                           |
| `user`                                            | The role name (same as `metadata.name` of the `PgUser`)                  |
| `password`                                        | The generated password for the role (generated once on creation, kept stable across reconciles) |
| `database.<name>.uri`                             | `<host>:<port>/<name>?sslmode=<mode>` — **no scheme prefix**, not a standalone URI |
| `database.<name>.connection_string`               | `postgres://<user>:<password>@<host>:<port>/<name>?sslmode=<mode>`       |
| `database.<name>.jdbc_connection_string`          | `jdbc:postgresql://<host>:<port>/<name>?sslmode=<mode>` — credentials are **not** embedded; pass `user`/`password` separately to the JDBC driver |

A `database.<name>.*` triplet of keys is generated for every entry in `spec.databases`. If a
database is removed from `spec.databases`, its keys are removed from the Secret on the next
reconcile (the Secret's data is fully regenerated each time, not merged).

The password stored in the Secret is authoritative after the first reconcile: the operator
re-applies whatever password is currently in the Secret to the Postgres role on every reconcile,
so editing the `password` key directly and letting the operator reconcile will change the role's
actual password in Postgres.

## Status Conditions

See [Status & Conditions](./index.md#status-conditions) for the condition types set on this
resource.

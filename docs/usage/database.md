# PgDatabase
## Resource Definition

The `PgDatabase` resource manages a database on the referenced instance.

```yaml
apiVersion: postgres.oebc.tools/v1
kind: PgDatabase
metadata:
  name: service_db
spec:
  instance:
    namespace: "default"
    name: "instance-001"
  deletion:
    wait: true # Wait until the database was deleted manually on the postgres instance
  defaultPrivileges:
    - schemaName: "service"
      roles: ["developer"]
      schemaPrivileges: ["USAGE", "CREATE"]
      tablePrivileges: ["SELECT", "INSERT", "UPDATE", "DELETE"]
      sequencePrivileges: ["SELECT", "UPDATE", "USAGE"]
      functionPrivileges: ["EXECUTE"]
      typePrivileges: ["USAGE"]
  publicPrivileges:
    revoke: false # revoke all public privileges from the database
  publicSchema:
    drop: false # drop the public schema from the database
```

When creating the resource a deletion strategy can be specified.
This allows the database resource to be deleted, without deleting the actual database in the Postgres Instance.

## Attribute Description

| Attribute                     | Description                                                                                  | Required |
|--------------------------------|------------------------------------------------------------------------------------------------|----------|
| `instance.namespace`/`.name`   | Reference to the `PgInstance` on which this database should be managed                        | :white_check_mark: |
| `deletion.drop`                | If `true`, the operator runs `DROP DATABASE` when this resource is deleted. Default `false`.   | :x: |
| `deletion.wait`                | If `true`, the finalizer blocks resource deletion until the database has been dropped manually from Postgres. Default `false`. | :x: |
| `extensions`                   | List of Postgres extension names that should exist in the database (created if missing, never dropped) | :x: |
| `defaultPrivileges`            | List of schema/role privilege grants, see below                                               | :x: |
| `publicPrivileges.revoke`      | If `true`, revokes all database-level and `public`-schema privileges from the `public` role    | :white_check_mark: |
| `publicSchema.drop`            | If `true`, drops the `public` schema from the database (if it exists)                          | :white_check_mark: |

`publicPrivileges` and `publicSchema` are always required in the manifest (even if both are set
to `false`), since Postgres grants broad `public` access to every new database by default and the
operator wants callers to make an explicit choice.

### `defaultPrivileges`

Each entry grants privileges on one schema to a list of roles, and additionally makes those the
*default* privileges for objects created in that schema in the future
(`ALTER DEFAULT PRIVILEGES ...`) as well as applying them to objects that already exist in the
schema at reconcile time (tables/sequences/functions; not types).

| Attribute            | Description                                                              | Required |
|-----------------------|---------------------------------------------------------------------------|----------|
| `schemaName`          | Name of the schema the privileges apply to. The schema must already exist — the operator does **not** create it. | :white_check_mark: |
| `roles`               | Names of the roles the privileges are granted to                        | :white_check_mark: |
| `schemaPrivileges`    | `USAGE`, `CREATE`                                                        | :x: |
| `tablePrivileges`     | `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `REFERENCES`, `TRIGGER` | :x: |
| `sequencePrivileges`  | `SELECT`, `UPDATE`, `USAGE`                                              | :x: |
| `functionPrivileges`  | `EXECUTE`                                                                | :x: |
| `typePrivileges`      | `USAGE`                                                                  | :x: |

There is no `"ALL"` shortcut — list every privilege you want to grant explicitly. The operator
also always grants `USAGE` on the schema itself to the listed roles, regardless of whether
`USAGE` is included in `schemaPrivileges`, since without it none of the other grants are usable.

## Deletion Strategies

`deletion.drop` and `deletion.wait` are independent flags:

| `drop` | `wait` | Behavior when the `PgDatabase` resource is deleted |
|--------|--------|------------------------------------------------------|
| `false` | `false` | **Default.** The k8s resource is removed immediately; the database is left in place in Postgres (orphaned). |
| `true`  | `false` | The operator runs `DROP DATABASE` and then removes the k8s resource. |
| `false` | `true`  | The k8s resource deletion is blocked until the database no longer exists in Postgres — useful if you want a human (or another process) to drop the database manually before the resource disappears. |
| `true`  | `true`  | The operator runs `DROP DATABASE`, then removes the resource once the database is confirmed gone. |

## Status Conditions

See [Status & Conditions](./index.md#status-conditions) for the condition types set on this
resource.

# Usage

The operator manages three resource types. They are always used together in this order:

1. **[PgInstance](./instance.md)** — connection details for an existing Postgres server. Create
   exactly one of these per Postgres server you want the operator to manage.
2. **[PgDatabase](./database.md)** — a database on that instance, plus optional extensions and
   default privilege grants.
3. **[PgUser](./user.md)** — a login role on that instance, with a generated k8s Secret
   containing its credentials and per-database connection strings, and ownership/privilege
   management for the databases it should access.

A `PgUser` referencing databases in its `spec.databases` list expects those databases to already
exist — reconciliation waits (fast-retrying) until they do, so `PgDatabase` and `PgUser` resources
for the same database can be applied at the same time without ordering them manually.

## Quickstart

```yaml
apiVersion: postgres.oebc.tools/v1
kind: PgInstance
metadata:
  name: instance-001
spec:
  host:
    value: "postgres.example.com"
  username:
    value: "postgres"
  password:
    secretKeyRef:
      name: "postgres-admin-credentials"
      key: "password"
---
apiVersion: postgres.oebc.tools/v1
kind: PgDatabase
metadata:
  name: service_db
spec:
  instance:
    namespace: "default"
    name: "instance-001"
  deletion:
    wait: true
  publicPrivileges:
    revoke: false
  publicSchema:
    drop: false
---
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

Applying this creates a `service_db` database on `instance-001`, a `service_user` login role that
owns it, and a `service-credentials` Secret with connection details for `service_user` — see
[PgInstance](./instance.md), [PgDatabase](./database.md) and [PgUser](./user.md) for the full
field reference of each resource.

## Status Conditions

All three resource types report their state via `status.conditions`, following the standard
Kubernetes condition shape (`type`, `status`, `reason`, `message`). These are the condition types
you'll see:

| Resource     | Condition Type                                        | Meaning                                                              |
|--------------|--------------------------------------------------------|-----------------------------------------------------------------------|
| All three    | `postgres.oebc.tools/connected`                        | Whether the operator could connect to the referenced `PgInstance`     |
| `PgDatabase` | `pgdatabase.postgres.oebc.tools/exists`                | Whether the database exists on the instance                          |
| `PgDatabase` | `pgdatabase.postgres.oebc.tools/extensions`             | Whether all `spec.extensions` are present in the database             |
| `PgDatabase` | `pgdatabase.postgres.oebc.tools/default-privileges`     | Whether all `spec.defaultPrivileges` grants were applied successfully |
| `PgUser`     | `pguser.postgres.oebc.tools/exists`                    | Whether the login role exists on the instance                        |
| `PgUser`     | `pguser.postgres.oebc.tools/databases`                 | Whether all databases in `spec.databases` currently exist            |

These are also what the [ArgoCD health checks](./argocd.md) are built on, so the same condition
types apply whether or not you use ArgoCD.

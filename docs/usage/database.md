!!! warning "Work in Progress"

    This page is still work in progress and will be updated as soon as possible.<br />
    Feel free to create a [Pull Request](https://github.com/brose-ebike/postgres-operator/pulls) for this page.

# PgDatabase

The `PgDatabase` resource manages a database on the referenced instance.

```yaml
apiVersion: postgres.brose.bike/v1
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
    - name: "service"
      roles: ["developer"]
      tablePrivileges: ["ALL"]
      sequencePrivileges: ["ALL"]
      functionPrivileges: ["ALL"]
      routinePrivileges: ["ALL"]
      typePrivileges: ["ALL"]
  publicPrivileges:
    revoke: false # revoke all public privileges from the database
  publicSchema:
    drop: false # drop the public schema from the database
```

When creating the resource a deletion strategy can be specified.
This allows the database resource to be deleted, without deleting the actual database in the Postgres Instance.
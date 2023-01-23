!!! warning "Work in Progress"

    This page is still work in progress and will be updated as soon as possible.<br />
    Feel free to create a [Pull Request](https://github.com/brose-ebike/postgres-operator/pulls) for this page.

# PgInstance

To manage databases and users on an instance, the `PgInstance` resource has to be created at first.
The `PgInstance` allows the operator to connect to the postgres instance.

```yaml
apiVersion: postgres.brose.bike/v1
kind: PgInstance
metadata:
  name: instance-001
spec:
  host:
    secretKeyRef: 
      name: "my-secret"
      key: "hostname"
      optional: false
  port:
    secretKeyRef: 
      name: "my-secret"
      key: "port"
      optional: false
  username:
    secretKeyRef: 
      name: "my-secret"
      key: "username"
      optional: false
  password:
    secretKeyRef: 
      name: "my-secret"
      key: "password"
      optional: false
  database:
    value: "postgres"
  sslMode:
    value: "require"
    
```

After the `PgInstance` was created successfully, databases and users can be managed on the referenced instance.
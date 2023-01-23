!!! warning "Work in Progress"

    This page is still work in progress and will be updated as soon as possible.<br />
    Feel free to create a [Pull Request](https://github.com/brose-ebike/postgres-operator/pulls) for this page.

# PgUser
The `PgUser` resource manages a role with login (user) on the referenced instance.

```yaml
apiVersion: postgres.brose.bike/v1
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
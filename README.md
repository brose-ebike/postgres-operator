# postgres-operator
A simple k8s operator to create PostgresSQL databases and users. Once you install the operator and point it at your existing PostgresSQL database instance, you can create `PgDatabase` or `PgUser` resource in k8s 
and the operator will create a database or a role with password in your PostgresSQL instance. 
Create a role that with access to this Postgres Instance and optionally update privileges.

## Description
The Brose E-Bike Postgres Operator manages Postgres Databases and Users on existing instances.
If you want to create Postgres instances in K8s checkout the [Zalando Postgres Operator](https://github.com/zalando/postgres-operator). When using this operator you need a user with `superuser` like privileges.
Checkout the [documentation](https://brose-ebike.github.io/postgres-operator/) for more information.

### PgInstance

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
Checkout the [documentation](https://brose-ebike.github.io/postgres-operator/) for more information.

### PgDatabase

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
Checkout the [documentation](https://brose-ebike.github.io/postgres-operator/) for more information.

### PgUser
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

Checkout the [documentation](https://brose-ebike.github.io/postgres-operator/) for more information.

## License

Copyright 2023 Brose Fahrzeugteile SE & Co. KG, Bamberg.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this writing and software except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


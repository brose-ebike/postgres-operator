!!! warning "Work in Progress"

    This page is still work in progress and will be updated as soon as possible.<br />
    Feel free to create a [Pull Request](https://github.com/brose-ebike/postgres-operator/pulls) for this page.

# PgInstance

To manage databases and users on an instance, the `PgInstance` resource has to be created at first.
The `PgInstance` allows the operator to connect to the postgres instance.
The connection details can be provided as secret, config map or directly in the `value` field.
After the `PgInstance` was created successfully, databases and users can be managed on the referenced instance.

=== "PgInstance from secrets"
    <!--codeinclude-->
    [PgInstance from secret](../../config/samples/pginstance/from_secret.yaml)
    <!--/codeinclude-->

=== "PgInstance from config map"
    <!--codeinclude-->
    [PgInstance from config map](../../config/samples/pginstance/from_config_map.yaml)
    <!--/codeinclude-->

=== "PgInstance from value"
    <!--codeinclude-->
    [PgInstance from value](../../config/samples/pginstance/from_values.yaml)
    <!--/codeinclude-->

## Attribute Description
The sources of the attribute values can differ.
For example host, port, database and sslmode can be stored in the value.
The username and password can be stored in a secret.
The operator will resolve all attribute values when the resource gets applied.
Its better to avoid having defined multiple sources for an attribute, buts its permitted by the operator.

| Attribute   | Decription                                                                   | Optional            | Default  |   |
|-------------|------------------------------------------------------------------------------|---------------------|----------|---|
| `hostname`  | The hostname of the postgres instance to which the operator should connect   | :x:                 | -        |   |
| `port`      | The port of the postgres instance to which the operator should connect       | :white_check_mark:  | 8432     |   |
| `username`  | The username of the administration user which should be used by the operator | :x:                 | -        |   |
| `password`  | The password of the administration user which should be used by the operator | :x:                 | -        |   |
| `database`  | The maintenance database which should be used to establish the connection to | :white_check_mark:  | postgres |   |
| `sslmode`   | The SSLMode which should be used for the connection to the postgres instance | :white_check_mark:  | none     |   |

## Required Privileges
The user which is provided for the `PgInstance` to connect to the instance needs to have at least superuser like privileges.
The minimal privileges are described below. Usually the superuser permission should be assigned to the user,
which should be used by the operator.

### Superuser Privileges
The superuser permission can be assigned easily by executing following sql statement, 
after replacing `[USERNAME]` with the name of the user which should be used by the operator:

```sql
ALTER USER [USERNAME] WITH SUPERUSER;
```

### Minimal Privileges 
The following privileges need to be assigned to the user or to one of the assigned roles of the user:

* Role Privileges:
    * `CREATEDB` is needed to create new database, which can be managed by the `PgDatabase` resource
    * `CREATEROLE` is needed to create new roles, which are created when using the `PgUser` resource
    * `CREATEUSER` is needed to create new users, which can be managed by the `PgUser` resource
* Database Privileges:
    * `CONNECT` for the given database is needed to establish the initial connection
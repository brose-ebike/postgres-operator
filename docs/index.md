# Postgres Controller

The Brose E-Bike Postgres Operator  adds users ([PgUser](./usage/user.md)) and databases ([PgDatabase](./usage/database.md)) as resource types in Kubernetes clusters, and simplifies the process of creating and managing databases and service roles.
If you want to create Postgres instances in K8s checkout the [Zalando Postgres Operator](https://github.com/zalando/postgres-operator). When using this operator you need a user with `superuser` like privileges.
Checkout the [documentation](https://brose-ebike.github.io/postgres-operator/) for more information.

The operator can interact with a variety of supported Postgres versions from different Vendors (e.g. Self hosted, Azure, AWS, GCP).
The user which connects to the Postgres instance should be a superuser or a user with superuser like privileges.

This website provides the full technical documentation for the project, and can be used as a reference; 
if you feel that there's anything missing, please let us know or  [raise a PR](https://github.com/brose-ebike/postgres-operator/pulls) to add it.
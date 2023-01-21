# postgres-operator
A simple k8s controller to create PostgresSQL databases and users. Once you install the controller and point it at your existing PostgresSQL database instance, you can create `PgDatabase` or `PgUser` resource in k8s 
and the controller will create a database or a role with password in your PostgresSQL instance. 
Create a role that with access to this Postgres Instance and optionally update privileges.

## Description
// TODO(user): An in-depth paragraph about your project and overview of use


## License

Copyright 2023 Brose Fahrzeugteile SE & Co. KG, Bamberg.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


/*
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
*/
/*
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
*/
package pgapi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PostgresAPI Database Handling", func() {

	It("can create database", func() {
		// Create new database
		err := pgApi.CreateDatabase("dummy_db_0")
		Expect(err).To(BeNil())
		// Check if database exists
		exists, err := pgApi.IsDatabaseExisting("dummy_db_0")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	It("can delete database", func() {
		// Create new database
		err := pgApi.CreateDatabase("dummy_db_1")
		Expect(err).To(BeNil())
		// Check if database exists
		exists, err := pgApi.IsDatabaseExisting("dummy_db_1")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		// Delete database
		err = pgApi.DeleteDatabase("dummy_db_1")
		Expect(err).To(BeNil())
	})

	It("can update database owner", func() {
		newOwnerName := "dummy_db_2_owner"
		databaseName := "dummy_db_2"
		// Create new role
		err := pgApi.CreateRole(newOwnerName)
		Expect(err).To(BeNil())
		// Create new database
		err = pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Check if database exists
		exists, err := pgApi.IsDatabaseExisting(databaseName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		// Update database owner
		err = pgApi.UpdateDatabaseOwner(databaseName, newOwnerName)
		Expect(err).To(BeNil())
		// Check Database owner
		dbOwner, err := pgApi.GetDatabaseOwner(databaseName)
		Expect(err).To(BeNil())
		Expect(dbOwner).To(Equal(newOwnerName))
	})

	It("can update database privileges", func() {
		roleName := "dummy_role_3"
		databaseName := "dummy_db_3"
		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())
		// Create new database
		err = pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Update Database Privileges
		err = pgApi.UpdateDatabasePrivileges(databaseName, roleName, []string{"CONNECT"})
		Expect(err).To(BeNil())
	})

	It("can reset database privileges", func() {
		roleName := "dummy_role_4"
		databaseName := "dummy_db_4"
		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())
		// Create new database
		err = pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Update Database Privileges
		err = pgApi.UpdateDatabasePrivileges(databaseName, roleName, []string{"CONNECT"})
		Expect(err).To(BeNil())
		// Reset Privileges
		err = pgApi.UpdateDatabasePrivileges(databaseName, roleName, []string{})
		Expect(err).To(BeNil())
	})

})

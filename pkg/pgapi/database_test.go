/*
Copyright 2026 Yamaha Motor eBike Systems GmbH.

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
	"errors"

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

	It("does not leave admin permanently a member of the role if UpdateDatabaseOwner fails", func() {
		roleName := "dummy_role_12"
		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())
		// Target a database that doesn't exist so ALTER DATABASE fails deterministically
		err = pgApi.UpdateDatabaseOwner("dummy_db_does_not_exist", roleName)
		Expect(err).ToNot(BeNil())
		// Admin should not be left a member of roleName after the failed attempt
		impl := pgApi.(*pgInstanceAPIImpl)
		conn, connErr := impl.newConnection()
		Expect(connErr).To(BeNil())
		defer conn.Close()
		isMember, memberErr := impl.isMember(conn, impl.connectionString.username, roleName)
		Expect(memberErr).To(BeNil())
		Expect(isMember).To(BeFalse())
	})

	It("still revokes a borrowed role even if the runner panics", func() {
		roleName := "dummy_role_13"
		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())

		impl := pgApi.(*pgInstanceAPIImpl)
		conn, connErr := impl.newConnection()
		Expect(connErr).To(BeNil())
		defer conn.Close()

		// Call runAs with a runner that panics, recovering locally so the
		// test itself keeps running afterward
		func() {
			defer func() { _ = recover() }()
			_ = impl.runAs(conn, roleName, func() error {
				panic("boom")
			})
		}()

		// Admin should not be left a member of roleName after the panic
		isMember, memberErr := impl.isMember(conn, impl.connectionString.username, roleName)
		Expect(memberErr).To(BeNil())
		Expect(isMember).To(BeFalse())
	})

	It("can update database privileges", func() {
		roleName := "dummy_role_10"
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
		roleName := "dummy_role_11"
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

	It("can create extensions", func() {
		databaseName := "dummy_db_5"
		// Create new database
		err := pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Create Extension
		err = pgApi.CreateDatabaseExtension(databaseName, "uuid-ossp")
		Expect(err).To(BeNil())
		// Create Extension
		exists, err := pgApi.IsDatabaseExtensionPresent(databaseName, "uuid-ossp")
		Expect(exists).To(BeTrue())
		Expect(err).To(BeNil())
	})

	It("cannot create a database twice", func() {
		// Create new database
		err := pgApi.CreateDatabase("dummy_db_6")
		Expect(err).To(BeNil())
		// Check if database exists
		exists, err := pgApi.IsDatabaseExisting("dummy_db_6")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		// Create the database twice
		err = pgApi.CreateDatabase("dummy_db_6")
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("Unable to execute query 'create database %s;' with arguments 'dummy_db_6'\npq: database \"dummy_db_6\" already exists (42P04)"))
		Expect(errors.Unwrap(err).Error()).To(Equal("pq: database \"dummy_db_6\" already exists (42P04)"))
	})
})

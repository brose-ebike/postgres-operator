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
	"context"
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
		nonSuperAdmin := "dummy_nonsuper_admin_12"
		nonSuperPassword := "super-secret-password-12"
		impl := pgApi.(*pgInstanceAPIImpl)

		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())

		// The container's own admin is a real Postgres superuser, and
		// pg_has_role(..., 'member') treats a superuser as an implicit
		// member of every role regardless of any actual grant - so
		// withBorrowedRole's grant/revoke branch never even runs for it.
		// Exercising the real grant/revoke path this fix targets requires
		// connecting as a genuinely non-superuser role instead, mirroring a
		// real "Minimal Privileges" deployment (CREATEROLE, no SUPERUSER).
		setupConn, setupConnErr := impl.newConnection()
		Expect(setupConnErr).To(BeNil())
		defer setupConn.Close()
		_, createNonSuperErr := setupConn.ExecContext(impl.ctx, formatQueryObj(
			"create user %s with createrole login password '"+nonSuperPassword+"';",
			nonSuperAdmin,
		))
		Expect(createNonSuperErr).To(BeNil())

		nonSuperConnStr, connStrErr := NewPgConnectionString(
			impl.connectionString.Hostname(),
			impl.connectionString.Port(),
			nonSuperAdmin,
			nonSuperPassword,
			impl.connectionString.Database(),
			impl.connectionString.SSLMode(),
		)
		Expect(connStrErr).To(BeNil())
		nonSuperCtx, nonSuperCancel := context.WithCancel(context.Background())
		defer nonSuperCancel()
		nonSuperApi, apiErr := NewPgInstanceAPI(nonSuperCtx, "nonsuper-test-12", nonSuperConnStr)
		Expect(apiErr).To(BeNil())

		// Target a database that doesn't exist so ALTER DATABASE fails deterministically
		updateErr := nonSuperApi.UpdateDatabaseOwner("dummy_db_does_not_exist", roleName)
		Expect(updateErr).ToNot(BeNil())

		// The non-superuser admin should not be left a member of roleName
		checkConn, checkConnErr := impl.newConnection()
		Expect(checkConnErr).To(BeNil())
		defer checkConn.Close()
		isMember, memberErr := impl.isMember(checkConn, nonSuperAdmin, roleName)
		Expect(memberErr).To(BeNil())
		Expect(isMember).To(BeFalse())
	})

	It("still revokes a borrowed role even if the runner panics", func() {
		roleName := "dummy_role_13"
		nonSuperAdmin := "dummy_nonsuper_admin_13"
		nonSuperPassword := "super-secret-password-13"
		impl := pgApi.(*pgInstanceAPIImpl)

		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())

		// See the previous test for why a genuinely non-superuser connection
		// is required to exercise the real grant/revoke path here.
		setupConn, setupConnErr := impl.newConnection()
		Expect(setupConnErr).To(BeNil())
		defer setupConn.Close()
		_, createNonSuperErr := setupConn.ExecContext(impl.ctx, formatQueryObj(
			"create user %s with createrole login password '"+nonSuperPassword+"';",
			nonSuperAdmin,
		))
		Expect(createNonSuperErr).To(BeNil())

		nonSuperConnStr, connStrErr := NewPgConnectionString(
			impl.connectionString.Hostname(),
			impl.connectionString.Port(),
			nonSuperAdmin,
			nonSuperPassword,
			impl.connectionString.Database(),
			impl.connectionString.SSLMode(),
		)
		Expect(connStrErr).To(BeNil())
		nonSuperCtx, nonSuperCancel := context.WithCancel(context.Background())
		defer nonSuperCancel()
		nonSuperApi, apiErr := NewPgInstanceAPI(nonSuperCtx, "nonsuper-test-13", nonSuperConnStr)
		Expect(apiErr).To(BeNil())
		nonSuperImpl := nonSuperApi.(*pgInstanceAPIImpl)

		workConn, workConnErr := nonSuperImpl.newConnection()
		Expect(workConnErr).To(BeNil())
		defer workConn.Close()

		// Call runAs with a runner that panics, recovering locally so the
		// test itself keeps running afterward
		func() {
			defer func() { _ = recover() }()
			_ = nonSuperImpl.runAs(workConn, roleName, func() error {
				panic("boom")
			})
		}()

		// The non-superuser admin should not be left a member of roleName after the panic
		checkConn, checkConnErr := impl.newConnection()
		Expect(checkConnErr).To(BeNil())
		defer checkConn.Close()
		isMember, memberErr := impl.isMember(checkConn, nonSuperAdmin, roleName)
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

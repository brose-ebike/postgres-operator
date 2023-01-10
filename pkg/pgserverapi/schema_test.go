package pgserverapi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PostgresAPI Schema Handling", func() {
	It("can create schema", func() {
		databaseName := "dummy_db_5"
		schemaName := "service"
		// Create new database
		err := pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Check if schema exists
		exists, err := pgApi.IsSchemaInDatabase(databaseName, schemaName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		// Create Schema
		err = pgApi.CreateSchema(databaseName, schemaName)
		Expect(err).To(BeNil())
		// Check if schema exists
		exists, err = pgApi.IsSchemaInDatabase(databaseName, schemaName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	It("can delete schema", func() {
		databaseName := "dummy_db_6"
		schemaName := "service"
		// Create new database
		err := pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Create Schema
		err = pgApi.CreateSchema(databaseName, schemaName)
		Expect(err).To(BeNil())
		// Check if schema exists
		exists, err := pgApi.IsSchemaInDatabase(databaseName, schemaName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		// Delete Schema
		err = pgApi.DeleteSchema(databaseName, schemaName)
		Expect(err).To(BeNil())
		// Check if schema exists
		exists, err = pgApi.IsSchemaInDatabase(databaseName, schemaName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
	})

	It("can update default privileges", func() {
		roleName := "dummy_role_7"
		databaseName := "dummy_db_7"
		schemaName := "service"
		// Create new role
		err := pgApi.CreateLoginRole(roleName)
		Expect(err).To(BeNil())
		// Create new database
		err = pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Create Schema
		err = pgApi.CreateSchema(databaseName, schemaName)
		Expect(err).To(BeNil())
		// Update Schema Privileges
		err = pgApi.UpdateDefaultPrivileges(databaseName, schemaName, roleName, "TABLES", []string{"SELECT"})
		Expect(err).To(BeNil())
	})

	It("can delete privileges on schema", func() {
		roleName := "dummy_role_8"
		databaseName := "dummy_db_8"
		schemaName := "service"
		// Create new role
		err := pgApi.CreateLoginRole(roleName)
		Expect(err).To(BeNil())
		// Create new database
		err = pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Create Schema
		err = pgApi.CreateSchema(databaseName, schemaName)
		Expect(err).To(BeNil())
		// Update Schema Privileges
		err = pgApi.UpdateDefaultPrivileges(databaseName, schemaName, roleName, "TABLES", []string{"SELECT"})
		Expect(err).To(BeNil())
		// Delete all privileges on schema
		err = pgApi.DeleteAllPrivilegesOnSchema(databaseName, schemaName, roleName)
		Expect(err).To(BeNil())
	})
})

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

var _ = Describe("PostgresAPI Schema Handling", func() {
	It("can create schema", func() {
		databaseName := "dummy_db_11"
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
		databaseName := "dummy_db_12"
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
		databaseName := "dummy_db_13"
		schemaName := "service"
		// Create new role
		err := pgApi.CreateRole(roleName)
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
		databaseName := "dummy_db_14"
		schemaName := "service"
		// Create new role
		err := pgApi.CreateRole(roleName)
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

	It("can update privileges", func() {
		roleName := "dummy_role_9"
		databaseName := "dummy_db_15"
		schemaName := "service"
		// Create new role
		err := pgApi.CreateRole(roleName)
		Expect(err).To(BeNil())
		// Create new database
		err = pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())
		// Create Schema
		err = pgApi.CreateSchema(databaseName, schemaName)
		Expect(err).To(BeNil())
		// Update Schema Privileges
		err = pgApi.UpdatePrivilegesOnAllObjects(databaseName, schemaName, roleName, "TABLES", []string{"SELECT"})
		Expect(err).To(BeNil())
	})
})

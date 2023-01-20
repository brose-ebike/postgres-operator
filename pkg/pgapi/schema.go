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
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/brose-ebike/postgres-controller/pkg/brose_errors"
	_ "github.com/lib/pq"
)

// PgSchemaAPI provides functionality to check and manipulate
// schemas and privileges on schemas
type PgSchemaAPI interface {
	// IsSchemaInDatabase returns true if a schema
	// with the given name exists in the given database and false if not.
	IsSchemaInDatabase(databaseName string, schemaName string) (bool, error)
	// CreateSchema creates a new schema with the given name in the given database
	CreateSchema(databaseName string, schemaName string) error
	// DeleteSchema drops the given schema from the given database
	DeleteSchema(databaseName string, schemaName string) error
	// UpdateDefaultPrivileges updates the default privileges in the given schema
	// for the given role on the given type to the given privileges
	UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error
	// DeleteAllPrivilegesOnSchema removes all privileges on the given schema for the given role
	DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error
}

func (s *pgInstanceAPIImpl) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	var exists bool
	err := s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "select exists(select * from pg_catalog.pg_namespace where nspname = $1);"
		return conn.QueryRowContext(ctx, query, schemaName).Scan(&exists)
	})
	return exists, err
}

func (s *pgInstanceAPIImpl) CreateSchema(databaseName string, schemaName string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "create schema %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName))
		return err
	})
}

func (s *pgInstanceAPIImpl) DeleteSchema(databaseName string, schemaName string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "drop schema %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName))
		return err
	})
}

func (s *pgInstanceAPIImpl) UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	if len(privileges) == 0 {
		return nil
	}
	// Validate typeName Parameter
	allowedTypes := []string{"TABLES", "SEQUENCES", "FUNCTIONS", "ROUTINES", "TYPES", "SCHEMAS"}
	if !hasElementString(allowedTypes, typeName) {
		return brose_errors.NewIllegalArgumentError("typeName", typeName, nil)
	}
	// Validate Privileges Parameter
	allowedPrivileges := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "USAGE", "CONNECT", "ALL"}
	for _, privilege := range privileges {
		if !hasElementString(allowedPrivileges, privilege) {
			return brose_errors.NewIllegalArgumentError("privileges", privilege, nil)
		}
	}
	// Run in Database
	err := s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		joinedPrivileges := strings.Join(privileges, ", ")
		query := "alter default privileges in schema  %s grant " + joinedPrivileges + " on " + typeName + " to  %s;"
		_, err := conn.ExecContext(ctx, fmt.Sprintf(query, schemaName, roleName))
		return err
	})
	return err
}

func (s *pgInstanceAPIImpl) DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		// This gets executed on the database `databaseName`
		const query = "revoke all on schema %s from %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName, role))
		return err
	})
}

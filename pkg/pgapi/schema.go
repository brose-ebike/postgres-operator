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

	"github.com/brose-ebike/postgres-operator/pkg/brose_errors"
	_ "github.com/lib/pq"
)

var pgTypes = []string{"TABLES", "SEQUENCES", "FUNCTIONS", "ROUTINES", "TYPES", "SCHEMAS"}
var pgPrivileges = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "USAGE", "CONNECT", "CREATE", "ALL"}

func validateTypeName(typeName string) error {
	if !hasElementString(pgTypes, typeName) {
		return brose_errors.NewIllegalArgumentError("typeName", typeName, nil)
	}
	return nil
}

func validatePrivileges(privileges []string) error {
	for _, privilege := range privileges {
		if !hasElementString(pgPrivileges, privilege) {
			return brose_errors.NewIllegalArgumentError("privileges", privilege, nil)
		}
	}
	return nil
}

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
	// UpdateSchemaPrivileges updates the privileges for the given schema
	UpdateSchemaPrivileges(databaseName string, schemaName string, roleName string, privileges []string) error
	// UpdatePrivilegesOnAllObjects updates the privileges according to the given parameters
	UpdatePrivilegesOnAllObjects(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error
	// UpdateDefaultPrivileges updates the default privileges in the given schema
	// for the given role on the given type to the given privileges
	UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error
	// DeleteAllPrivilegesOnSchema removes all privileges on the given schema for the given role
	DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error
	// IsSchemaUsable checks if the current user has the use privilege on the given schema
	IsSchemaUsable(databaseName string, schemaName string) (bool, error)
	// MakeSchemaUseable grants the use privilege on the given schema to the current user
	MakeSchemaUseable(databaseName string, schemaName string) error
	// GetSchemaOwner returns the owner of the database with the given name on the connected instance
	GetSchemaOwner(databaseName string, schemaName string) (string, error)
}

func (s *pgInstanceAPIImpl) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	var exists bool
	err := s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "select exists(select * from pg_catalog.pg_namespace where nspname = $1);"
		err := conn.QueryRowContext(ctx, query, schemaName).Scan(&exists)
		return WrapSqlExecutionError(err, query, schemaName)
	})
	return exists, err
}

func (s *pgInstanceAPIImpl) CreateSchema(databaseName string, schemaName string) error {
	return s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "create schema %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName))
		return WrapSqlExecutionError(err, query, schemaName)
	})
}

func (s *pgInstanceAPIImpl) DeleteSchema(databaseName string, schemaName string) error {
	return s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "drop schema %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName))
		return WrapSqlExecutionError(err, query, schemaName)
	})
}

func (s *pgInstanceAPIImpl) UpdateSchemaPrivileges(databaseName string, schemaName string, roleName string, privileges []string) error {
	if len(privileges) == 0 {
		return nil
	}
	// Get Database Owner
	dbOwner, err := s.GetDatabaseOwner(databaseName)
	if err != nil {
		return err
	}
	// Validate Privileges Parameter
	if err := validatePrivileges(privileges); err != nil {
		return err
	}
	// Execute Grants
	return s.runInAs(databaseName, dbOwner, func(ctx context.Context, conn *sql.Conn) error {
		// This gets executed on the database `databaseName`
		joinedPrivileges := strings.Join(privileges, ", ")
		var queryB = "GRANT " + joinedPrivileges + " ON SCHEMA %s TO %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(queryB, schemaName, roleName))
		return WrapSqlExecutionError(err, queryB, schemaName, roleName)
	})
}

func (s *pgInstanceAPIImpl) UpdatePrivilegesOnAllObjects(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	if len(privileges) == 0 {
		return nil
	}
	// Validate typeName Parameter
	if err := validateTypeName(typeName); err != nil {
		return err
	}
	// Validate Privileges Parameter
	if err := validatePrivileges(privileges); err != nil {
		return err
	}
	// Get Database Owner
	dbOwner, err := s.GetDatabaseOwner(databaseName)
	if err != nil {
		return err
	}
	// Execute Grants
	return s.runInAs(databaseName, dbOwner, func(ctx context.Context, conn *sql.Conn) error {
		joinedPrivileges := strings.Join(privileges, ", ")
		query := "GRANT " + joinedPrivileges + " ON ALL " + typeName + " IN SCHEMA %s TO  %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName, roleName))
		return WrapSqlExecutionError(err, query, schemaName, roleName)
	})
}

func (s *pgInstanceAPIImpl) UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	if len(privileges) == 0 {
		return nil
	}
	// Validate typeName Parameter
	if err := validateTypeName(typeName); err != nil {
		return err
	}
	// Validate Privileges Parameter
	if err := validatePrivileges(privileges); err != nil {
		return err
	}
	// Run in Database
	err := s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		joinedPrivileges := strings.Join(privileges, ", ")
		query := "alter default privileges in schema  %s grant " + joinedPrivileges + " on " + typeName + " to  %s;"
		_, err := conn.ExecContext(ctx, fmt.Sprintf(query, schemaName, roleName))
		return WrapSqlExecutionError(err, query, schemaName, roleName)
	})
}

func (s *pgInstanceAPIImpl) DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error {
	return s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		// This gets executed on the database `databaseName`
		const query = "revoke all on schema %s from %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName, role))
		return WrapSqlExecutionError(err, query, schemaName, role)
	})
}

func (s *pgInstanceAPIImpl) IsSchemaUsable(databaseName string, schemaName string) (bool, error) {
	var useable bool
	err := s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "SELECT pg_catalog.has_schema_privilege(current_user, $1, 'USAGE');"
		err := conn.QueryRowContext(ctx, query, schemaName).Scan(&useable)
		return WrapSqlExecutionError(err, query, schemaName)
	})
	return useable, err
}

func (s *pgInstanceAPIImpl) MakeSchemaUseable(databaseName string, schemaName string) error {
	// Get Database Owner
	schemaOwner, err := s.GetSchemaOwner(databaseName, schemaName)
	if err != nil {
		return err
	}

	// Execute Grants
	return s.runInAs(databaseName, schemaOwner, func(ctx context.Context, conn *sql.Conn) error {
		// This gets executed on the database `databaseName`
		const queryA = "GRANT CONNECT ON DATABASE %s TO %s;"
		if _, err := conn.ExecContext(ctx, formatQueryObj(queryA, databaseName, s.connectionString.username)); err != nil {
			return WrapSqlExecutionError(err, queryA, schemaName, s.connectionString.username)
		}
		// This gets executed on the database `databaseName`
		const queryB = "GRANT USAGE ON SCHEMA %s TO %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(queryB, schemaName, s.connectionString.username))
		return WrapSqlExecutionError(err, queryB, schemaName, s.connectionString.username)
	})
}

func (s *pgInstanceAPIImpl) GetSchemaOwner(databaseName string, schemaName string) (string, error) {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return "", err
	}
	var schemaOwner string
	const query = "select r.rolname as schema_owner from pg_namespace ns join pg_roles r on ns.nspowner = r.oid where nspname=$1;"
	err = conn.QueryRowContext(s.ctx, query, databaseName).Scan(&schemaOwner)
	if err != nil {
		return "", WrapSqlExecutionError(err, query, databaseName)
	}
	return schemaOwner, nil
}

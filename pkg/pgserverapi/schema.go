package pgserverapi

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/brose-ebike/postgres-controller/pkg/brose_errors"
	_ "github.com/lib/pq"
)

func (s *PgServerAPIImpl) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	var exists bool
	err := s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "select exists(select * from pg_catalog.pg_namespace where nspname = $1);"
		return conn.QueryRowContext(ctx, query, schemaName).Scan(&exists)
	})
	return exists, err
}

func (s *PgServerAPIImpl) CreateSchema(databaseName string, schemaName string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "create schema %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName))
		return err
	})
}

func (s *PgServerAPIImpl) DeleteSchema(databaseName string, schemaName string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "drop schema %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName))
		return err
	})
}

func (s *PgServerAPIImpl) UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
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

func (s *PgServerAPIImpl) DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		// This gets executed on the database `databaseName`
		const query = "revoke all on schema %s from %s;"
		_, err := conn.ExecContext(ctx, formatQueryObj(query, schemaName, role))
		if err != nil {
			return err
		}
		return nil
	})
}

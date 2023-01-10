package pgserverapi

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/brose-ebike/postgres-controller/pkg/brose_errors"
	"github.com/brose-ebike/postgres-controller/pkg/utils"
	_ "github.com/lib/pq"
)

func (s *PgServerApiImpl) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	var exists bool
	err := s.runInDatabase(databaseName, func(ctx context.Context, con *sql.Conn) error {
		const query = "select exists(select * from pg_catalog.pg_namespace where nspname = $1);"
		return con.QueryRowContext(ctx, query, schemaName).Scan(&exists)
	})
	return exists, err
}

func (s *PgServerApiImpl) CreateSchema(databaseName string, schemaName string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, con *sql.Conn) error {
		const query = "create schema %s;"
		_, err := con.ExecContext(ctx, formatQueryObj(query, schemaName))
		return err
	})
}

func (s *PgServerApiImpl) DeleteSchema(databaseName string, schemaName string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, con *sql.Conn) error {
		const query = "drop schema %s;"
		_, err := con.ExecContext(ctx, formatQueryObj(query, schemaName))
		return err
	})
}

func (s *PgServerApiImpl) UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	if len(privileges) == 0 {
		return nil
	}
	// Validate typeName Parameter
	allowedTypes := []string{"TABLES", "SEQUENCES", "FUNCTIONS", "ROUTINES", "TYPES", "SCHEMAS"}
	if !utils.ContainsString(allowedTypes, typeName) {
		return brose_errors.NewIllegalArgumentError("Illegal Value for Type Name: " + typeName)
	}
	// Validate Privileges Parameter
	allowedPrivileges := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "USAGE", "CONNECT", "ALL"}
	for _, privilege := range privileges {
		if !utils.ContainsString(allowedPrivileges, privilege) {
			return brose_errors.NewIllegalArgumentError("Illegal Value for Privileges: " + privilege)
		}
	}
	// Run in Database
	err := s.runInDatabase(databaseName, func(ctx context.Context, con *sql.Conn) error {
		joinedPrivileges := strings.Join(privileges, ", ")
		query := "alter default privileges in schema  %s grant " + joinedPrivileges + " on " + typeName + " to  %s;"
		_, err := con.ExecContext(ctx, fmt.Sprintf(query, schemaName, roleName))
		return err
	})
	return err
}

func (s *PgServerApiImpl) DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error {
	return s.runInDatabase(databaseName, func(ctx context.Context, con *sql.Conn) error {
		// This gets executed on the database `databaseName`
		const query = "revoke all on schema %s from %s;"
		_, err := con.ExecContext(ctx, formatQueryObj(query, schemaName, role))
		if err != nil {
			return err
		}
		return nil
	})
}

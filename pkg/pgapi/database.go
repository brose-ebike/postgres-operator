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
	"strings"

	"github.com/brose-ebike/postgres-operator/pkg/brose_errors"
	_ "github.com/lib/pq"
)

// PgDatabaseAPI provides functionality to check and manipulate
// databases, database ownership and privileges on databases
type PgDatabaseAPI interface {
	// IsDatabaseExisting returns true if a database
	// with the given name exists on the connected instance and false if not.
	IsDatabaseExisting(databaseName string) (bool, error)
	// CreateDatabase creates a new database on the connected instance
	CreateDatabase(databaseName string) error
	// DeleteDatabase drops the database with the given name on the connected instance
	DeleteDatabase(databaseName string) error
	// GetDatabaseOwner returns the owner of the database with the given name on the connected instance
	GetDatabaseOwner(databaseName string) (string, error)
	// UpdateDatabaseOwner changes the owner of the database with the given name to the role with the given name
	UpdateDatabaseOwner(databaseName string, roleName string) error
	// ResetDatabaseOwner changes the owner of the database with the given name to the role with which the client is connected
	ResetDatabaseOwner(databaseName string) error
	// UpdateDatabasePrivileges changes the given privileges on the given database for the given role
	UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error
	// IsDatabaseExtensionPresent checks if the given extension is created in the database
	IsDatabaseExtensionPresent(databaseName string, extension string) (bool, error)
	// CreateDatabaseExtension creates the given extension in the database
	CreateDatabaseExtension(databaseName string, extension string) error
}

func (s *pgInstanceAPIImpl) IsDatabaseExisting(databaseName string) (bool, error) {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return false, err
	}
	var exists bool
	const query = "select exists(select * from pg_catalog.pg_database where datname = $1);"
	err = conn.QueryRowContext(s.ctx, query, databaseName).Scan(&exists)
	if err != nil {
		return false, WrapSqlExecutionError(err, query, databaseName)
	}
	return exists, nil
}

func (s *pgInstanceAPIImpl) CreateDatabase(databaseName string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	// Execute Query
	const query = "create database %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(query, databaseName))
	return WrapSqlExecutionError(err, query, databaseName)
}

func (s *pgInstanceAPIImpl) DeleteDatabase(databaseName string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	return s.runAs(conn, s.connectionString.username, func() error {
		// Execute Query
		const query = "drop database %s;"
		_, err = conn.ExecContext(s.ctx, formatQueryObj(query, databaseName))
		return WrapSqlExecutionError(err, query, databaseName)
	})
}

func (s *pgInstanceAPIImpl) UpdateDatabaseOwner(databaseName string, roleName string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	// Execute Query
	const queryGrant = "grant %s to %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryGrant, roleName, s.connectionString.username))
	if err != nil {
		return WrapSqlExecutionError(err, queryGrant, databaseName, s.connectionString.username)
	}
	// Execute Query
	const queryAlterDBOwner = "alter database %s owner to %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryAlterDBOwner, databaseName, roleName))
	if err != nil {
		return WrapSqlExecutionError(err, queryAlterDBOwner, databaseName, roleName)
	}
	// Execute Query
	const queryRevoke = "revoke %s from %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryRevoke, roleName, s.connectionString.username))
	return WrapSqlExecutionError(err, queryRevoke, databaseName, s.connectionString.username)
}

func (s *pgInstanceAPIImpl) UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error {
	// Validate Privileges Parameter
	databasePrivileges := []string{"CONNECT", "CREATE", "TEMPLATE", "TEMPORARY"}
	for _, privilege := range privileges {
		if !hasElementString(databasePrivileges, privilege) {
			return brose_errors.NewIllegalArgumentError("privileges", privilege, nil)
		}
	}
	// Create Context
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	// TODO replace revoke all with specific revoke for the privileges which are not contained in the slice
	// revoke all
	const queryRevoke = "revoke all on database %s from %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryRevoke, databaseName, roleName))
	if err != nil {
		return WrapSqlExecutionError(err, queryRevoke, databaseName, roleName)
	}
	// no privileges need to be granted
	if len(privileges) == 0 {
		return nil
	}
	joinedPrivileges := strings.Join(privileges, ", ")
	// grant all privileges
	queryGrant := "grant " + joinedPrivileges + " on database %s to %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryGrant, databaseName, roleName))
	return WrapSqlExecutionError(err, queryGrant, databaseName, roleName)
}

func (s *pgInstanceAPIImpl) GetDatabaseOwner(databaseName string) (string, error) {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return "", err
	}
	var databaseOwner string
	const query = "select pg_catalog.pg_get_userbyid(d.datdba) as owner from pg_catalog.pg_database as d where d.datname = $1;"
	err = conn.QueryRowContext(s.ctx, query, databaseName).Scan(&databaseOwner)
	if err != nil {
		return "", WrapSqlExecutionError(err, query, databaseName)
	}
	return databaseOwner, nil
}

func (s *pgInstanceAPIImpl) ResetDatabaseOwner(databaseName string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	oldOwner, err := s.GetDatabaseOwner(databaseName)
	if err != nil {
		return err
	}
	return s.runAs(conn, oldOwner, func() error {
		const query = "alter database %s owner to %s;"
		_, err = conn.ExecContext(s.ctx, formatQueryObj(query, databaseName, s.connectionString.username))
		return WrapSqlExecutionError(err, query, databaseName, s.connectionString.username)
	})
}

func (s *pgInstanceAPIImpl) IsDatabaseExtensionPresent(databaseName string, extension string) (bool, error) {
	var exists bool
	// Execute Query
	err := s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "select exists(SELECT * FROM pg_extension where extname = $1);"
		err := conn.QueryRowContext(s.ctx, query, extension).Scan(&exists)
		return WrapSqlExecutionError(err, query, extension)
	})
	return exists, err
}

func (s *pgInstanceAPIImpl) CreateDatabaseExtension(databaseName string, extension string) error {
	// Execute Query
	return s.runIn(databaseName, func(ctx context.Context, conn *sql.Conn) error {
		const query = "create extension %s;"
		_, err := conn.ExecContext(s.ctx, formatQueryObj(query, extension))
		return WrapSqlExecutionError(err, query, extension)
	})
}

package pgapi

import (
	"strings"

	"github.com/brose-ebike/postgres-controller/pkg/brose_errors"
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
		return false, err
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
	return err
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
		return err
	})
}

func (s *pgInstanceAPIImpl) UpdateDatabaseOwner(databaseName string, roleName string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	// Execute Query
	const queryG = "grant %s to %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryG, roleName, s.connectionString.username))
	if err != nil {
		return err
	}
	// Execute Query
	const queryA = "alter database %s owner to %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryA, databaseName, roleName))
	if err != nil {
		return err
	}
	// Execute Query
	const queryR = "revoke %s from %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryR, roleName, s.connectionString.username))
	return err
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
	const queryR = "revoke all on database %s from %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryR, databaseName, roleName))
	if err != nil {
		return err
	}
	// no privileges need to be granted
	if len(privileges) == 0 {
		return nil
	}
	joinedPrivileges := strings.Join(privileges, ", ")
	// grant all privileges
	queryG := "grant " + joinedPrivileges + " on database %s to %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryG, databaseName, roleName))
	return err
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
		return "", err
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
		return err
	})
}

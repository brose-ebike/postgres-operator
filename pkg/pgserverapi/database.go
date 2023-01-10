package pgserverapi

import (
	"strings"

	"github.com/brose-ebike/postgres-controller/pkg/brose_errors"
	"github.com/brose-ebike/postgres-controller/pkg/utils"
	_ "github.com/lib/pq"
)

func (s *PgServerApiImpl) IsDatabaseExisting(databaseName string) (bool, error) {
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return false, err
	}
	var exists bool
	const query = "select exists(select * from pg_catalog.pg_database where datname = $1);"
	err = con.QueryRowContext(s.ctx, query, databaseName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PgServerApiImpl) CreateDatabase(databaseName string) error {
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return err
	}
	// Execute Query
	const query = "create database %s;"
	_, err = con.ExecContext(s.ctx, formatQueryObj(query, databaseName))
	return err
}

func (s *PgServerApiImpl) DeleteDatabase(databaseName string) error {
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return err
	}
	return s.runAs(con, s.connectionString.username, func() error {
		// Execute Query
		const query = "drop database %s;"
		_, err = con.ExecContext(s.ctx, formatQueryObj(query, databaseName))
		return err
	})
}

func (s *PgServerApiImpl) UpdateDatabaseOwner(databaseName string, roleName string) error {
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return err
	}
	// Execute Query
	const queryG = "grant %s to %s;"
	_, err = con.ExecContext(s.ctx, formatQueryObj(queryG, roleName, s.connectionString.username))
	if err != nil {
		return err
	}
	// Execute Query
	const queryA = "alter database %s owner to %s;"
	_, err = con.ExecContext(s.ctx, formatQueryObj(queryA, databaseName, roleName))
	if err != nil {
		return err
	}
	// Execute Query
	const queryR = "revoke %s from %s;"
	_, err = con.ExecContext(s.ctx, formatQueryObj(queryR, roleName, s.connectionString.username))
	return err
}

func (s *PgServerApiImpl) UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error {
	// Validate Privileges Parameter
	databasePrivileges := []string{"CONNECT", "CREATE", "TEMPLATE", "TEMPORARY"}
	for _, privilege := range privileges {
		if !utils.ContainsString(databasePrivileges, privilege) {
			return brose_errors.NewIllegalArgumentError("Illegal Value for Privileges: " + privilege)
		}
	}
	// Create Context
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return err
	}
	// TODO replace revoke all with specific revoke for the privileges which are not contained in the slice
	// revoke all
	const queryR = "revoke all on database %s from %s;"
	_, err = con.ExecContext(s.ctx, formatQueryObj(queryR, databaseName, roleName))
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
	_, err = con.ExecContext(s.ctx, formatQueryObj(queryG, databaseName, roleName))
	return err
}

func (s *PgServerApiImpl) GetDatabaseOwner(databaseName string) (string, error) {
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return "", err
	}
	var databaseOwner string
	const query = "select pg_catalog.pg_get_userbyid(d.datdba) as owner from pg_catalog.pg_database as d where d.datname = $1;"
	err = con.QueryRowContext(s.ctx, query, databaseName).Scan(&databaseOwner)
	if err != nil {
		return "", err
	}
	return databaseOwner, nil
}

func (s *PgServerApiImpl) ResetDatabaseOwner(databaseName string) error {
	// Connect to Database Server
	con, err := s.newConnection()
	if err != nil {
		return err
	}
	oldOwner, err := s.GetDatabaseOwner(databaseName)
	if err != nil {
		return err
	}
	return s.runAs(con, oldOwner, func() error {
		const query = "alter database %s owner to %s;"
		_, err = con.ExecContext(s.ctx, formatQueryObj(query, databaseName, s.connectionString.username))
		return err
	})
}
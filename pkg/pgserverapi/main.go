package pgserverapi

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PgServerApi interface {
	// Connection Details
	ConnectionString() PgConnectionString
	// Connection
	IsConnected() bool
	TestConnection() error
	// Login Roles
	IsLoginRoleExisting(roleName string) (bool, error)
	CreateLoginRole(name string) error
	DeleteLoginRole(name string) error
	UpdateLoginRolePassword(name string, password string) error
	// Databases
	IsDatabaseExisting(databaseName string) (bool, error)
	CreateDatabase(databaseName string) error
	DeleteDatabase(databaseName string) error
	GetDatabaseOwner(databaseName string) (string, error)
	UpdateDatabaseOwner(databaseName string, roleName string) error
	ResetDatabaseOwner(databaseName string) error
	UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error
	// Schema
	IsSchemaInDatabase(databaseName string, schemaName string) (bool, error)
	CreateSchema(databaseName string, schemaName string) error
	DeleteSchema(databaseName string, schemaName string) error
	UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error
	DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error
}

type PgServerApiImpl struct {
	name             string
	connectionString PgConnectionString
	ctx              context.Context
	instance         *sql.DB
}

func NewPgServerApi(ctx context.Context, name string, connectionString PgConnectionString) (PgServerApi, error) {
	api := PgServerApiImpl{
		name,
		connectionString,
		ctx,
		nil,
	}
	if err := api.connect(); err != nil {
		return nil, err
	}
	// Auto disconnect when context is done
	go func() {
		<-ctx.Done()
		api.disconnect()
	}()
	return &api, nil
}

// isMember determines if roleA is a member of roleB
func (s *PgServerApiImpl) isMember(con *sql.Conn, roleA string, roleB string) (bool, error) {
	var result bool
	const query = "select pg_has_role(%s, %s, 'member');"
	sqlRow := con.QueryRowContext(s.ctx, formatQueryValue(query, roleA, roleB))
	if err := sqlRow.Scan(&result); err != nil {
		return false, err
	}
	return result, nil
}

func (s *PgServerApiImpl) runAs(con *sql.Conn, role string, runner func() error) error {
	myRole := s.connectionString.username
	isMember, err := s.isMember(con, myRole, role)
	if err != nil {
		return err
	}
	// Grant role to myRole
	if !isMember {
		const queryG = "grant %s to %s;"
		_, err := con.ExecContext(s.ctx, fmt.Sprintf(queryG, role, myRole))
		if err != nil {
			return err
		}
	}
	// Execute runner
	err = runner()
	// Revoke role to myRole
	if !isMember {
		const queryR = "revoke %s from %s;"
		_, err := con.ExecContext(s.ctx, fmt.Sprintf(queryR, role, myRole))
		if err != nil {
			return err
		}
	}
	return err
}

func (s *PgServerApiImpl) runInDatabase(database string, runner func(ctx context.Context, con *sql.Conn) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectionString := s.connectionString.copy()
	connectionString.database = database
	db, err := sql.Open("postgres", connectionString.toString())
	if err != nil {
		return err
	}

	// Connect to Database Server
	con, err := db.Conn(ctx)
	if err != nil {
		return err
	}

	err = runner(ctx, con)

	if err := con.Close(); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}

	return err
}

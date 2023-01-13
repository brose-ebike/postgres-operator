package pgapi

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PgInstanceAPI represents the full functionality of the API to a postgres instance of a cluster
// The implementation for this interface can be created by NewPgInstanceAPI
// Instead of using this interface directly a client should implement its own interfaces or use one of the provided interfaces like
// PgConnector, PgLoginRoleAPI, PgDatabaseAPI or PgSchemaAPI
type PgInstanceAPI interface {
	PgConnector
	PgLoginRoleAPI
	PgDatabaseAPI
	PgSchemaAPI
}

// NewPgInstanceAPI creates an implementation for the PgInstanceAPI interface
func NewPgInstanceAPI(ctx context.Context, name string, connectionString *PgConnectionString) (PgInstanceAPI, error) {
	logger := log.FromContext(ctx)
	api := pgInstanceAPIImpl{
		name,
		*connectionString,
		ctx,
		nil,
	}
	if err := api.connect(); err != nil {
		logger.Error(err, "Unable to connect")
		return nil, err
	}
	// Auto disconnect when context is done
	go func() {
		<-ctx.Done()
		if err := api.disconnect(); err != nil {
			logger.Error(err, "Unable to disconnect")
		}
	}()
	return &api, nil
}

// Implementation

type pgInstanceAPIImpl struct {
	name             string
	connectionString PgConnectionString
	// ctx is the global context in which the PgInstanceAPI is available
	// It is current best practice to utilize context as arguments, see https://go.dev/blog/context-and-structs
	// but in this struct should only be available until the request context finishes.
	// Therefore the same context would be used in all calls.
	// If the clients need to set other contexts we need to refactor this struct and all methods!
	ctx      context.Context
	instance *sql.DB
}

// isMember determines if roleA is a member of roleB
func (s *pgInstanceAPIImpl) isMember(con *sql.Conn, roleA, roleB string) (bool, error) {
	var result bool
	const query = "select pg_has_role(%s, %s, 'member');"
	sqlRow := con.QueryRowContext(s.ctx, formatQueryValue(query, roleA, roleB))
	if err := sqlRow.Scan(&result); err != nil {
		return false, err
	}
	return result, nil
}

func (s *pgInstanceAPIImpl) runAs(con *sql.Conn, role string, runner func() error) error {
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

func (s *pgInstanceAPIImpl) runInDatabase(database string, runner func(ctx context.Context, conn *sql.Conn) error) error {
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

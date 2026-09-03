/*
Copyright 2026 Yamaha Motor eBike Systems GmbH.

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
	"errors"
	"sync"

	_ "github.com/lib/pq"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PgInstanceAPI represents the full functionality of the API to a postgres instance of a cluster
// The implementation for this interface can be created by NewPgInstanceAPI
// Instead of using this interface directly a client should implement its own interfaces or use one of the provided interfaces like
// PgConnector, PgRoleAPI, PgDatabaseAPI or PgSchemaAPI
type PgInstanceAPI interface {
	PgConnector
	PgRoleAPI
	PgDatabaseAPI
	PgSchemaAPI
}

// NewPgInstanceAPI creates an implementation for the PgInstanceAPI interface
func NewPgInstanceAPI(ctx context.Context, name string, connectionString *PgConnectionString) (PgInstanceAPI, error) {
	logger := log.FromContext(ctx)
	api := pgInstanceAPIImpl{
		name:             name,
		connectionString: *connectionString,
		ctx:              ctx,
		databases:        map[string]*sql.DB{},
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
	ctx context.Context
	// mu protects instance and databases below. Both are mutated by the
	// background auto-disconnect goroutine started in NewPgInstanceAPI as
	// soon as ctx is cancelled, which can happen concurrently with an
	// in-flight call from whichever goroutine is actively reconciling -
	// without a shared lock, that read and this write race on the same
	// memory with no synchronization.
	mu       sync.Mutex
	instance *sql.DB
	// databases caches one connection pool per target database for the
	// lifetime of this pgInstanceAPIImpl (i.e. for one Reconcile() call), so
	// repeated runIn/runInAs calls against the same database reuse a
	// connection instead of opening (and leaking) a new one every time.
	databases map[string]*sql.DB
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

// withBorrowedRole temporarily grants `role` to the connecting role for the
// duration of `runner`, then revokes it again - even if `runner` returns an
// error or panics. If the connecting role is already a member of `role`, no
// grant or revoke is issued at all. This is the shared implementation behind
// both runAs and runInAs.
func (s *pgInstanceAPIImpl) withBorrowedRole(con *sql.Conn, role string, runner func() error) (err error) {
	myRole := s.connectionString.username
	isMember, err := s.isMember(con, myRole, role)
	if err != nil {
		return err
	}

	// Grant role to myRole
	if !isMember {
		const queryGrant = "grant %s to %s;"
		if _, grantErr := con.ExecContext(s.ctx, formatQueryObj(queryGrant, role, myRole)); grantErr != nil {
			return grantErr
		}
	}

	defer func() {
		// Revoke role from myRole. Joined with any runner error rather than
		// silently dropped, so a failing revoke is never invisible even when
		// the runner itself also failed. This also runs when runner panics -
		// deferred functions execute during panic unwinding regardless of
		// whether they call recover(), and the panic resumes automatically
		// once this function returns, so no explicit recover/re-panic is
		// needed here.
		if !isMember {
			const queryRevoke = "revoke %s from %s;"
			_, revokeErr := con.ExecContext(s.ctx, formatQueryObj(queryRevoke, role, myRole))
			if revokeErr != nil {
				err = errors.Join(err, revokeErr)
			}
		}
	}()

	// Execute runner
	err = runner()
	return err
}

func (s *pgInstanceAPIImpl) runAs(con *sql.Conn, role string, runner func() error) error {
	return s.withBorrowedRole(con, role, runner)
}

func (s *pgInstanceAPIImpl) runIn(database string, runner func(ctx context.Context, conn *sql.Conn) error) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Reuse (or lazily create) the connection pool for this database instead
	// of opening a new one for every call
	db, err := s.databaseConn(database)
	if err != nil {
		return err
	}

	// Connect to Database Server
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	// Execute commands
	err = runner(ctx, conn)
	return err
}

func (s *pgInstanceAPIImpl) runInAs(database string, role string, runner func(ctx context.Context, conn *sql.Conn) error) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Reuse (or lazily create) the connection pool for this database instead
	// of opening a new one for every call
	db, err := s.databaseConn(database)
	if err != nil {
		return err
	}

	// Connect to Database Server
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	err = s.withBorrowedRole(conn, role, func() error {
		return runner(ctx, conn)
	})
	return err
}

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
	"database/sql"
	"errors"

	_ "github.com/lib/pq"
)

// PgConnector provides functionality to check
// the current connection to a Postgres instance
type PgConnector interface {
	// ConnectionString provides the PgConnectionString of the current connection
	ConnectionString() PgConnectionString
	// IsConnected returns the current connection state,
	// true if the connection is established, false if not
	IsConnected() bool
	// TestConnection tries to establish a connection
	// and communicates with the Postgres instance if possible.
	// If the connection cannot be established, or the server does not communicate
	// as expected, an error is returned
	TestConnection() error
}

func (s *pgInstanceAPIImpl) ConnectionString() PgConnectionString {
	return s.connectionString
}

func (s *pgInstanceAPIImpl) connect() error {
	if s.instance != nil {
		return nil
	}
	// Start SQL Database
	db, err := sql.Open("postgres", s.connectionString.toString())
	if err != nil {
		return err
	}

	// Connect to Database Server
	con, err := db.Conn(s.ctx)
	if err != nil {
		return err
	}

	err = con.Close()
	if err != nil {
		return err
	}

	// Connection established
	s.instance = db
	return nil
}

func (s *pgInstanceAPIImpl) disconnect() error {
	var err error
	if s.instance != nil {
		err = s.instance.Close()
		s.instance = nil
	}

	// Close every cached per-database connection pool alongside the main
	// one. This runs regardless of whether the main connection was ever
	// established, since databaseConn can populate this cache independently
	// (via runIn/runInAs) - draining it unconditionally avoids relying on an
	// invariant between the two that isn't otherwise enforced. Every close
	// error is joined rather than only keeping the first, consistent with
	// how withBorrowedRole treats the revoke error.
	s.databasesMu.Lock()
	for database, db := range s.databases {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		delete(s.databases, database)
	}
	s.databasesMu.Unlock()

	return err
}

func (s *pgInstanceAPIImpl) IsConnected() bool {
	return s.instance != nil
}

func (s *pgInstanceAPIImpl) TestConnection() error {
	err := s.connect()
	if err != nil {
		return err
	}

	err = s.instance.PingContext(s.ctx)
	if err != nil {
		return err
	}

	return s.disconnect()
}

func (s *pgInstanceAPIImpl) newConnection() (*sql.Conn, error) {
	// Auto Connect if needed
	if !s.IsConnected() {
		return nil, errors.New("Missing Connection, unable to execute query")
	}
	// Connect to Database Server
	return s.instance.Conn(s.ctx)
}

// databaseConn returns a cached connection pool for the given database,
// creating one lazily on first use. The pool is reused for the lifetime of
// this pgInstanceAPIImpl (i.e. for the duration of one Reconcile() call)
// instead of being opened fresh on every call, and is closed together with
// the rest of the connections in disconnect().
func (s *pgInstanceAPIImpl) databaseConn(database string) (*sql.DB, error) {
	s.databasesMu.Lock()
	defer s.databasesMu.Unlock()

	if db, ok := s.databases[database]; ok {
		return db, nil
	}

	conStr := s.connectionString.copy()
	conStr.database = database
	db, err := sql.Open("postgres", conStr.toString())
	if err != nil {
		return nil, err
	}
	s.databases[database] = db
	return db, nil
}

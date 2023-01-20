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
	if s.instance == nil {
		return nil
	}

	err := s.instance.Close()
	s.instance = nil
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

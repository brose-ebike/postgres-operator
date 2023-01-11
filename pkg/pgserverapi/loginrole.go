package pgserverapi

import (
	"strings"

	_ "github.com/lib/pq"
)

func (s *PgServerAPIImpl) IsLoginRoleExisting(roleName string) (bool, error) {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return false, err
	}
	var exists bool
	const query = "select exists(select * from pg_catalog.pg_user where usename = $1);"
	err = conn.QueryRowContext(s.ctx, query, roleName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PgServerAPIImpl) CreateLoginRole(name string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	// Execute Query
	const query = "create user %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(query, name))
	return err
}

func (s *PgServerAPIImpl) DeleteLoginRole(name string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}

	err = s.runAs(conn, name, func() error {
		// reassign owned objects
		const queryR = "reassign owned by %s to %s;"
		_, err = conn.ExecContext(s.ctx, formatQueryObj(queryR, name, s.connectionString.username))
		if err != nil {
			return err
		}
		// drop all existing privileges
		const queryD = "drop owned by %s;"
		_, err = conn.ExecContext(s.ctx, formatQueryObj(queryD, name))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Execute Drop User
	const queryD = "drop user %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryD, name))
	return err
}

func (s *PgServerAPIImpl) UpdateLoginRolePassword(name string, password string) error {
	// Connect to Database Server
	conn, err := s.newConnection()
	if err != nil {
		return err
	}
	// Escape Password manually because its not an object identifier
	password = strings.ReplaceAll(password, "'", "\\'")
	query := "alter user %s with password '" + password + "' login;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(query, name))
	return err
}

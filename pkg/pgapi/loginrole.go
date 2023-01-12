package pgapi

import (
	"strings"

	_ "github.com/lib/pq"
)

// PgLoginRoleAPI provides functionality to check and manipulate login roles (role with login)
type PgLoginRoleAPI interface {
	// IsLoginRoleExisting returns true if a role
	// with the given name exists on the connected instance and false if not.
	IsLoginRoleExisting(roleName string) (bool, error)
	// CreateLoginRole creates the given role on the connected instance
	CreateLoginRole(name string) error
	// DeleteLoginRole drops the given role from the connected instance
	DeleteLoginRole(name string) error
	// UpdateLoginRolePassword changes the password for the given role
	UpdateLoginRolePassword(name string, password string) error
}

func (s *pgInstanceAPIImpl) IsLoginRoleExisting(roleName string) (bool, error) {
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

func (s *pgInstanceAPIImpl) CreateLoginRole(name string) error {
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

func (s *pgInstanceAPIImpl) DeleteLoginRole(name string) error {
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
		return err
	})
	if err != nil {
		return err
	}

	// Execute Drop User
	const queryD = "drop user %s;"
	_, err = conn.ExecContext(s.ctx, formatQueryObj(queryD, name))
	return err
}

func (s *pgInstanceAPIImpl) UpdateLoginRolePassword(name string, password string) error {
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

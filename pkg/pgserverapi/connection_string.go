package pgserverapi

import (
	"errors"
	"strconv"
	"strings"
)

type PgConnectionString struct {
	hostname string
	port     int
	username string
	password string
	database string
	sslMode  string
}

func NewPgConnectionString(
	hostname string,
	port int,
	username string,
	password string,
	database string,
	sslMode string,
) (*PgConnectionString, error) {
	if port < 0 || port > 65536 {
		return nil, errors.New("value for port is out of range 0..65536")
	}
	return &PgConnectionString{
		hostname,
		port,
		username,
		password,
		database,
		sslMode,
	}, nil
}

func (pgcs *PgConnectionString) toString() string {
	result := ""
	if pgcs.hostname != "" {
		result += "host=" + pgcs.hostname + " "
	}
	if pgcs.port != 5432 {
		result += "port=" + strconv.Itoa(pgcs.port) + " "
	}
	if pgcs.username != "" {
		result += "user=" + pgcs.username + " "
	}
	if pgcs.password != "" {
		result += "password=" + pgcs.password + " "
	}
	if pgcs.database != "" {
		result += "dbname=" + pgcs.database + " "
	}
	if pgcs.sslMode != "" {
		result += "sslmode=" + pgcs.sslMode + " "
	}
	return strings.TrimSpace(result)
}

func (pgcs *PgConnectionString) Hostname() string {
	return pgcs.hostname
}

func (pgcs *PgConnectionString) Port() int {
	return pgcs.port
}

func (pgcs *PgConnectionString) Username() string {
	return pgcs.username
}

func (pgcs *PgConnectionString) Password() string {
	return pgcs.password
}

func (pgcs *PgConnectionString) Database() string {
	return pgcs.database
}

func (pgcs *PgConnectionString) SslMode() string {
	return pgcs.sslMode
}

func (pgcs *PgConnectionString) copy() *PgConnectionString {
	return &PgConnectionString{
		hostname: pgcs.hostname,
		port:     pgcs.port,
		username: pgcs.username,
		password: pgcs.password,
		database: pgcs.database,
		sslMode:  pgcs.sslMode,
	}
}

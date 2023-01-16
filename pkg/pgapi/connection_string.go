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

func (pgcs *PgConnectionString) SSLMode() string {
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

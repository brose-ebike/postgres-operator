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

import "testing"

func TestNewPgConnectionString(t *testing.T) {
	pgCS, err := NewPgConnectionString("hostname", 1234, "username", "password", "database", "none")
	if err != nil {
		t.Errorf("Create connection string failed")
	}
	if pgCS == nil {
		t.Errorf("Connection string is nil")
	}
}

func TestNewPgConnectionStringWithNegativePort(t *testing.T) {
	_, err := NewPgConnectionString("hostname", -1, "username", "password", "database", "none")
	if err == nil {
		t.Errorf("Create connection string succeeded with invalid port")
	}
}

func TestPgConnectionStringProperties(t *testing.T) {
	pgCS, err := NewPgConnectionString("hostname", 1234, "username", "password", "database", "none")
	if err != nil {
		t.Errorf("Create connection string failed")
	}
	if pgCS.Hostname() != "hostname" {
		t.Errorf("Hostname is invalid")
	}
	if pgCS.Port() != 1234 {
		t.Errorf("Port is invalid")
	}
	if pgCS.Username() != "username" {
		t.Errorf("Username is invalid")
	}
	if pgCS.Password() != "password" {
		t.Errorf("Password is invalid")
	}
	if pgCS.Database() != "database" {
		t.Errorf("Database is invalid")
	}
	if pgCS.SSLMode() != "none" {
		t.Errorf("SslMode is invalid")
	}
}

func TestPgConnectionStringCopy(t *testing.T) {
	pgCS, err := NewPgConnectionString("hostname", 1234, "username", "password", "database", "none")
	if err != nil {
		t.Errorf("Create connection string failed")
	}
	pgCScopy := pgCS.copy()
	if pgCS == pgCScopy {
		t.Errorf("equal references for different objects")
	}
}

func TestPgConnectionStringToString(t *testing.T) {
	pgCS, err := NewPgConnectionString("hostname", 1234, "username", "password", "database", "none")
	if err != nil {
		t.Errorf("Create connection string failed")
	}
	actual := pgCS.toString()
	if actual != "host=hostname port=1234 user=username password=password dbname=database sslmode=none" {
		t.Errorf("Postgres Connection String: %s", actual)
	}
}

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

import "context"

/*
	This file contains mocks, which can be used until the real implementation is available
*/

type PgInstanceAPI interface {
	TestConnection() error
}

type PgConnectionString interface {
}

type pgInstanceAPIImpl struct {
}

// Implementations

type pgConnectionStringImpl struct {
}

func (a *pgInstanceAPIImpl) TestConnection() error {
	return nil
}

func NewPgConnectionString(
	hostname string,
	port int,
	username string,
	password string,
	database string,
	sslMode string,
) (PgConnectionString, error) {
	return &pgConnectionStringImpl{}, nil
}

func NewPgInstanceAPI(ctx context.Context, name string, connectionString PgConnectionString) (PgInstanceAPI, error) {
	return &pgInstanceAPIImpl{}, nil
}

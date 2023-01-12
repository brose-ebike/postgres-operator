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

package services

import (
	"context"

	apiV1 "github.com/brose-ebike/postgres-controller/api/v1"
	"github.com/brose-ebike/postgres-controller/pkg/pgapi"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewPgInstanceAPI(ctx context.Context, r client.Reader, instance apiV1.PgInstance) (pgapi.PgInstanceAPI, error) {
	logger := log.FromContext(ctx)
	namespace := instance.Namespace
	hostname, err := instance.Spec.GetHostname(ctx, r, namespace)
	if err != nil {
		logger.Error(err, "Unable to read the value for the host property")
		return nil, err
	}

	port, err := instance.Spec.GetPort(ctx, r, namespace)
	if err != nil {
		logger.Error(err, "Unable to read the value for the host property")
		return nil, err
	}

	username, err := instance.Spec.GetUsername(ctx, r, namespace)
	if err != nil {
		logger.Error(err, "Unable to read the value for the username property")
		return nil, err
	}

	password, err := instance.Spec.GetPassword(ctx, r, namespace)
	if err != nil {
		logger.Error(err, "Unable to read the value for the password property")
		return nil, err
	}

	database, err := instance.Spec.GetDatabase(ctx, r, namespace)
	if err != nil {
		logger.Error(err, "Unable to read the value for the database property")
		return nil, err
	}

	sslMode, err := instance.Spec.GetSSLMode(ctx, r, namespace)
	if err != nil {
		logger.Error(err, "Unable to read the value for the sslMode property")
		return nil, err
	}

	connectionString, err := pgapi.NewPgConnectionString(
		hostname,
		port,
		username,
		password,
		database,
		sslMode,
	)

	if err != nil {
		logger.Error(err, "Unable to create the postgresql connection string")
		return nil, err
	}

	pgApi, err := pgapi.NewPgInstanceAPI(ctx, instance.Name, connectionString)
	if err != nil {
		logger.Error(err, "Unable to connect to the Postgres instance")
		return nil, err
	}
	return pgApi, nil
}

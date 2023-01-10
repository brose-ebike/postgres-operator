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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
	This file contains mocks, which can be used until the real implementation is available
*/

type PgServerApi interface {
	TestConnection() error
}

type PgServerApiFactory = func(ctx context.Context, r client.Reader, instance apiV1.PgInstance) (PgServerApi, error)

func NewPgServerApiFromObject(ctx context.Context, r client.Reader, instance apiV1.PgInstance) (PgServerApi, error) {
	return &PgServerApiImpl{}, nil
}

// Another module

type PgServerApiImpl struct {
}

func (a *PgServerApiImpl) TestConnection() error {
	return nil
}

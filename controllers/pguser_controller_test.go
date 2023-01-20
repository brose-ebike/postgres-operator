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

package controllers

import (
	"context"
	"errors"

	apiV1 "github.com/brose-ebike/postgres-controller/api/v1"
	"github.com/brose-ebike/postgres-controller/pkg/pgapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pgRoleMock struct{}

func (r *pgRoleMock) IsRoleExisting(roleName string) (bool, error) {
	return false, nil
}

func (r *pgRoleMock) CreateRole(name string) error {
	return nil
}

func (r *pgRoleMock) DeleteRole(name string) error {
	return nil
}

func (r *pgRoleMock) UpdateUserPassword(name string, password string) error {
	return nil
}

func (r *pgRoleMock) ConnectionString() pgapi.PgConnectionString {
	return pgapi.PgConnectionString{}
}

func (r *pgRoleMock) TestConnection() error {
	return nil
}

func (r *pgRoleMock) IsConnected() bool {
	return false
}

func (r *pgRoleMock) CreateDatabase(name string) error {
	return nil
}

func (r *pgRoleMock) DeleteDatabase(name string) error {
	return nil
}

func (r *pgRoleMock) GetDatabaseOwner(name string) (string, error) {
	return "", nil
}

func (r *pgRoleMock) IsDatabaseExisting(name string) (bool, error) {
	return false, nil
}

func (r *pgRoleMock) ResetDatabaseOwner(name string) error {
	return nil
}

func (r *pgRoleMock) UpdateDatabaseOwner(name string, owner string) error {
	return nil
}

func (r *pgRoleMock) UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error {
	return nil
}

var _ = Describe("PgUserReconciler", func() {

	var pgApiMock PgRoleAPI
	var reconciler *PgUserReconciler

	BeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Create ApiMock
		pgApiMock = &pgRoleMock{}

		// Create Reconciler
		reconciler = &PgUserReconciler{
			k8sClient,
			nil,
			func(ctx context.Context, r client.Reader, instance *apiV1.PgInstance) (PgRoleAPI, error) {
				if instance.Name == "failure" {
					return nil, errors.New("Connection Failure")
				}
				return pgApiMock, nil
			},
		}

		// Create dummy
		createDummy := func() {
			instance := apiV1.PgInstance{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgInstance",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "dummy",
				},
				Spec: apiV1.PgInstanceSpec{
					Hostname: apiV1.PgProperty{Value: "localhost"},
					Port:     apiV1.PgProperty{Value: "5432"},
					Username: apiV1.PgProperty{Value: "admin"},
					Password: apiV1.PgProperty{Value: "password"},
				},
				Status: apiV1.PgInstanceStatus{},
			}
			err := k8sClient.Create(ctx, &instance)
			Expect(err).To(BeNil())
		}
		createDummy()
		// Next Instance
		createFailure := func() {
			instance := apiV1.PgInstance{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgInstance",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "failure",
				},
				Spec: apiV1.PgInstanceSpec{
					Hostname: apiV1.PgProperty{Value: "failure"},
					Port:     apiV1.PgProperty{Value: "5432"},
					Username: apiV1.PgProperty{Value: "admin"},
					Password: apiV1.PgProperty{Value: "password"},
				},
				Status: apiV1.PgInstanceStatus{},
			}
			err := k8sClient.Create(ctx, &instance)
			Expect(err).To(BeNil())
		}
		createFailure()
	})

	AfterEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Instances
		instance := apiV1.PgInstance{}
		opts := []client.DeleteAllOfOption{
			client.InNamespace("default"),
			client.GracePeriodSeconds(5),
		}
		err := k8sClient.DeleteAllOf(ctx, &instance, opts...)
		Expect(err).To(BeNil())
	})

	It("dummy", func() {
		Expect(pgApiMock).NotTo(BeNil())
		Expect(reconciler).NotTo(BeNil())
	})
})
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type pgDatabaseMock struct {
}

func (a *pgDatabaseMock) IsDatabaseExisting(databaseName string) (bool, error) {
	return true, nil
}

func (a *pgDatabaseMock) CreateDatabase(databaseName string) error {
	return nil
}

func (a *pgDatabaseMock) DeleteDatabase(databaseName string) error {
	return nil
}

func (a *pgDatabaseMock) GetDatabaseOwner(databaseName string) (string, error) {
	return "", nil
}

func (a *pgDatabaseMock) UpdateDatabaseOwner(databaseName string, roleName string) error {
	return nil
}

func (a *pgDatabaseMock) ResetDatabaseOwner(databaseName string) error {
	return nil
}

func (a *pgDatabaseMock) UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error {
	return nil
}

func (a *pgDatabaseMock) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	return true, nil
}

func (a *pgDatabaseMock) CreateSchema(databaseName string, schemaName string) error {
	return nil
}

func (a *pgDatabaseMock) DeleteSchema(databaseName string, schemaName string) error {
	return nil
}

func (a *pgDatabaseMock) UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	return nil
}

func (a *pgDatabaseMock) DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error {
	return nil
}

var _ = Describe("PgInstanceReconciler", func() {

	var pgApiMock PgDatabaseAPI
	var reconciler *PgDatabaseReconciler

	BeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Create ApiMock
		pgApiMock = &pgDatabaseMock{}

		// Create Reconciler
		reconciler = &PgDatabaseReconciler{
			k8sClient,
			nil,
			func(ctx context.Context, r client.Reader, instance *apiV1.PgInstance) (PgDatabaseAPI, error) {
				if instance.Name == "failure" {
					return nil, errors.New("Connection Failure")
				}
				return pgApiMock, nil
			},
		}

		// Create instance
		createInstance := func() {
			instance := apiV1.PgInstance{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgInstance",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "instance",
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
		createInstance()
		// Create instance
		createDatabase := func() {
			instance := apiV1.PgDatabase{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgDatabase",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "dummy",
				},
				Spec: apiV1.PgDatabaseSpec{
					Instance: apiV1.PgInstanceRef{
						Namespace: "default",
						Name:      "instance",
					},
					DefaultPrivileges: []apiV1.PgDatabaseDefaultPrivileges{},
					DeletionBehavior: apiV1.PgDatabaseDeletion{
						Drop: false,
						Wait: false,
					},
					PublicPrivileges: apiV1.PgDatabasePublicPrivileges{},
					PublicSchema:     apiV1.PgDatabasePublicSchema{},
				},
				Status: apiV1.PgDatabaseStatus{},
			}
			err := k8sClient.Create(ctx, &instance)
			Expect(err).To(BeNil())
		}
		createDatabase()
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
		//Databases
		databases := apiV1.PgDatabaseList{}
		err = k8sClient.List(ctx, &databases)
		Expect(err).To(BeNil())
		for _, db := range databases.Items {
			db.Finalizers = []string{}
			err = k8sClient.Update(ctx, &db)
			Expect(err).To(BeNil())
		}
		database := apiV1.PgDatabase{}
		err = k8sClient.DeleteAllOf(ctx, &database, opts...)
		Expect(err).To(BeNil())
	})

	It("reconciles on create of PgDatabase", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}
		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())

		// and
		var database apiV1.PgDatabase
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		Expect(database.Status.Conditions).To(HaveLen(1))
		Expect(database.Status.Conditions[0].Status).To(Equal(metaV1.ConditionTrue))
	})

	It("reconciles on delete of PgDatabase", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "missing",
			},
		}
		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())

		// and
		Expect(nil).To(BeNil())
	})
})

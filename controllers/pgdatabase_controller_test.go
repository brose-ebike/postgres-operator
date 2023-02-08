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

	kErrors "k8s.io/apimachinery/pkg/api/errors"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type dummyDB struct {
	owner   string
	schemas map[string]string
}

type pgDatabaseMock struct {
	databases                         map[string]dummyDB
	callsIsDatabaseExisting           int
	callsCreateDatabase               int
	callsDeleteDatabase               int
	callsGetDatabaseOwner             int
	callsUpdateDatabaseOwner          int
	callsResetDatabaseOwner           int
	callsUpdateDatabasePrivileges     int
	callsIsSchemaInDatabase           int
	callsCreateSchema                 int
	callsDeleteSchema                 int
	callsUpdateDefaultPrivileges      int
	callsDeleteAllPrivilegesOnSchema  int
	callsIsDatabaseExtensionPresent   int
	callsCreateDatabaseExtension      int
	callsUpdatePrivilegesOnAllObjects int
	callsIsSchemaUsable               int
	callsMakeSchemaUseable            int
	callsUpdateSchemaPrivileges       int
	callsGetSchemaOwner               int
}

func (m *pgDatabaseMock) IsDatabaseExisting(databaseName string) (bool, error) {
	m.callsIsDatabaseExisting += 1
	_, exists := m.databases[databaseName]
	return exists, nil
}

func (m *pgDatabaseMock) CreateDatabase(databaseName string) error {
	m.callsCreateDatabase += 1
	if _, exists := m.databases[databaseName]; exists {
		return errors.New("Database already exists")
	}
	m.databases[databaseName] = dummyDB{
		owner: "pgadmin",
	}
	return nil
}

func (m *pgDatabaseMock) DeleteDatabase(databaseName string) error {
	m.callsDeleteDatabase += 1
	delete(m.databases, databaseName)
	return nil
}

func (m *pgDatabaseMock) GetDatabaseOwner(databaseName string) (string, error) {
	m.callsGetDatabaseOwner += 1
	value, exists := m.databases[databaseName]
	if !exists {
		return "", errors.New("Database does not exist")
	}
	return value.owner, nil
}

func (m *pgDatabaseMock) UpdateDatabaseOwner(databaseName string, roleName string) error {
	m.callsUpdateDatabaseOwner += 1
	value, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	value.owner = roleName
	return nil
}

func (m *pgDatabaseMock) ResetDatabaseOwner(databaseName string) error {
	m.callsResetDatabaseOwner += 1
	value, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	value.owner = "pgadmin"
	return nil
}

func (m *pgDatabaseMock) UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error {
	m.callsUpdateDatabasePrivileges += 1
	_, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	return nil
}

func (m *pgDatabaseMock) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	m.callsIsSchemaInDatabase += 1
	_, exists := m.databases[databaseName]
	if !exists {
		return false, errors.New("Database does not exist")
	}
	return true, nil
}

func (m *pgDatabaseMock) CreateSchema(databaseName string, schemaName string) error {
	m.callsCreateSchema += 1
	_, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	return nil
}

func (m *pgDatabaseMock) DeleteSchema(databaseName string, schemaName string) error {
	m.callsDeleteSchema += 1
	_, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	return nil
}

func (m *pgDatabaseMock) UpdateDefaultPrivileges(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	m.callsUpdateDefaultPrivileges += 1
	_, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	return nil
}

func (m *pgDatabaseMock) DeleteAllPrivilegesOnSchema(databaseName string, schemaName string, role string) error {
	m.callsDeleteAllPrivilegesOnSchema += 1
	_, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	return nil
}

func (m *pgDatabaseMock) IsDatabaseExtensionPresent(databaseName string, extension string) (bool, error) {
	m.callsIsDatabaseExtensionPresent += 1
	return true, nil
}

func (m *pgDatabaseMock) CreateDatabaseExtension(databaseName string, extension string) error {
	m.callsCreateDatabaseExtension += 1
	return nil
}

func (m *pgDatabaseMock) UpdatePrivilegesOnAllObjects(databaseName string, schemaName string, roleName string, typeName string, privileges []string) error {
	m.callsUpdatePrivilegesOnAllObjects += 1
	return nil
}

func (m *pgDatabaseMock) IsSchemaUsable(databaseName string, schemaName string) (bool, error) {
	m.callsIsSchemaUsable += 1
	return true, nil
}

func (m *pgDatabaseMock) MakeSchemaUseable(databaseName string, schemaName string) error {
	m.callsMakeSchemaUseable += 1
	return nil
}

func (m *pgDatabaseMock) UpdateSchemaPrivileges(databaseName string, schemaName string, roleName string, privileges []string) error {
	m.callsUpdateSchemaPrivileges += 1
	return nil
}

func (m *pgDatabaseMock) GetSchemaOwner(databaseName string, schemaName string) (string, error) {
	m.callsGetSchemaOwner += 1
	return "", nil
}

var _ = Describe("PgInstanceReconciler", func() {

	var pgApiMock PgDatabaseAPI
	var reconciler *PgDatabaseReconciler

	BeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Create ApiMock
		pgApiMock = &pgDatabaseMock{
			databases: make(map[string]dummyDB),
		}

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
					Extensions:        []string{},
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
		Expect(database.Status.Conditions).To(HaveLen(3))
		// and Connected Condition is true
		connectionCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgConnectedConditionType)
		Expect(connectionCondition.Status).To(Equal(v1.ConditionTrue))
		// and Database Exists Condition is true
		databaseCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseExistsConditionType)
		Expect(databaseCondition.Status).To(Equal(v1.ConditionTrue))
		// and Extensions Exists Condition is true
		extensionCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseExtensionsConditionType)
		Expect(extensionCondition.Status).To(Equal(v1.ConditionTrue))

		// and
		database = apiV1.PgDatabase{}
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		Expect(database.Finalizers).To(HaveLen(1))

		// and
		mock := pgApiMock.(*pgDatabaseMock)
		Expect(mock.callsCreateDatabase).To(Equal(1))
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

	It("reconciles on finalize of PgDatabase", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}
		_, err := reconciler.Reconcile(ctx, request)
		Expect(err).To(BeNil())

		// and
		database := apiV1.PgDatabase{}
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		err = k8sClient.Delete(ctx, &database)
		Expect(err).To(BeNil())

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())

		// and
		database = apiV1.PgDatabase{}
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(kErrors.IsNotFound(err)).To(BeTrue())
	})
})

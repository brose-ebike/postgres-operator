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

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/pgapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type pgRoleMock struct {
	databases                       map[string]dummyDB
	roles                           map[string]bool
	callsIsRoleExisting             int
	callsCreateRole                 int
	callsDeleteRole                 int
	callsUpdateUserPassword         int
	callsConnectionString           int
	callsTestConnection             int
	callsIsConnected                int
	callsCreateDatabase             int
	callsDeleteDatabase             int
	callsGetDatabaseOwner           int
	callsIsDatabaseExisting         int
	callsResetDatabaseOwner         int
	callsUpdateDatabaseOwner        int
	callsUpdateDatabasePrivileges   int
	callsIsDatabaseExtensionPresent int
	callsCreateDatabaseExtension    int
}

func (r *pgRoleMock) IsRoleExisting(roleName string) (bool, error) {
	r.callsIsRoleExisting += 1
	_, exists := r.roles[roleName]
	return exists, nil
}

func (r *pgRoleMock) CreateRole(name string) error {
	r.callsCreateRole += 1
	return nil
}

func (r *pgRoleMock) DeleteRole(name string) error {
	r.callsDeleteRole += 1
	return nil
}

func (r *pgRoleMock) UpdateUserPassword(name string, password string) error {
	r.callsUpdateUserPassword += 1
	return nil
}

func (r *pgRoleMock) ConnectionString() pgapi.PgConnectionString {
	r.callsConnectionString += 1
	return pgapi.PgConnectionString{}
}

func (r *pgRoleMock) TestConnection() error {
	r.callsTestConnection += 1
	return nil
}

func (r *pgRoleMock) IsConnected() bool {
	r.callsIsConnected += 1
	return false
}

func (r *pgRoleMock) CreateDatabase(databaseName string) error {
	r.callsCreateDatabase += 1
	if _, exists := r.databases[databaseName]; exists {
		return errors.New("Database already exists")
	}
	r.databases[databaseName] = dummyDB{
		owner: "pgadmin",
	}
	return nil
}

func (r *pgRoleMock) DeleteDatabase(name string) error {
	r.callsDeleteDatabase += 1
	return nil
}

func (r *pgRoleMock) GetDatabaseOwner(name string) (string, error) {
	r.callsGetDatabaseOwner += 1
	return "", nil
}

func (r *pgRoleMock) IsDatabaseExisting(databaseName string) (bool, error) {
	r.callsIsDatabaseExisting += 1
	_, exists := r.databases[databaseName]
	return exists, nil
}

func (r *pgRoleMock) ResetDatabaseOwner(name string) error {
	r.callsResetDatabaseOwner += 1
	return nil
}

func (r *pgRoleMock) UpdateDatabaseOwner(name string, owner string) error {
	r.callsUpdateDatabaseOwner += 1
	return nil
}

func (r *pgRoleMock) UpdateDatabasePrivileges(databaseName string, roleName string, privileges []string) error {
	r.callsUpdateDatabasePrivileges += 1
	return nil
}

func (m *pgRoleMock) IsDatabaseExtensionPresent(databaseName string, extension string) (bool, error) {
	m.callsIsDatabaseExtensionPresent += 1
	return true, nil
}

func (m *pgRoleMock) CreateDatabaseExtension(databaseName string, extension string) error {
	m.callsCreateDatabaseExtension += 1
	return nil
}

var _ = Describe("PgUserReconciler", func() {

	var pgApiMock PgRoleAPI
	var reconciler *PgUserReconciler

	BeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Create ApiMock
		pgApiMock = &pgRoleMock{
			databases: map[string]dummyDB{
				"testdb": {
					owner:   "pgadmin",
					schemas: make(map[string]string),
				},
			},
		}

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
		createDummy()
		// Create user
		createUser := func() {
			instance := apiV1.PgUser{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgUser",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "dummy",
				},
				Spec: apiV1.PgUserSpec{
					Instance: apiV1.PgInstanceRef{
						Namespace: "default",
						Name:      "instance",
					},
					Secret: &apiV1.PgUserSecret{
						Name: "credentials",
					},
					Databases: []apiV1.PgUserDatabase{
						{
							Name:       "testdb",
							Owner:      &cFalse,
							Privileges: []apiV1.DatabasePrivilege{},
						},
					},
				},
				Status: apiV1.PgUserStatus{},
			}
			err := k8sClient.Create(ctx, &instance)
			Expect(err).To(BeNil())
		}
		createUser()
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
		users := apiV1.PgUserList{}
		err = k8sClient.List(ctx, &users)
		Expect(err).To(BeNil())
		for _, db := range users.Items {
			db.Finalizers = []string{}
			err = k8sClient.Update(ctx, &db)
			Expect(err).To(BeNil())
		}
		user := apiV1.PgUser{}
		err = k8sClient.DeleteAllOf(ctx, &user, opts...)
		Expect(err).To(BeNil())
	})

	It("reconciles on create of PgUser", func() {
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
		var user apiV1.PgUser
		err = k8sClient.Get(ctx, request.NamespacedName, &user)
		Expect(err).To(BeNil())
		Expect(user.Status.Conditions).To(HaveLen(3))
		// and connection is true
		connectionCondition := meta.FindStatusCondition(user.Status.Conditions, apiV1.PgConnectedConditionType)
		Expect(connectionCondition.Status).To(Equal(v1.ConditionTrue))
		// and user is true
		userCondition := meta.FindStatusCondition(user.Status.Conditions, apiV1.PgUserExistsConditionType)
		Expect(userCondition.Status).To(Equal(v1.ConditionTrue))
		// and database is true
		databaseCondition := meta.FindStatusCondition(user.Status.Conditions, apiV1.PgUserDatabasesExistsConditionType)
		Expect(databaseCondition.Status).To(Equal(v1.ConditionTrue))

		// and
		user = apiV1.PgUser{}
		err = k8sClient.Get(ctx, request.NamespacedName, &user)
		Expect(err).To(BeNil())
		Expect(user.Finalizers).To(HaveLen(1))

		// and
		mock := pgApiMock.(*pgRoleMock)
		Expect(mock.callsCreateRole).To(Equal(1))

		// and
		secret := coreV1.Secret{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "credentials"}, &secret)
		Expect(err).To(BeNil())
		Expect(secret.ObjectMeta.OwnerReferences).To(HaveLen(1))
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

	It("reconciles on finalize of PgUser", func() {
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
		user := apiV1.PgUser{}
		err = k8sClient.Get(ctx, request.NamespacedName, &user)
		Expect(err).To(BeNil())
		err = k8sClient.Delete(ctx, &user)
		Expect(err).To(BeNil())

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())
	})
})

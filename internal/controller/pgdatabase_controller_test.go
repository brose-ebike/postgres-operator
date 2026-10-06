/*
Copyright 2026 Yamaha Motor eBike Systems GmbH.

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

package controller

import (
	"context"
	"errors"
	"time"

	kErrors "k8s.io/apimachinery/pkg/api/errors"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coreV1 "k8s.io/api/core/v1"
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
	forceErr                          map[string]error
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
	if err, ok := m.forceErr["IsDatabaseExisting"]; ok {
		return false, err
	}
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
	if err, ok := m.forceErr["UpdateDatabasePrivileges"]; ok {
		return err
	}
	_, exists := m.databases[databaseName]
	if !exists {
		return errors.New("Database does not exist")
	}
	return nil
}

func (m *pgDatabaseMock) IsSchemaInDatabase(databaseName string, schemaName string) (bool, error) {
	m.callsIsSchemaInDatabase += 1
	if err, ok := m.forceErr["IsSchemaInDatabase"]; ok {
		return false, err
	}
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
	if err, ok := m.forceErr["IsDatabaseExtensionPresent"]; ok {
		return false, err
	}
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
					APIVersion: "postgres.oebc.tools/v1",
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
					APIVersion: "postgres.oebc.tools/v1",
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
		err := deleteAllCustomResources(ctx, k8sClient, "default")
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
		Expect(database.Status.Conditions).To(HaveLen(6))
		// and Connected Condition is true
		connectionCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgConnectedConditionType)
		Expect(connectionCondition.Status).To(Equal(v1.ConditionTrue))
		// and Database Exists Condition is true
		databaseCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseExistsConditionType)
		Expect(databaseCondition.Status).To(Equal(v1.ConditionTrue))
		// and Extensions Exists Condition is true
		extensionCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseExtensionsConditionType)
		Expect(extensionCondition.Status).To(Equal(v1.ConditionTrue))
		// and Default Privileges Condition is true
		defaultPrivilegesCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseDefaultPrivilegesConditionType)
		Expect(defaultPrivilegesCondition.Status).To(Equal(v1.ConditionTrue))
		// and Public Privileges Condition is true
		publicPrivilegesCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabasePublicPrivilegesConditionType)
		Expect(publicPrivilegesCondition.Status).To(Equal(v1.ConditionTrue))
		// and Public Schema Condition is true
		publicSchemaCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabasePublicSchemaConditionType)
		Expect(publicSchemaCondition.Status).To(Equal(v1.ConditionTrue))

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

	It("does not panic and returns an error when the referenced PgInstance does not exist", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		orphan := apiV1.PgDatabase{
			TypeMeta: v1.TypeMeta{
				APIVersion: "postgres.oebc.tools/v1",
				Kind:       "PgDatabase",
			},
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Name:      "orphan",
			},
			Spec: apiV1.PgDatabaseSpec{
				Instance: apiV1.PgInstanceRef{
					Namespace: "default",
					Name:      "does-not-exist",
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
		err := k8sClient.Create(ctx, &orphan)
		Expect(err).To(BeNil())

		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "orphan",
			},
		}

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then reconcile fails cleanly (a panic would abort the test run, not surface here)
		Expect(err).ToNot(BeNil())
		Expect(kErrors.IsNotFound(err)).To(BeTrue())
		Expect(result.RequeueAfter).To(Equal(time.Minute))

		// and the connected condition reflects the missing instance
		database := apiV1.PgDatabase{}
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		connectionCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgConnectedConditionType)
		Expect(connectionCondition).ToNot(BeNil())
		Expect(connectionCondition.Status).To(Equal(v1.ConditionFalse))
		Expect(connectionCondition.Reason).To(Equal(apiV1.PgConnectedConditionReasonInstanceNotFound))
	})

	It("sets the database exists condition to false when creating the database fails", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		pgApiMock.(*pgDatabaseMock).forceErr = map[string]error{
			"IsDatabaseExisting": errors.New("connection refused"),
		}
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).ToNot(BeNil())
		Expect(result.RequeueAfter).To(Equal(time.Minute))

		// and
		database := apiV1.PgDatabase{}
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		databaseCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseExistsConditionType)
		Expect(databaseCondition).ToNot(BeNil())
		Expect(databaseCondition.Status).To(Equal(v1.ConditionFalse))
		Expect(databaseCondition.Reason).To(Equal("DatabaseQueryFailed"))
	})

	It("sets the public privileges condition to false when revoking public privileges fails", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		pgApiMock.(*pgDatabaseMock).forceErr = map[string]error{
			"UpdateDatabasePrivileges": errors.New("permission denied"),
		}
		database := apiV1.PgDatabase{}
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "dummy"}, &database)
		Expect(err).To(BeNil())
		database.Spec.PublicPrivileges = apiV1.PgDatabasePublicPrivileges{Revoke: true}
		err = k8sClient.Update(ctx, &database)
		Expect(err).To(BeNil())

		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).ToNot(BeNil())
		Expect(result.RequeueAfter).To(Equal(time.Minute))

		// and
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		publicPrivilegesCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabasePublicPrivilegesConditionType)
		Expect(publicPrivilegesCondition).ToNot(BeNil())
		Expect(publicPrivilegesCondition.Status).To(Equal(v1.ConditionFalse))
		Expect(publicPrivilegesCondition.Reason).To(Equal(apiV1.PgDatabasePublicPrivilegesConditionReasonFailed))
	})

	It("sets the public schema condition to false when dropping the public schema fails", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		pgApiMock.(*pgDatabaseMock).forceErr = map[string]error{
			"IsSchemaInDatabase": errors.New("connection refused"),
		}
		database := apiV1.PgDatabase{}
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "dummy"}, &database)
		Expect(err).To(BeNil())
		database.Spec.PublicSchema = apiV1.PgDatabasePublicSchema{Drop: true}
		err = k8sClient.Update(ctx, &database)
		Expect(err).To(BeNil())

		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).ToNot(BeNil())
		Expect(result.RequeueAfter).To(Equal(time.Minute))

		// and
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		publicSchemaCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabasePublicSchemaConditionType)
		Expect(publicSchemaCondition).ToNot(BeNil())
		Expect(publicSchemaCondition.Status).To(Equal(v1.ConditionFalse))
		Expect(publicSchemaCondition.Reason).To(Equal(apiV1.PgDatabasePublicSchemaConditionReasonFailed))
	})

	It("sets the extensions condition to false when checking for an extension fails", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		pgApiMock.(*pgDatabaseMock).forceErr = map[string]error{
			"IsDatabaseExtensionPresent": errors.New("connection refused"),
		}
		database := apiV1.PgDatabase{}
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "dummy"}, &database)
		Expect(err).To(BeNil())
		database.Spec.Extensions = []string{"pg_trgm"}
		err = k8sClient.Update(ctx, &database)
		Expect(err).To(BeNil())

		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}

		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).ToNot(BeNil())
		Expect(result.RequeueAfter).To(Equal(time.Minute))

		// and
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		extensionCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseExtensionsConditionType)
		Expect(extensionCondition).ToNot(BeNil())
		Expect(extensionCondition.Status).To(Equal(v1.ConditionFalse))
		Expect(extensionCondition.Reason).To(Equal("ExtensionCheckFailed"))
	})

	It("does not set the backup-policy-found condition when spec.backupPolicy is unset", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: "default", Name: "dummy"},
		}

		_, err := reconciler.Reconcile(ctx, request)
		Expect(err).To(BeNil())

		database := apiV1.PgDatabase{}
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		Expect(meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseBackupPolicyFoundConditionType)).To(BeNil())
	})

	It("sets the backup-policy-found condition to true when the referenced PgBackupPolicy exists", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		policy := apiV1.PgBackupPolicy{
			ObjectMeta: v1.ObjectMeta{Namespace: "default", Name: "backup-policy"},
			Spec: apiV1.PgBackupPolicySpec{
				Schedule: "0 2 * * *",
				Storage: apiV1.PgBackupStorage{
					Type: apiV1.PgBackupStorageTypeS3,
					S3: &apiV1.PgBackupStorageS3{
						Endpoint:  "minio.example.com",
						Bucket:    "pg-backups",
						SecretRef: coreV1.LocalObjectReference{Name: "storage-creds"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())

		database := apiV1.PgDatabase{}
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "dummy"}, &database)
		Expect(err).To(BeNil())
		database.Spec.BackupPolicy = &apiV1.PgInstanceRef{Namespace: "default", Name: "backup-policy"}
		err = k8sClient.Update(ctx, &database)
		Expect(err).To(BeNil())

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
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		backupPolicyCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseBackupPolicyFoundConditionType)
		Expect(backupPolicyCondition).ToNot(BeNil())
		Expect(backupPolicyCondition.Status).To(Equal(v1.ConditionTrue))
		Expect(backupPolicyCondition.Reason).To(Equal(apiV1.PgDatabaseBackupPolicyFoundConditionReasonSucceeded))

		Expect(k8sClient.Delete(ctx, &policy)).To(Succeed())
	})

	It("sets the backup-policy-found condition to false when the referenced PgBackupPolicy does not exist", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		database := apiV1.PgDatabase{}
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "dummy"}, &database)
		Expect(err).To(BeNil())
		database.Spec.BackupPolicy = &apiV1.PgInstanceRef{Namespace: "default", Name: "does-not-exist"}
		err = k8sClient.Update(ctx, &database)
		Expect(err).To(BeNil())

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
		err = k8sClient.Get(ctx, request.NamespacedName, &database)
		Expect(err).To(BeNil())
		backupPolicyCondition := meta.FindStatusCondition(database.Status.Conditions, apiV1.PgDatabaseBackupPolicyFoundConditionType)
		Expect(backupPolicyCondition).ToNot(BeNil())
		Expect(backupPolicyCondition.Status).To(Equal(v1.ConditionFalse))
		Expect(backupPolicyCondition.Reason).To(Equal(apiV1.PgDatabaseBackupPolicyFoundConditionReasonFailed))
	})
})

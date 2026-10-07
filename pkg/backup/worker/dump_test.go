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

package worker

import (
	"context"
	"testing"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("unable to register client-go scheme: %v", err)
	}
	if err := apiV1.AddToScheme(scheme); err != nil {
		t.Fatalf("unable to register postgres-operator scheme: %v", err)
	}
	return scheme
}

func testPolicy(namespace string, name string) *apiV1.PgBackupPolicy {
	return &apiV1.PgBackupPolicy{
		ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: name},
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
}

func testDatabase(namespace string, name string, policyNamespace string, policyName string) *apiV1.PgDatabase {
	return &apiV1.PgDatabase{
		ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: apiV1.PgDatabaseSpec{
			Instance:     apiV1.PgInstanceRef{Namespace: namespace, Name: "does-not-exist"},
			BackupPolicy: &apiV1.PgInstanceRef{Namespace: policyNamespace, Name: policyName},
		},
	}
}

func TestListDatabasesForPolicy(t *testing.T) {
	scheme := newTestScheme(t)
	policyRef := types.NamespacedName{Namespace: "default", Name: "nightly"}

	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(
		testDatabase("default", "matches", "default", "nightly"),
		testDatabase("default", "different-policy", "default", "other-policy"),
		testDatabase("other-ns", "wrong-namespace", "default", "nightly"),
		&apiV1.PgDatabase{ObjectMeta: metaV1.ObjectMeta{Namespace: "default", Name: "no-policy"}, Spec: apiV1.PgDatabaseSpec{Instance: apiV1.PgInstanceRef{Namespace: "default", Name: "x"}}},
	).Build()

	databases, err := listDatabasesForPolicy(context.Background(), c, policyRef)
	if err != nil {
		t.Fatalf("listDatabasesForPolicy failed: %v", err)
	}
	if len(databases) != 1 || databases[0].Name != "matches" {
		names := make([]string, len(databases))
		for i, d := range databases {
			names[i] = d.Name
		}
		t.Fatalf("expected only 'matches', got %v", names)
	}
}

func TestRunDump_PartialFailureIsolation(t *testing.T) {
	scheme := newTestScheme(t)
	policy := testPolicy("default", "nightly")
	dbA := testDatabase("default", "db-a", "default", "nightly")
	dbB := testDatabase("default", "db-b", "default", "nightly")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiV1.PgBackupPolicy{}, &apiV1.PgBackupInstance{}).
		WithRuntimeObjects(policy, dbA, dbB).
		Build()

	err := RunDump(context.Background(), c, types.NamespacedName{Namespace: "default", Name: "nightly"}, t.TempDir())
	if err == nil {
		t.Fatal("expected RunDump to report errors for both databases (neither PgInstance exists)")
	}

	var instances apiV1.PgBackupInstanceList
	if listErr := c.List(context.Background(), &instances); listErr != nil {
		t.Fatalf("unable to list PgBackupInstances: %v", listErr)
	}
	if len(instances.Items) != 2 {
		t.Fatalf("expected one PgBackupInstance per database (partial-failure isolation), got %d", len(instances.Items))
	}
	for _, inst := range instances.Items {
		if inst.Status.Phase != apiV1.PgBackupInstancePhaseFailure {
			t.Errorf("expected instance %s to be Failure, got %s", inst.Name, inst.Status.Phase)
		}
		if inst.Status.Message == "" {
			t.Errorf("expected instance %s to carry a failure message", inst.Name)
		}
		if inst.Status.StartedAt == nil || inst.Status.FinishedAt == nil {
			t.Errorf("expected instance %s to have StartedAt/FinishedAt set", inst.Name)
		}
		owned := false
		for _, ref := range inst.OwnerReferences {
			if ref.Name == policy.Name && ref.Kind == "PgBackupPolicy" {
				owned = true
			}
		}
		if !owned {
			t.Errorf("expected instance %s to be owned by the policy", inst.Name)
		}
	}
}

func TestResolveDumpOptions_AppliesDefaults(t *testing.T) {
	resolved := resolveDumpOptions(apiV1.PgBackupDumpOptions{})
	if resolved.Format != apiV1.PgBackupDumpFormatCustom {
		t.Errorf("expected default format custom, got %s", resolved.Format)
	}
	if resolved.CompressionLevel == nil || *resolved.CompressionLevel != defaultCompressionLevel {
		t.Errorf("expected default compression level %d, got %v", defaultCompressionLevel, resolved.CompressionLevel)
	}

	level := int32(9)
	resolved = resolveDumpOptions(apiV1.PgBackupDumpOptions{Format: apiV1.PgBackupDumpFormatTar, CompressionLevel: &level})
	if resolved.Format != apiV1.PgBackupDumpFormatTar {
		t.Errorf("expected configured format to be preserved, got %s", resolved.Format)
	}
	if resolved.CompressionLevel == nil || *resolved.CompressionLevel != 9 {
		t.Errorf("expected configured compression level to be preserved, got %v", resolved.CompressionLevel)
	}
}

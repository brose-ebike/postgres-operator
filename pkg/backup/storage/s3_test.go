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

package storage

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"k8s.io/utils/ptr"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/tcminio"
)

func setupTestDestination(t *testing.T, ctx context.Context, prefix string) (*s3Destination, func()) {
	t.Helper()

	container, err := tcminio.SetupMinio(ctx, tcminio.WithCredentials("minioadmin", "minioadmin"))
	if err != nil {
		t.Fatalf("unable to setup minio container: %v", err)
	}

	endpoint, err := container.Endpoint(ctx)
	if err != nil {
		t.Fatalf("unable to resolve minio endpoint: %v", err)
	}

	dest, err := newS3Destination(ctx, &apiV1.PgBackupStorageS3{
		Endpoint: endpoint,
		Bucket:   "pg-backups",
		Prefix:   prefix,
		Secure:   ptr.To(false),
	}, container.AccessKey(), container.SecretKey())
	if err != nil {
		t.Fatalf("unable to construct s3 destination: %v", err)
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("unable to terminate minio container: %v", err)
		}
	}
	return dest, cleanup
}

func TestS3Destination_StoreListDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dest, cleanup := setupTestDestination(t, ctx, "/pgbackups/")
	defer cleanup()

	tmpFile := filepath.Join(t.TempDir(), "dump.pgdump")
	if err := os.WriteFile(tmpFile, []byte("dummy dump contents"), 0o600); err != nil {
		t.Fatalf("unable to write temp dump file: %v", err)
	}

	if err := dest.Store(ctx, tmpFile, "db-a/backup-1.dump"); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := dest.Store(ctx, tmpFile, "db-a/backup-2.dump"); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	names, err := dest.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	sort.Strings(names)
	want := []string{"db-a/backup-1.dump", "db-a/backup-2.dump"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Fatalf("expected prefix-stripped names %v, got %v", want, names)
	}

	if err := dest.Delete(ctx, "db-a/backup-1.dump"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	namesAfterDelete, err := dest.List(ctx)
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}
	if len(namesAfterDelete) != 1 || namesAfterDelete[0] != "db-a/backup-2.dump" {
		t.Fatalf("expected only backup-2 to remain, got %v", namesAfterDelete)
	}
}

func TestS3Destination_BucketAutoCreatedAndPrefixSlashesStripped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A leading/trailing-slash prefix must be normalized the same way
	// cloud-pgdumper's S3Destination does.
	dest, cleanup := setupTestDestination(t, ctx, "///nested/prefix///")
	defer cleanup()

	if dest.prefix != "nested/prefix" {
		t.Fatalf("expected prefix to be slash-trimmed to %q, got %q", "nested/prefix", dest.prefix)
	}

	tmpFile := filepath.Join(t.TempDir(), "dump.pgdump")
	if err := os.WriteFile(tmpFile, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("unable to write temp dump file: %v", err)
	}
	if err := dest.Store(ctx, tmpFile, "db-b/backup.dump"); err != nil {
		t.Fatalf("Store into auto-created bucket failed: %v", err)
	}
}

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

package tcminio

import (
	"context"
	"testing"
)

func TestMinioTestContainer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	container, err := SetupMinio(ctx, WithCredentials("minioadmin", "minioadmin"), WithBucket("pg-backups"))
	if err != nil {
		t.Fatalf("unable to setup minio container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("unable to terminate minio container: %v", err)
		}
	}()

	endpoint, err := container.Endpoint(ctx)
	if err != nil {
		t.Fatalf("unable to resolve minio endpoint: %v", err)
	}
	if endpoint == "" {
		t.Errorf("expected a non-empty endpoint")
	}
	if container.AccessKey() != "minioadmin" || container.SecretKey() != "minioadmin" {
		t.Errorf("expected credentials to round-trip, got access=%q secret=%q", container.AccessKey(), container.SecretKey())
	}
	if container.Bucket() != "pg-backups" {
		t.Errorf("expected bucket to round-trip, got %q", container.Bucket())
	}
}

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

// Package storage implements backup storage destinations, mirroring
// cloud-pgdumper's DestinationInterface abstraction
// (src/pgdumper/archive/destination.py) so that adding volume/ftp backends
// later is a new implementation, not a redesign.
package storage

import (
	"context"
	"fmt"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

// Destination stores, lists, and deletes backup objects at one storage
// backend.
type Destination interface {
	// Store uploads the file at sourcePath under the given target key.
	Store(ctx context.Context, sourcePath string, target string) error
	// List returns the target keys currently stored.
	List(ctx context.Context) ([]string, error)
	// Delete removes the object at the given target key.
	Delete(ctx context.Context, target string) error
}

// NewDestination is a factory keyed on cfg.Type, mirroring the
// discriminated union in api/v1.PgBackupStorage.
func NewDestination(ctx context.Context, cfg apiV1.PgBackupStorage, accessKey string, secretKey string) (Destination, error) {
	switch cfg.Type {
	case apiV1.PgBackupStorageTypeS3:
		if cfg.S3 == nil {
			return nil, fmt.Errorf("storage type is %q but s3 configuration is not set", cfg.Type)
		}
		return newS3Destination(ctx, cfg.S3, accessKey, secretKey)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", cfg.Type)
	}
}

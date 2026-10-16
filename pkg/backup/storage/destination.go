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
//
// This package deliberately has no dependency on api/v1: every field it
// needs (endpoint, bucket, prefix, secure, accessKey, secretKey) is taken
// as an already-resolved plain value. PgBackupStorageS3's fields are
// PgProperty (sourceable from a literal value, a ConfigMap, or a Secret -
// see api/v1/pgbackuppolicy_types.go), and resolving those belongs to the
// caller (pkg/backup/worker), which already has the ctx/client/namespace
// on hand when it reconciles - not duplicated or half-duplicated here.
package storage

import (
	"context"
	"fmt"
)

// TypeS3 identifies the S3-compatible storage backend - the only one
// implemented today, mirroring api/v1.PgBackupStorageTypeS3's value
// without importing api/v1 itself.
const TypeS3 = "s3"

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

// NewDestination is a factory keyed on storageType, mirroring the
// discriminated union in api/v1.PgBackupStorage. endpoint/bucket/prefix/
// secure/accessKey/secretKey are all already-resolved values - see the
// package doc comment for why this takes plain strings rather than
// api/v1.PgBackupStorageS3 directly.
func NewDestination(ctx context.Context, storageType string, endpoint string, bucket string, prefix string, secure bool, accessKey string, secretKey string) (Destination, error) {
	switch storageType {
	case TypeS3:
		return newS3Destination(ctx, endpoint, bucket, prefix, secure, accessKey, secretKey)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", storageType)
	}
}

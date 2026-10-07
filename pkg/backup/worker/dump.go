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

// Package worker implements the backup-worker binary's actual orchestration
// (RunDump/RunCleanup) as plain importable functions, so it can be invoked
// as a function call from tests (and the e2e suite) without spawning a
// subprocess - cmd/backup-worker is just a thin CLI shim around this
// package.
package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/backup/dumper"
	"github.com/brose-ebike/postgres-operator/pkg/backup/encryption"
	"github.com/brose-ebike/postgres-operator/pkg/backup/storage"
	"github.com/brose-ebike/postgres-operator/pkg/services"
)

const defaultCompressionLevel = int32(6)

// RunDump discovers every PgDatabase in the policy's namespace whose
// spec.backupPolicy references policyRef, dumps each one (encrypting and
// uploading per the policy's configuration), and creates/patches a
// PgBackupInstance recording the outcome. One database's failure does not
// abort the rest; errors are joined and returned once every database has
// been attempted. workspaceDir is the scratch directory dump artifacts are
// written to before upload (the CronJob mounts this as an emptyDir; tests
// pass a temp directory).
func RunDump(ctx context.Context, c client.Client, policyRef types.NamespacedName, workspaceDir string) error {
	var policy apiV1.PgBackupPolicy
	if err := c.Get(ctx, policyRef, &policy); err != nil {
		return fmt.Errorf("unable to fetch PgBackupPolicy %s: %w", policyRef, err)
	}

	databases, err := listDatabasesForPolicy(ctx, c, policyRef)
	if err != nil {
		return fmt.Errorf("unable to list PgDatabases for policy %s: %w", policyRef, err)
	}

	var errs []error
	for i := range databases {
		if err := dumpOneDatabase(ctx, c, workspaceDir, &policy, &databases[i]); err != nil {
			errs = append(errs, fmt.Errorf("database %s/%s: %w", databases[i].Namespace, databases[i].Name, err))
		}
	}
	return errors.Join(errs...)
}

// listDatabasesForPolicy lists PgDatabase objects in policyRef's namespace
// whose spec.backupPolicy matches policyRef. Same-namespace-only by
// design: cross-namespace backupPolicy references are unsupported, so the
// worker never needs cluster-wide PgDatabase list permission.
func listDatabasesForPolicy(ctx context.Context, c client.Client, policyRef types.NamespacedName) ([]apiV1.PgDatabase, error) {
	var all apiV1.PgDatabaseList
	if err := c.List(ctx, &all, client.InNamespace(policyRef.Namespace)); err != nil {
		return nil, err
	}
	var matched []apiV1.PgDatabase
	for _, db := range all.Items {
		if db.Spec.BackupPolicy == nil {
			continue
		}
		if db.Spec.BackupPolicy.Namespace == policyRef.Namespace && db.Spec.BackupPolicy.Name == policyRef.Name {
			matched = append(matched, db)
		}
	}
	return matched, nil
}

// dumpOneDatabase creates a Pending PgBackupInstance first (so a mid-run
// crash still leaves an observable Failure record once cleanup's stale-
// pending detection catches it), then dump -> encrypt (if configured) ->
// upload -> patch to Success/Failure.
func dumpOneDatabase(ctx context.Context, c client.Client, workspaceDir string, policy *apiV1.PgBackupPolicy, database *apiV1.PgDatabase) error {
	startedAt := metaV1.Now()
	dumpOptions := resolveDumpOptions(policy.Spec.DumpOptions)

	instance := &apiV1.PgBackupInstance{
		ObjectMeta: metaV1.ObjectMeta{
			Namespace:    policy.Namespace,
			GenerateName: policy.Name + "-" + database.Name + "-",
		},
		Spec: apiV1.PgBackupInstanceSpec{
			Database:     apiV1.PgInstanceRef{Namespace: database.Namespace, Name: database.Name},
			BackupPolicy: apiV1.PgInstanceRef{Namespace: policy.Namespace, Name: policy.Name},
		},
	}
	if err := controllerutil.SetControllerReference(policy, instance, c.Scheme()); err != nil {
		return fmt.Errorf("unable to set owner reference: %w", err)
	}
	if err := c.Create(ctx, instance); err != nil {
		return fmt.Errorf("unable to create PgBackupInstance: %w", err)
	}
	instance.Status = apiV1.PgBackupInstanceStatus{
		Phase:     apiV1.PgBackupInstancePhasePending,
		StartedAt: &startedAt,
	}
	if err := c.Status().Update(ctx, instance); err != nil {
		return fmt.Errorf("unable to set PgBackupInstance to Pending: %w", err)
	}

	if err := runDumpPipeline(ctx, c, workspaceDir, policy, database, instance, dumpOptions); err != nil {
		finishedAt := metaV1.Now()
		instance.Status.Phase = apiV1.PgBackupInstancePhaseFailure
		instance.Status.Message = err.Error()
		instance.Status.FinishedAt = &finishedAt
		if statusErr := c.Status().Update(ctx, instance); statusErr != nil {
			return errors.Join(err, fmt.Errorf("additionally failed to record Failure status: %w", statusErr))
		}
		return err
	}
	return nil
}

// runDumpPipeline does the actual dump -> encrypt -> upload -> Success work,
// leaving instance.Status untouched on failure so the caller can record it.
func runDumpPipeline(ctx context.Context, c client.Client, workspaceDir string, policy *apiV1.PgBackupPolicy, database *apiV1.PgDatabase, instance *apiV1.PgBackupInstance, dumpOptions apiV1.PgBackupDumpOptions) error {
	var pgInstance apiV1.PgInstance
	if err := c.Get(ctx, database.Spec.Instance.ToNamespacedName(), &pgInstance); err != nil {
		return fmt.Errorf("unable to fetch PgInstance: %w", err)
	}
	pgApi, err := services.NewPgInstanceAPI(ctx, c, &pgInstance)
	if err != nil {
		return fmt.Errorf("unable to connect to PgInstance: %w", err)
	}
	cs := pgApi.ConnectionString()

	artifactPath, err := dumper.Dump(ctx, dumper.Options{
		Host:             cs.Hostname(),
		Port:             cs.Port(),
		Username:         cs.Username(),
		Password:         cs.Password(),
		SSLMode:          cs.SSLMode(),
		Database:         database.Name,
		Format:           dumpOptions.Format,
		CompressionLevel: *dumpOptions.CompressionLevel,
		OutputPath:       filepath.Join(workspaceDir, instance.Name+".dump"),
	})
	if err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}

	uploadPath := artifactPath
	targetKey := database.Namespace + "/" + database.Name + "/" + instance.Name + ".dump"
	if policy.Spec.Encryption != nil {
		keyMaterial, err := resolveSecretValue(ctx, c, policy.Namespace, policy.Spec.Encryption.SecretKeyRef.Name, policy.Spec.Encryption.SecretKeyRef.Key)
		if err != nil {
			return fmt.Errorf("unable to resolve encryption key: %w", err)
		}
		encryptor, err := encryption.NewEncryptor(policy.Spec.Encryption.Type, keyMaterial)
		if err != nil {
			return fmt.Errorf("unable to build encryptor: %w", err)
		}
		encryptedPath := artifactPath + ".gpg"
		if err := encryptor.Encrypt(ctx, artifactPath, encryptedPath); err != nil {
			return fmt.Errorf("encryption failed: %w", err)
		}
		uploadPath = encryptedPath
		targetKey += ".gpg"
	}

	dest, err := buildDestination(ctx, c, policy)
	if err != nil {
		return fmt.Errorf("unable to build storage destination: %w", err)
	}

	if err := dest.Store(ctx, uploadPath, targetKey); err != nil {
		// Best-effort cleanup of a possibly-partial object: each attempt's
		// target key is derived from this instance's generated name, so
		// nothing will ever naturally retry/overwrite it later.
		_ = dest.Delete(ctx, targetKey)
		return fmt.Errorf("upload failed: %w", err)
	}

	sizeBytes := int64(0)
	if info, err := os.Stat(uploadPath); err == nil {
		sizeBytes = info.Size()
	}

	finishedAt := metaV1.Now()
	instance.Status.Phase = apiV1.PgBackupInstancePhaseSuccess
	instance.Status.Location = locationFor(policy, targetKey)
	instance.Status.DumpOptions = dumpOptions
	instance.Status.SizeBytes = &sizeBytes
	instance.Status.FinishedAt = &finishedAt
	instance.Status.Message = "-"
	if policy.Spec.Retention.MaxAge != nil {
		expiresAt := metaV1.NewTime(finishedAt.Add(policy.Spec.Retention.MaxAge.Duration))
		instance.Status.ExpiresAt = &expiresAt
	}
	return c.Status().Update(ctx, instance)
}

// buildDestination resolves the storage secret and constructs the
// storage.Destination for this policy's currently configured storage.
func buildDestination(ctx context.Context, c client.Client, policy *apiV1.PgBackupPolicy) (storage.Destination, error) {
	if policy.Spec.Storage.Type != apiV1.PgBackupStorageTypeS3 || policy.Spec.Storage.S3 == nil {
		return nil, fmt.Errorf("unsupported or missing storage configuration")
	}
	accessKey, err := resolveSecretValue(ctx, c, policy.Namespace, policy.Spec.Storage.S3.SecretRef.Name, apiV1.PgBackupStorageS3SecretKeyAccessKey)
	if err != nil {
		return nil, err
	}
	secretKey, err := resolveSecretValue(ctx, c, policy.Namespace, policy.Spec.Storage.S3.SecretRef.Name, apiV1.PgBackupStorageS3SecretKeySecretKey)
	if err != nil {
		return nil, err
	}
	return storage.NewDestination(ctx, policy.Spec.Storage, accessKey, secretKey)
}

// locationFor builds the status.location discriminated union recording
// where this specific backup was actually stored. Endpoint/Bucket/Secure
// are snapshotted (not just a human-readable URL) because cleanup must be
// able to rebuild the exact Destination this object was written to even if
// the policy's storage config changes later - only SecretRef is
// deliberately omitted, since credentials always come from the policy's
// *current* secret, not a stale snapshot. URL encodes the bare,
// prefix-free target key passed to Destination.Store so cleanup can
// recover it losslessly via targetKeyFromLocation.
func locationFor(policy *apiV1.PgBackupPolicy, targetKey string) *apiV1.PgBackupStorage {
	s3 := policy.Spec.Storage.S3
	return &apiV1.PgBackupStorage{
		Type: apiV1.PgBackupStorageTypeS3,
		S3: &apiV1.PgBackupStorageS3{
			Endpoint: s3.Endpoint,
			Bucket:   s3.Bucket,
			Secure:   s3.Secure,
			URL:      fmt.Sprintf("s3://%s/%s", s3.Bucket, targetKey),
		},
	}
}

// resolveDumpOptions applies defaults on top of the policy's configured
// dump options so callers always get a fully-resolved value to snapshot
// into PgBackupInstanceStatus.DumpOptions.
func resolveDumpOptions(configured apiV1.PgBackupDumpOptions) apiV1.PgBackupDumpOptions {
	resolved := configured
	if resolved.Format == "" {
		resolved.Format = apiV1.PgBackupDumpFormatCustom
	}
	if resolved.CompressionLevel == nil {
		level := defaultCompressionLevel
		resolved.CompressionLevel = &level
	}
	return resolved
}

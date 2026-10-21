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
	"strings"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/backup/dumper"
	"github.com/brose-ebike/postgres-operator/pkg/backup/encryption"
	"github.com/brose-ebike/postgres-operator/pkg/backup/storage"
	"github.com/brose-ebike/postgres-operator/pkg/pgapi"
	"github.com/brose-ebike/postgres-operator/pkg/security"
	"github.com/brose-ebike/postgres-operator/pkg/services"
)

const defaultCompressionLevel = int32(6)

// backupRolePredefinedRole is the built-in, read-only Postgres role granted
// to a per-dump temporary role, instead of running pg_dump as the
// PgInstance's admin connection directly.
const backupRolePredefinedRole = "pg_read_all_data"

// backupRoleName derives a deterministic, unique-per-attempt Postgres role
// name from the (already-unique) generated PgBackupInstance name.
func backupRoleName(instanceName string) string {
	return "pgbackup_" + strings.ReplaceAll(instanceName, "-", "_")
}

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

// listDatabasesForPolicy lists every PgDatabase cluster-wide whose
// spec.backupPolicy matches policyRef - a PgDatabase can reference a
// PgBackupPolicy in a different namespace, so this cannot be scoped to the
// policy's own namespace. The operator's own ClusterRole (the worker runs
// as the operator's own ServiceAccount, see
// internal/controller/pgbackuppolicy_controller.go) already grants the
// cluster-wide list this requires.
func listDatabasesForPolicy(ctx context.Context, c client.Client, policyRef types.NamespacedName) ([]apiV1.PgDatabase, error) {
	var all apiV1.PgDatabaseList
	if err := c.List(ctx, &all); err != nil {
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

	var artifactPath string
	roleName := backupRoleName(instance.Name)
	err = withTemporaryBackupRole(pgApi, roleName, database.Name, func(username, password string) error {
		var dumpErr error
		artifactPath, dumpErr = dumper.Dump(ctx, dumper.Options{
			Host:             cs.Hostname(),
			Port:             cs.Port(),
			Username:         username,
			Password:         password,
			SSLMode:          cs.SSLMode(),
			Database:         database.Name,
			Format:           dumpOptions.Format,
			CompressionLevel: *dumpOptions.CompressionLevel,
			OutputPath:       filepath.Join(workspaceDir, instance.Name+".dump"),
		})
		return dumpErr
	})
	if err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}

	uploadPath := artifactPath
	targetKey := database.Namespace + "/" + database.Name + "/" + instance.Name + ".dump"
	if policy.Spec.Encryption != nil {
		encryptedPath, err := encryptArtifact(ctx, c, policy, artifactPath)
		if err != nil {
			return err
		}
		uploadPath = encryptedPath
		targetKey += ".gpg"
	}

	dest, resolvedStorage, err := buildDestination(ctx, c, policy)
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
	instance.Status.Location = locationFor(resolvedStorage, targetKey)
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

// encryptArtifact encrypts artifactPath into "<artifactPath>.gpg" and
// removes the plaintext artifact once encryption succeeds - encryption
// exists specifically to avoid an unencrypted copy of the dump at rest, so
// leaving the plaintext sitting on the workspace volume after a successful
// encrypt would defeat that. Best-effort: a cleanup failure doesn't
// invalidate the already-valid encrypted artifact, so it doesn't fail the
// dump.
func encryptArtifact(ctx context.Context, c client.Client, policy *apiV1.PgBackupPolicy, artifactPath string) (string, error) {
	keyMaterial, err := resolveEncryptionKeyMaterial(ctx, c, policy)
	if err != nil {
		return "", fmt.Errorf("unable to resolve encryption key: %w", err)
	}
	encryptor, err := encryption.NewEncryptor(policy.Spec.Encryption.Type, keyMaterial)
	if err != nil {
		return "", fmt.Errorf("unable to build encryptor: %w", err)
	}
	encryptedPath := artifactPath + ".gpg"
	if err := encryptor.Encrypt(ctx, artifactPath, encryptedPath); err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}
	_ = os.Remove(artifactPath)
	return encryptedPath, nil
}

// resolveEncryptionKeyMaterial resolves the PgProperty matching the
// policy's configured encryption type - PublicKey for gpg-rsa, Passphrase
// for gpg-aes. The CEL validation on PgBackupEncryption (api/v1) already
// guarantees the matching field is set for a given type.
func resolveEncryptionKeyMaterial(ctx context.Context, c client.Reader, policy *apiV1.PgBackupPolicy) (string, error) {
	enc := policy.Spec.Encryption
	switch enc.Type {
	case apiV1.PgBackupEncryptionTypeGPGRSA:
		return enc.PublicKey.GetPropertyValue(ctx, c, policy.Namespace, "publicKey")
	case apiV1.PgBackupEncryptionTypeGPGAES:
		return enc.Passphrase.GetPropertyValue(ctx, c, policy.Namespace, "passphrase")
	default:
		return "", fmt.Errorf("unsupported encryption type %q", enc.Type)
	}
}

// withTemporaryBackupRole creates a dedicated, read-only login role scoped
// to pg_read_all_data (plus CONNECT on the one target database, since a
// hardened database may have revoked PUBLIC's default CONNECT grant via
// PgDatabase.spec.publicPrivileges.revoke) for the duration of fn, then
// best-effort revokes and drops it again - pg_dump never runs as the
// PgInstance's admin connection directly.
//
// pg_read_all_data is an instance-wide predefined role, not scoped to the
// one database being dumped: by default (PUBLIC retains CONNECT on every
// database unless a PgDatabase has explicitly revoked it), this temporary
// role can open a connection to any sibling database on the same
// PgInstance and read everything in it for the duration of this one dump.
// Accepted as a documented risk rather than redesigned to a narrower grant
// (see docs/usage/backup-policy.md) - databases sharing an instance with a
// backed-up database should revoke PUBLIC's default CONNECT if they don't
// want to be readable during that instance's backup window.
//
// Cleanup errors are joined into the returned error (so they're visible
// and don't silently leave a credential behind) but never suppress fn's
// own result.
func withTemporaryBackupRole(pgApi pgapi.PgInstanceAPI, roleName string, databaseName string, fn func(username, password string) error) error {
	password := security.GeneratePassword()
	if err := pgApi.CreateRole(roleName); err != nil {
		return fmt.Errorf("unable to create backup role: %w", err)
	}
	if err := pgApi.UpdateUserPassword(roleName, password); err != nil {
		_ = pgApi.DeleteRole(roleName)
		return fmt.Errorf("unable to set backup role password: %w", err)
	}
	if err := pgApi.UpdateDatabasePrivileges(databaseName, roleName, []string{"CONNECT"}); err != nil {
		_ = pgApi.DeleteRole(roleName)
		return fmt.Errorf("unable to grant CONNECT to backup role: %w", err)
	}
	if err := pgApi.GrantPredefinedRole(roleName, backupRolePredefinedRole); err != nil {
		_ = pgApi.DeleteRole(roleName)
		return fmt.Errorf("unable to grant %s to backup role: %w", backupRolePredefinedRole, err)
	}

	fnErr := fn(roleName, password)

	revokeErr := pgApi.RevokePredefinedRole(roleName, backupRolePredefinedRole)
	deleteErr := pgApi.DeleteRole(roleName)
	var cleanupErr error
	if revokeErr != nil || deleteErr != nil {
		cleanupErr = fmt.Errorf("unable to fully clean up temporary backup role %q (manual cleanup needed): %w", roleName, errors.Join(revokeErr, deleteErr))
	}
	return errors.Join(fnErr, cleanupErr)
}

// resolvedS3Storage holds a PgBackupStorageS3's PgProperty fields after
// resolution - the plain values storage.NewDestination and a
// PgBackupStorageS3Location snapshot both need.
type resolvedS3Storage struct {
	endpoint  string
	bucket    string
	prefix    string
	secure    bool
	accessKey string
	secretKey string
}

// resolveS3Storage resolves every PgProperty field on an S3 storage config.
// Prefix is optional (GetPropertyValueWithDefault, defaulting to ""); every
// other field is required.
func resolveS3Storage(ctx context.Context, c client.Reader, namespace string, s3 *apiV1.PgBackupStorageS3) (resolvedS3Storage, error) {
	var resolved resolvedS3Storage
	var err error
	if resolved.endpoint, err = s3.Endpoint.GetPropertyValue(ctx, c, namespace, "endpoint"); err != nil {
		return resolvedS3Storage{}, fmt.Errorf("unable to resolve storage endpoint: %w", err)
	}
	if resolved.bucket, err = s3.Bucket.GetPropertyValue(ctx, c, namespace, "bucket"); err != nil {
		return resolvedS3Storage{}, fmt.Errorf("unable to resolve storage bucket: %w", err)
	}
	if resolved.prefix, err = s3.Prefix.GetPropertyValueWithDefault(ctx, c, namespace, "prefix", ""); err != nil {
		return resolvedS3Storage{}, fmt.Errorf("unable to resolve storage prefix: %w", err)
	}
	if resolved.accessKey, err = s3.AccessKey.GetPropertyValue(ctx, c, namespace, "accessKey"); err != nil {
		return resolvedS3Storage{}, fmt.Errorf("unable to resolve storage access key: %w", err)
	}
	if resolved.secretKey, err = s3.SecretKey.GetPropertyValue(ctx, c, namespace, "secretKey"); err != nil {
		return resolvedS3Storage{}, fmt.Errorf("unable to resolve storage secret key: %w", err)
	}
	resolved.secure = s3.IsSecure()
	return resolved, nil
}

// buildDestination resolves the policy's currently configured S3 storage
// and constructs its storage.Destination. Also returns the resolved values
// so the caller can snapshot them into PgBackupInstanceStatus.Location
// without re-resolving (and without recording credentials there - see
// locationFor).
func buildDestination(ctx context.Context, c client.Client, policy *apiV1.PgBackupPolicy) (storage.Destination, resolvedS3Storage, error) {
	if policy.Spec.Storage.Type != apiV1.PgBackupStorageTypeS3 || policy.Spec.Storage.S3 == nil {
		return nil, resolvedS3Storage{}, fmt.Errorf("unsupported or missing storage configuration")
	}
	resolved, err := resolveS3Storage(ctx, c, policy.Namespace, policy.Spec.Storage.S3)
	if err != nil {
		return nil, resolvedS3Storage{}, err
	}
	dest, err := storage.NewDestination(ctx, storage.TypeS3, resolved.endpoint, resolved.bucket, resolved.prefix, resolved.secure, resolved.accessKey, resolved.secretKey)
	if err != nil {
		return nil, resolvedS3Storage{}, err
	}
	return dest, resolved, nil
}

// locationFor builds the status.location snapshot recording where this
// specific backup was actually stored. Endpoint/Bucket/Prefix/Secure are
// snapshotted (not just a human-readable URL) because cleanup must be able
// to rebuild the exact Destination this object was written to - including
// the same prefix, or it computes the wrong object key - even if the
// policy's storage config changes later. AccessKey/SecretKey are
// deliberately never snapshotted: credentials always come from the
// policy's *current* secret, not a stale one. URL encodes the bare,
// prefix-free target key passed to Destination.Store so cleanup can
// recover it losslessly via targetKeyFromLocation.
func locationFor(s3 resolvedS3Storage, targetKey string) *apiV1.PgBackupStorageLocation {
	return &apiV1.PgBackupStorageLocation{
		Type: apiV1.PgBackupStorageTypeS3,
		S3: &apiV1.PgBackupStorageS3Location{
			Endpoint: s3.endpoint,
			Bucket:   s3.bucket,
			Prefix:   s3.prefix,
			Secure:   s3.secure,
			URL:      fmt.Sprintf("s3://%s/%s", s3.bucket, targetKey),
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

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
	"errors"
	"fmt"
	"strings"
	"time"

	kErrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/backup/retention"
	"github.com/brose-ebike/postgres-operator/pkg/backup/storage"
)

// stalePendingTimeout marks a Pending instance as Failure if the worker
// that should have completed it never did (crash, OOM-kill, etc).
const stalePendingTimeout = 30 * time.Minute

// failureFallbackRetention is how long a Failure record is kept for
// debugging visibility when the policy has no explicit MaxAge.
const failureFallbackRetention = 7 * 24 * time.Hour

// RunCleanup enforces retention for every database backed up by this
// policy: Success-phase instances are grouped per-database (cloud-pgdumper
// applies retention globally across every dump regardless of database,
// which would be wrong for a policy backing up several databases - see
// pkg/backup/retention's package doc) and retention.ComputeOutdated is run
// once per group against the policy's current retention settings. Stale
// Pending instances and expired Failure records are also cleaned up here,
// independently of the per-database retention grouping (neither counts as
// a valid backup).
func RunCleanup(ctx context.Context, c client.Client, policyRef types.NamespacedName) error {
	var policy apiV1.PgBackupPolicy
	if err := c.Get(ctx, policyRef, &policy); err != nil {
		return fmt.Errorf("unable to fetch PgBackupPolicy %s: %w", policyRef, err)
	}

	var all apiV1.PgBackupInstanceList
	if err := c.List(ctx, &all, client.InNamespace(policyRef.Namespace)); err != nil {
		return fmt.Errorf("unable to list PgBackupInstances for policy %s: %w", policyRef, err)
	}

	now := time.Now()
	var errs []error
	successByDatabase := map[string][]apiV1.PgBackupInstance{}

	for i := range all.Items {
		instance := &all.Items[i]
		if instance.Spec.BackupPolicy.Name != policyRef.Name {
			continue
		}

		switch instance.Status.Phase {
		case apiV1.PgBackupInstancePhaseSuccess:
			if instance.Status.FinishedAt != nil {
				successByDatabase[instance.Spec.Database.Name] = append(successByDatabase[instance.Spec.Database.Name], *instance)
			}
		case apiV1.PgBackupInstancePhasePending:
			if instance.Status.StartedAt != nil && now.Sub(instance.Status.StartedAt.Time) > stalePendingTimeout {
				if err := markStalePendingAsFailure(ctx, c, instance, now); err != nil {
					errs = append(errs, err)
				}
			}
		case apiV1.PgBackupInstancePhaseFailure:
			maxAge := failureFallbackRetention
			if policy.Spec.Retention.MaxAge != nil {
				maxAge = policy.Spec.Retention.MaxAge.Duration
			}
			if instance.Status.FinishedAt != nil && now.Sub(instance.Status.FinishedAt.Time) > maxAge {
				if err := deleteInstanceRecord(ctx, c, instance); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	retentionPolicy := retention.Policy{
		MinCount: policy.Spec.Retention.MinCount,
	}
	if policy.Spec.Retention.MaxAge != nil {
		maxAge := policy.Spec.Retention.MaxAge.Duration
		retentionPolicy.MaxAge = &maxAge
	}

	for _, group := range successByDatabase {
		entries := make([]retention.Entry, len(group))
		for i, instance := range group {
			entries[i] = retention.Entry{Name: instance.Name, FinishedAt: instance.Status.FinishedAt.Time}
		}
		outdated := retention.ComputeOutdated(entries, retentionPolicy, now)
		for _, e := range outdated {
			instance := findInstanceByName(group, e.Name)
			if instance == nil {
				continue
			}
			if err := deleteExpiredBackup(ctx, c, &policy, instance); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func findInstanceByName(instances []apiV1.PgBackupInstance, name string) *apiV1.PgBackupInstance {
	for i := range instances {
		if instances[i].Name == name {
			return &instances[i]
		}
	}
	return nil
}

func markStalePendingAsFailure(ctx context.Context, c client.Client, instance *apiV1.PgBackupInstance, now time.Time) error {
	finishedAt := metaV1.NewTime(now)
	instance.Status.Phase = apiV1.PgBackupInstancePhaseFailure
	instance.Status.Message = "stale pending: dump worker did not complete within the timeout (likely crashed or was killed)"
	instance.Status.FinishedAt = &finishedAt
	if err := c.Status().Update(ctx, instance); err != nil {
		return fmt.Errorf("unable to mark stale pending instance %s as Failure: %w", instance.Name, err)
	}
	return nil
}

// deleteExpiredBackup deletes a Success instance's storage object - rebuilt
// from that instance's own status.location, not the policy's current
// storage config, since storage config may have changed since that dump -
// and then the PgBackupInstance CR itself.
func deleteExpiredBackup(ctx context.Context, c client.Client, policy *apiV1.PgBackupPolicy, instance *apiV1.PgBackupInstance) error {
	if instance.Status.Location != nil {
		dest, targetKey, err := destinationFromLocation(ctx, c, policy, instance.Status.Location)
		if err != nil {
			return fmt.Errorf("unable to build destination for instance %s: %w", instance.Name, err)
		}
		if err := dest.Delete(ctx, targetKey); err != nil {
			return fmt.Errorf("unable to delete storage object for instance %s: %w", instance.Name, err)
		}
	}
	return deleteInstanceRecord(ctx, c, instance)
}

func deleteInstanceRecord(ctx context.Context, c client.Client, instance *apiV1.PgBackupInstance) error {
	if err := c.Delete(ctx, instance); err != nil && !kErrors.IsNotFound(err) {
		return fmt.Errorf("unable to delete PgBackupInstance %s: %w", instance.Name, err)
	}
	return nil
}

// destinationFromLocation rebuilds a storage.Destination from a recorded
// PgBackupInstanceStatus.Location, resolving credentials from the policy's
// *current* storage secret (location never carries a SecretRef).
func destinationFromLocation(ctx context.Context, c client.Client, policy *apiV1.PgBackupPolicy, location *apiV1.PgBackupStorage) (storage.Destination, string, error) {
	if location.Type != apiV1.PgBackupStorageTypeS3 || location.S3 == nil {
		return nil, "", fmt.Errorf("unsupported or missing location")
	}
	if policy.Spec.Storage.S3 == nil {
		return nil, "", fmt.Errorf("policy no longer has s3 storage configured")
	}
	targetKey := targetKeyFromLocation(location.S3)

	cfg := apiV1.PgBackupStorage{
		Type: apiV1.PgBackupStorageTypeS3,
		S3: &apiV1.PgBackupStorageS3{
			Endpoint: location.S3.Endpoint,
			Bucket:   location.S3.Bucket,
			Secure:   location.S3.Secure,
		},
	}
	dest, err := buildDestination(ctx, c, policyWithStorage(policy, cfg))
	if err != nil {
		return nil, "", err
	}
	return dest, targetKey, nil
}

// policyWithStorage returns a shallow copy of policy with Spec.Storage
// overridden, so buildDestination (which resolves credentials from
// policy.Namespace + policy.Spec.Storage.S3.SecretRef) can be reused
// unchanged for both the live dump path and the recorded-location cleanup
// path.
func policyWithStorage(policy *apiV1.PgBackupPolicy, overrideStorage apiV1.PgBackupStorage) *apiV1.PgBackupPolicy {
	copied := *policy
	copied.Spec.Storage = overrideStorage
	copied.Spec.Storage.S3.SecretRef = policy.Spec.Storage.S3.SecretRef
	return &copied
}

// targetKeyFromLocation recovers the bare, prefix-free target key that was
// originally passed to Destination.Store, by stripping the
// "s3://<bucket>/" prefix locationFor encoded it with.
func targetKeyFromLocation(s3 *apiV1.PgBackupStorageS3) string {
	prefix := fmt.Sprintf("s3://%s/", s3.Bucket)
	return strings.TrimPrefix(s3.URL, prefix)
}

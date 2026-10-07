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
	"time"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

func successInstance(namespace string, name string, policyName string, databaseName string, finishedAgo time.Duration) *apiV1.PgBackupInstance {
	finishedAt := metaV1.NewTime(time.Now().Add(-finishedAgo))
	startedAt := metaV1.NewTime(finishedAt.Add(-time.Minute))
	return &apiV1.PgBackupInstance{
		ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: apiV1.PgBackupInstanceSpec{
			Database:     apiV1.PgInstanceRef{Namespace: namespace, Name: databaseName},
			BackupPolicy: apiV1.PgInstanceRef{Namespace: namespace, Name: policyName},
		},
		Status: apiV1.PgBackupInstanceStatus{
			Phase:      apiV1.PgBackupInstancePhaseSuccess,
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,
		},
	}
}

func TestRunCleanup_PerDatabaseGroupingNoDeletionNeeded(t *testing.T) {
	scheme := newTestScheme(t)
	minCount := int32(3)
	policy := testPolicy("default", "nightly")
	policy.Spec.Retention = apiV1.PgBackupRetention{MinCount: &minCount}

	// db-a: 2 successes (below minCount=3, nothing should be deleted).
	// db-b: 3 successes, old ages, but MaxAge is unset so pure count-based
	// retention applies and 3 <= minCount=3 -> nothing deleted either.
	// If grouping were broken (evaluated together, 5 total > minCount) the
	// pure count-based branch would incorrectly delete the 2 oldest.
	objs := []*apiV1.PgBackupInstance{
		successInstance("default", "db-a-1", "nightly", "db-a", time.Hour),
		successInstance("default", "db-a-2", "nightly", "db-a", 2*time.Hour),
		successInstance("default", "db-b-1", "nightly", "db-b", 100*24*time.Hour),
		successInstance("default", "db-b-2", "nightly", "db-b", 101*24*time.Hour),
		successInstance("default", "db-b-3", "nightly", "db-b", 102*24*time.Hour),
	}

	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&apiV1.PgBackupPolicy{}, &apiV1.PgBackupInstance{}).WithObjects(policy)
	for _, o := range objs {
		builder = builder.WithObjects(o)
	}
	c := builder.Build()

	if err := RunCleanup(context.Background(), c, types.NamespacedName{Namespace: "default", Name: "nightly"}); err != nil {
		t.Fatalf("RunCleanup failed: %v", err)
	}

	var remaining apiV1.PgBackupInstanceList
	if err := c.List(context.Background(), &remaining); err != nil {
		t.Fatalf("unable to list remaining instances: %v", err)
	}
	if len(remaining.Items) != len(objs) {
		t.Fatalf("expected all %d instances to survive (per-database grouping keeps each group within its own minCount), got %d", len(objs), len(remaining.Items))
	}
}

func TestRunCleanup_StalePendingMarkedFailure(t *testing.T) {
	scheme := newTestScheme(t)
	policy := testPolicy("default", "nightly")
	staleStartedAt := metaV1.NewTime(time.Now().Add(-time.Hour))
	pending := &apiV1.PgBackupInstance{
		ObjectMeta: metaV1.ObjectMeta{Namespace: "default", Name: "stale-pending"},
		Spec: apiV1.PgBackupInstanceSpec{
			Database:     apiV1.PgInstanceRef{Namespace: "default", Name: "db-a"},
			BackupPolicy: apiV1.PgInstanceRef{Namespace: "default", Name: "nightly"},
		},
		Status: apiV1.PgBackupInstanceStatus{
			Phase:     apiV1.PgBackupInstancePhasePending,
			StartedAt: &staleStartedAt,
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiV1.PgBackupPolicy{}, &apiV1.PgBackupInstance{}).
		WithObjects(policy, pending).
		Build()

	if err := RunCleanup(context.Background(), c, types.NamespacedName{Namespace: "default", Name: "nightly"}); err != nil {
		t.Fatalf("RunCleanup failed: %v", err)
	}

	var updated apiV1.PgBackupInstance
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "stale-pending"}, &updated); err != nil {
		t.Fatalf("unable to fetch instance: %v", err)
	}
	if updated.Status.Phase != apiV1.PgBackupInstancePhaseFailure {
		t.Fatalf("expected stale pending instance to become Failure, got %s", updated.Status.Phase)
	}
	if updated.Status.Message == "" {
		t.Fatalf("expected a message explaining the stale pending timeout")
	}
}

func TestRunCleanup_ExpiredFailureDeletedButRecentKept(t *testing.T) {
	scheme := newTestScheme(t)
	policy := testPolicy("default", "nightly")

	oldFinishedAt := metaV1.NewTime(time.Now().Add(-8 * 24 * time.Hour))
	recentFinishedAt := metaV1.NewTime(time.Now().Add(-time.Hour))

	oldFailure := &apiV1.PgBackupInstance{
		ObjectMeta: metaV1.ObjectMeta{Namespace: "default", Name: "old-failure"},
		Spec: apiV1.PgBackupInstanceSpec{
			Database:     apiV1.PgInstanceRef{Namespace: "default", Name: "db-a"},
			BackupPolicy: apiV1.PgInstanceRef{Namespace: "default", Name: "nightly"},
		},
		Status: apiV1.PgBackupInstanceStatus{Phase: apiV1.PgBackupInstancePhaseFailure, FinishedAt: &oldFinishedAt},
	}
	recentFailure := &apiV1.PgBackupInstance{
		ObjectMeta: metaV1.ObjectMeta{Namespace: "default", Name: "recent-failure"},
		Spec: apiV1.PgBackupInstanceSpec{
			Database:     apiV1.PgInstanceRef{Namespace: "default", Name: "db-a"},
			BackupPolicy: apiV1.PgInstanceRef{Namespace: "default", Name: "nightly"},
		},
		Status: apiV1.PgBackupInstanceStatus{Phase: apiV1.PgBackupInstancePhaseFailure, FinishedAt: &recentFinishedAt},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiV1.PgBackupPolicy{}, &apiV1.PgBackupInstance{}).
		WithObjects(policy, oldFailure, recentFailure).
		Build()

	// No explicit MaxAge on the policy -> falls back to the 7-day window,
	// so the 8-day-old Failure is expired and the 1-hour-old one isn't.
	if err := RunCleanup(context.Background(), c, types.NamespacedName{Namespace: "default", Name: "nightly"}); err != nil {
		t.Fatalf("RunCleanup failed: %v", err)
	}

	err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "old-failure"}, &apiV1.PgBackupInstance{})
	if !kErrors.IsNotFound(err) {
		t.Fatalf("expected the old Failure record to be deleted, got err=%v", err)
	}

	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "recent-failure"}, &apiV1.PgBackupInstance{}); err != nil {
		t.Fatalf("expected the recent Failure record to survive, got err=%v", err)
	}
}

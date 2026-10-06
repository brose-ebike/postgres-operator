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
	"time"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("PgBackupInstanceReconciler", func() {

	var reconciler *PgBackupInstanceReconciler

	BeforeEach(func() {
		reconciler = &PgBackupInstanceReconciler{Client: k8sClient}
	})

	createInstance := func(ctx context.Context, name string, policy string, database string, phase apiV1.PgBackupInstancePhase, startedAt v1.Time) *apiV1.PgBackupInstance {
		finishedAt := v1.NewTime(startedAt.Add(time.Minute))
		instance := &apiV1.PgBackupInstance{
			ObjectMeta: v1.ObjectMeta{Namespace: "default", Name: name},
			Spec: apiV1.PgBackupInstanceSpec{
				Database:     apiV1.PgInstanceRef{Namespace: "default", Name: database},
				BackupPolicy: apiV1.PgInstanceRef{Namespace: "default", Name: policy},
			},
			Status: apiV1.PgBackupInstanceStatus{
				Phase:      phase,
				StartedAt:  &startedAt,
				FinishedAt: &finishedAt,
			},
		}
		Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		// Status is a separate subresource - Create() above ignores it, so
		// it must be persisted via a follow-up status update, same as the
		// real worker would do.
		Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())
		return instance
	}

	reconcileInstance := func(ctx context.Context, name string) {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: name}})
		Expect(err).To(BeNil())
	}

	phaseValue := func(policy string, database string, phase apiV1.PgBackupInstancePhase) float64 {
		return testutil.ToFloat64(backupLastPhaseGauge.WithLabelValues("default", policy, database, string(phase)))
	}

	AfterEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var all apiV1.PgBackupInstanceList
		Expect(k8sClient.List(ctx, &all)).To(Succeed())
		for i := range all.Items {
			inst := &all.Items[i]
			inst.Finalizers = []string{}
			Expect(k8sClient.Update(ctx, inst)).To(Succeed())
		}
		Expect(k8sClient.DeleteAllOf(ctx, &apiV1.PgBackupInstance{}, client.InNamespace("default"))).To(Succeed())
	})

	It("only the newest instance per database drives the gauges, and out-of-order reconciles don't clobber it", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		older := createInstance(ctx, "gauge-older", "gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseSuccess, v1.NewTime(time.Now().Add(-time.Hour)))
		newer := createInstance(ctx, "gauge-newer", "gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseFailure, v1.NewTime(time.Now()))

		reconcileInstance(ctx, older.Name)
		Expect(phaseValue("gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseSuccess)).To(Equal(1.0))
		Expect(phaseValue("gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseFailure)).To(Equal(0.0))

		reconcileInstance(ctx, newer.Name)
		Expect(phaseValue("gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseFailure)).To(Equal(1.0))
		Expect(phaseValue("gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseSuccess)).To(Equal(0.0))

		// Reconciling the older object again (e.g. a resync) must not
		// revert the gauges back to the older instance's phase.
		reconcileInstance(ctx, older.Name)
		Expect(phaseValue("gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseFailure)).To(Equal(1.0))
		Expect(phaseValue("gauge-policy-1", "gauge-db-1", apiV1.PgBackupInstancePhaseSuccess)).To(Equal(0.0))
	})

	It("recomputes from the remaining sibling when the latest instance is deleted, and clears the series when none remain", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		older := createInstance(ctx, "gauge-older-2", "gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseSuccess, v1.NewTime(time.Now().Add(-time.Hour)))
		newer := createInstance(ctx, "gauge-newer-2", "gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseFailure, v1.NewTime(time.Now()))

		reconcileInstance(ctx, older.Name)
		reconcileInstance(ctx, newer.Name)
		Expect(phaseValue("gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseFailure)).To(Equal(1.0))

		// Delete the newer (currently latest) instance - the gauges should
		// fall back to reflecting the remaining older instance.
		Expect(k8sClient.Delete(ctx, newer)).To(Succeed())
		reconcileInstance(ctx, newer.Name)
		Expect(phaseValue("gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseSuccess)).To(Equal(1.0))
		Expect(phaseValue("gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseFailure)).To(Equal(0.0))

		// Delete the last remaining instance - no stale "1" should survive.
		Expect(k8sClient.Delete(ctx, older)).To(Succeed())
		reconcileInstance(ctx, older.Name)
		Expect(phaseValue("gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseSuccess)).To(Equal(0.0))
		Expect(phaseValue("gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhaseFailure)).To(Equal(0.0))
		Expect(phaseValue("gauge-policy-2", "gauge-db-2", apiV1.PgBackupInstancePhasePending)).To(Equal(0.0))
	})

	It("sets the last-success-timestamp gauge on Success and leaves it untouched on a later Failure", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		successTime := time.Now().Add(-time.Hour)
		success := createInstance(ctx, "gauge-success-3", "gauge-policy-3", "gauge-db-3", apiV1.PgBackupInstancePhaseSuccess, v1.NewTime(successTime))
		reconcileInstance(ctx, success.Name)

		got := testutil.ToFloat64(backupLastSuccessTimestampGauge.WithLabelValues("default", "gauge-policy-3", "gauge-db-3"))
		Expect(got).To(Equal(float64(success.Status.FinishedAt.Unix())))

		failure := createInstance(ctx, "gauge-failure-3", "gauge-policy-3", "gauge-db-3", apiV1.PgBackupInstancePhaseFailure, v1.NewTime(time.Now()))
		reconcileInstance(ctx, failure.Name)

		// The latest attempt failed, but the last known success timestamp
		// must still read the earlier success's FinishedAt, not be cleared.
		stillGot := testutil.ToFloat64(backupLastSuccessTimestampGauge.WithLabelValues("default", "gauge-policy-3", "gauge-db-3"))
		Expect(stillGot).To(Equal(got))
	})
})

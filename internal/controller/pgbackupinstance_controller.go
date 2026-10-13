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
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

var backupLastPhaseGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "postgres_operator_backup_last_phase",
		Help: "1 for the phase of the most recent backup attempt for this namespace/policy/database, 0 for the other two phase values of the same triple.",
	},
	[]string{"namespace", "policy", "database", "phase"},
)

var backupLastSuccessTimestampGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "postgres_operator_backup_last_success_timestamp_seconds",
		Help: "Unix timestamp (status.finishedAt) of the most recent Success backup for this namespace/policy/database.",
	},
	[]string{"namespace", "policy", "database"},
)

func init() {
	metrics.Registry.MustRegister(backupLastPhaseGauge, backupLastSuccessTimestampGauge)
}

var allPhases = []apiV1.PgBackupInstancePhase{
	apiV1.PgBackupInstancePhasePending,
	apiV1.PgBackupInstancePhaseSuccess,
	apiV1.PgBackupInstancePhaseFailure,
}

// PgBackupInstanceReconciler reconciles a PgBackupInstance object
//
// This controller is purely metrics-reflecting: all dump/retention logic
// lives in pkg/backup/worker, which creates/patches PgBackupInstance
// objects directly. The gauges here are keyed on "the latest backup
// attempt per database", not on every instance individually, so a single
// stale instance can never shadow a later one.
type PgBackupInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackupinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackupinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackupinstances/finalizers,verbs=update

func (r *PgBackupInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := log.FromContext(ctx)

	var instance apiV1.PgBackupInstance
	exists, err := getResource(ctx, r, req.NamespacedName, &instance)
	if err != nil {
		logger.Error(err, "Unable to fetch PgBackupInstance", "instance", req.NamespacedName.String())
		return ctrl.Result{}, err
	}
	if !exists {
		return ctrl.Result{}, nil
	}

	namespace := instance.Namespace
	policyName := instance.Spec.BackupPolicy.Name
	databaseName := instance.Spec.Database.Name

	if instance.DeletionTimestamp != nil {
		siblings, err := r.listSiblings(ctx, namespace, policyName, databaseName, instance.Name)
		if err != nil {
			logger.Error(err, "Unable to list sibling PgBackupInstances", "instance", instance.ToNamespacedName())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		if latest := latestOf(siblings); latest != nil {
			setGauges(namespace, policyName, databaseName, *latest)
		} else {
			clearGauges(namespace, policyName, databaseName)
		}

		if controllerutil.ContainsFinalizer(&instance, apiV1.DefaultFinalizerPgBackupInstance) {
			controllerutil.RemoveFinalizer(&instance, apiV1.DefaultFinalizerPgBackupInstance)
			if err := r.Update(ctx, &instance); err != nil {
				logger.Error(err, "Failed to update finalizers", "instance", instance.ToNamespacedName())
				return ctrl.Result{RequeueAfter: time.Second}, err
			}
		}
		return ctrl.Result{}, nil
	}

	siblings, err := r.listSiblings(ctx, namespace, policyName, databaseName, "")
	if err != nil {
		logger.Error(err, "Unable to list sibling PgBackupInstances", "instance", instance.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	// Only the newest instance for this database drives the gauges - an
	// out-of-order reconcile of an older object (e.g. on controller
	// restart/resync) must never clobber the current reading.
	if latest := latestOf(siblings); latest != nil && latest.Name == instance.Name {
		setGauges(namespace, policyName, databaseName, instance)
	}

	if !controllerutil.ContainsFinalizer(&instance, apiV1.DefaultFinalizerPgBackupInstance) {
		controllerutil.AddFinalizer(&instance, apiV1.DefaultFinalizerPgBackupInstance)
		if err := r.Update(ctx, &instance); err != nil {
			logger.Error(err, "Failed to update finalizers", "instance", instance.ToNamespacedName())
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PgBackupInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiV1.PgBackupInstance{}).
		Complete(r)
}

// listSiblings lists every PgBackupInstance for the same
// (namespace, policy, database) triple, excluding the object named
// excludeName (used when deleting, so the object being torn down doesn't
// count against itself).
func (r *PgBackupInstanceReconciler) listSiblings(ctx context.Context, namespace string, policyName string, databaseName string, excludeName string) ([]apiV1.PgBackupInstance, error) {
	var all apiV1.PgBackupInstanceList
	if err := r.List(ctx, &all, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	var matched []apiV1.PgBackupInstance
	for _, inst := range all.Items {
		if inst.Name == excludeName {
			continue
		}
		if inst.Spec.BackupPolicy.Name == policyName && inst.Spec.Database.Name == databaseName {
			matched = append(matched, inst)
		}
	}
	return matched, nil
}

// latestOf returns the instance with the newest Status.StartedAt (falling
// back to CreationTimestamp if unset), or nil if instances is empty.
func latestOf(instances []apiV1.PgBackupInstance) *apiV1.PgBackupInstance {
	if len(instances) == 0 {
		return nil
	}
	sorted := make([]apiV1.PgBackupInstance, len(instances))
	copy(sorted, instances)
	sort.Slice(sorted, func(i, j int) bool {
		return startedAt(&sorted[i]).After(startedAt(&sorted[j]))
	})
	return &sorted[0]
}

func startedAt(instance *apiV1.PgBackupInstance) time.Time {
	if instance.Status.StartedAt != nil {
		return instance.Status.StartedAt.Time
	}
	return instance.CreationTimestamp.Time
}

// setGauges sets the current phase's label-combination to 1 and the other
// two phases' to 0, so switching phase never leaves a stale "1" behind
// under the old phase label. Also updates the last-success-timestamp gauge
// when the current phase is Success.
func setGauges(namespace string, policyName string, databaseName string, instance apiV1.PgBackupInstance) {
	for _, phase := range allPhases {
		value := 0.0
		if instance.Status.Phase == phase {
			value = 1.0
		}
		backupLastPhaseGauge.WithLabelValues(namespace, policyName, databaseName, string(phase)).Set(value)
	}
	if instance.Status.Phase == apiV1.PgBackupInstancePhaseSuccess && instance.Status.FinishedAt != nil {
		backupLastSuccessTimestampGauge.WithLabelValues(namespace, policyName, databaseName).Set(float64(instance.Status.FinishedAt.Unix()))
	}
}

// clearGauges removes all series for this triple, used when the last
// instance for a database is deleted.
func clearGauges(namespace string, policyName string, databaseName string) {
	for _, phase := range allPhases {
		backupLastPhaseGauge.DeleteLabelValues(namespace, policyName, databaseName, string(phase))
	}
	backupLastSuccessTimestampGauge.DeleteLabelValues(namespace, policyName, databaseName)
}

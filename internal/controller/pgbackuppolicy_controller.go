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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	kErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

// defaultWorkspaceSizeLimit is used when PgBackupPolicySpec.WorkspaceSizeLimit
// is unset, matching the kubebuilder default on the field itself.
var defaultWorkspaceSizeLimit = resource.MustParse("10Gi")

// cleanupCronSchedule is a fixed daily schedule for the retention/cleanup
// CronJob; only the dump CronJob's schedule is user-configurable.
const cleanupCronSchedule = "0 3 * * *"

// maxCronJobNameLength is Kubernetes' own limit on CronJob names (it
// reserves the rest of the 63-character object-name budget for the
// timestamp suffix it appends when creating a Job from the CronJob).
const maxCronJobNameLength = 52

// PgBackupPolicyReconciler reconciles a PgBackupPolicy object
type PgBackupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Image is the container image reference used for both the dump and
	// cleanup CronJobs - the same image the manager itself is running as,
	// resolved in cmd/main.go (see SetupWithManager callers).
	Image string
	// OperatorNamespace is the namespace the operator itself runs in. The
	// dump/cleanup CronJobs run under the operator's own ServiceAccount
	// (see reconcileCronJob's doc comment for why), and a ServiceAccount
	// can only be mounted by a Pod in its own namespace - so every policy's
	// CronJobs are created here, not in the policy's own namespace.
	OperatorNamespace string
	// OperatorServiceAccountName is the ServiceAccount the manager itself
	// runs as; the backup CronJobs run as this same ServiceAccount rather
	// than a dynamically-created per-policy one (see reconcileCronJob).
	OperatorServiceAccountName string
}

//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackuppolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackuppolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackuppolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackupinstances,verbs=get;list;create;patch;update;delete
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackupinstances/status,verbs=get;patch;update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile validates the policy's storage/encryption secrets resolve
// (including the keys the worker will actually read, not just that the
// Secret object exists), sets the ready condition, and creates/updates the
// dump and cleanup CronJobs. This controller never talks to Postgres
// directly - all dump/retention logic lives in the backup-worker binary
// these CronJobs invoke.
func (r *PgBackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := log.FromContext(ctx)

	var policy apiV1.PgBackupPolicy
	exists, err := getResource(ctx, r, req.NamespacedName, &policy)
	if err != nil {
		logger.Error(err, "Unable to fetch PgBackupPolicy", "policy", req.NamespacedName.String())
		return ctrl.Result{}, err
	}
	if !exists {
		return ctrl.Result{}, nil
	}

	// Handle finalizing. The CronJobs live in the operator's own namespace
	// (see OperatorNamespace's doc comment), not the policy's namespace, so
	// Kubernetes ignores any owner reference between them (a namespaced
	// owner reference across namespaces is treated as absent) - they are
	// never garbage-collected automatically and must be deleted explicitly
	// here, the same way the per-policy ClusterRole/ClusterRoleBinding this
	// design replaces previously had to be.
	if policy.DeletionTimestamp != nil {
		if err := r.deleteCronJob(ctx, r.dumpCronJobName(&policy)); err != nil {
			logger.Error(err, "Unable to delete dump CronJob", "policy", policy.ToNamespacedName())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		if err := r.deleteCronJob(ctx, r.cleanupCronJobName(&policy)); err != nil {
			logger.Error(err, "Unable to delete cleanup CronJob", "policy", policy.ToNamespacedName())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}

		if controllerutil.ContainsFinalizer(&policy, apiV1.DefaultFinalizerPgBackupPolicy) {
			controllerutil.RemoveFinalizer(&policy, apiV1.DefaultFinalizerPgBackupPolicy)
			if err := r.Update(ctx, &policy); err != nil {
				logger.Error(err, "Failed to update finalizers", "policy", policy.ToNamespacedName())
				return ctrl.Result{RequeueAfter: time.Second}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if r.Image == "" || r.OperatorNamespace == "" || r.OperatorServiceAccountName == "" {
		if err := setCondition(ctx, r.Status(), &policy, apiV1.PgBackupPolicyReadyConditionType, false, "NotConfigured", "backup worker image/operator namespace/operator service account not configured"); err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		logger.Error(fmt.Errorf("backup worker image/operator namespace/operator service account not configured"), "Cannot reconcile PgBackupPolicy", "policy", policy.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if err := r.validateSecrets(ctx, &policy); err != nil {
		if condErr := setCondition(ctx, r.Status(), &policy, apiV1.PgBackupPolicyReadyConditionType, false, apiV1.PgBackupPolicyReadyConditionReasonFailed, err.Error()); condErr != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, condErr
		}
		logger.Error(err, "PgBackupPolicy secrets did not resolve", "policy", policy.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	if err := setCondition(ctx, r.Status(), &policy, apiV1.PgBackupPolicyReadyConditionType, true, apiV1.PgBackupPolicyReadyConditionReasonSucceeded, "-"); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	if err := r.reconcileCronJob(ctx, &policy, r.dumpCronJobName(&policy), policy.Spec.Schedule, "dump"); err != nil {
		logger.Error(err, "Unable to reconcile dump CronJob", "policy", policy.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	if err := r.reconcileCronJob(ctx, &policy, r.cleanupCronJobName(&policy), cleanupCronSchedule, "cleanup"); err != nil {
		logger.Error(err, "Unable to reconcile cleanup CronJob", "policy", policy.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	if !controllerutil.ContainsFinalizer(&policy, apiV1.DefaultFinalizerPgBackupPolicy) {
		controllerutil.AddFinalizer(&policy, apiV1.DefaultFinalizerPgBackupPolicy)
		if err := r.Update(ctx, &policy); err != nil {
			logger.Error(err, "Failed to update finalizers", "policy", policy.ToNamespacedName())
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PgBackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiV1.PgBackupPolicy{}).
		Complete(r)
}

// validateSecrets resolves spec.storage's secretRef and, if spec.encryption
// is set, its secretKeyRef - checking not just that the Secret exists but
// that it actually carries the key(s) the worker will read later, so a
// typo'd key surfaces on the Ready condition instead of at the next
// scheduled dump.
func (r *PgBackupPolicyReconciler) validateSecrets(ctx context.Context, policy *apiV1.PgBackupPolicy) error {
	switch policy.Spec.Storage.Type {
	case apiV1.PgBackupStorageTypeS3:
		if policy.Spec.Storage.S3 == nil {
			return fmt.Errorf("storage type is %q but s3 configuration is not set", policy.Spec.Storage.Type)
		}
		if err := r.checkSecretHasKeys(ctx, policy.Namespace, policy.Spec.Storage.S3.SecretRef.Name,
			apiV1.PgBackupStorageS3SecretKeyAccessKey, apiV1.PgBackupStorageS3SecretKeySecretKey); err != nil {
			return fmt.Errorf("storage secret: %w", err)
		}
	default:
		return fmt.Errorf("unsupported storage type %q", policy.Spec.Storage.Type)
	}

	if policy.Spec.Encryption != nil {
		if err := r.checkSecretHasKeys(ctx, policy.Namespace, policy.Spec.Encryption.SecretKeyRef.Name, policy.Spec.Encryption.SecretKeyRef.Key); err != nil {
			return fmt.Errorf("encryption secret: %w", err)
		}
	}
	return nil
}

// checkSecretHasKeys verifies the named Secret exists AND carries every
// key in requiredKeys. Checking existence alone (the previous behavior)
// let a misnamed key pass the Ready condition and only fail once the dump
// CronJob actually ran and tried to read it.
func (r *PgBackupPolicyReconciler) checkSecretHasKeys(ctx context.Context, namespace string, name string, requiredKeys ...string) error {
	if name == "" {
		return fmt.Errorf("secret name is empty")
	}
	var secret coreV1.Secret
	exists, err := getResource(ctx, r, types.NamespacedName{Namespace: namespace, Name: name}, &secret)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("secret %q not found", name)
	}

	var missing []string
	for _, key := range requiredKeys {
		if _, ok := secret.Data[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("secret %q is missing required key(s): %s", name, strings.Join(missing, ", "))
	}
	return nil
}

// backupWorkerCronJobName builds a CronJob name that stays unique across
// every PgBackupPolicy in the cluster, now that all backup CronJobs live
// in the operator's own namespace rather than one per policy namespace.
// Plain "<namespace>-<name>-<suffix>" string concatenation isn't
// collision-safe: (namespace="foo-bar", name="baz") and (namespace="foo",
// name="bar-baz") both produce "foo-bar-baz" - exactly the bug flagged in
// review for the now-removed per-policy ClusterRole name. CronJob names
// are also capped at maxCronJobNameLength, so a long namespace+name pair
// can't simply be concatenated either way. A short deterministic hash of
// the fully-qualified policy guarantees uniqueness; a truncated
// human-readable prefix keeps `kubectl get cronjobs` legible.
func backupWorkerCronJobName(policy *apiV1.PgBackupPolicy, suffix string) string {
	sum := sha256.Sum256([]byte(policy.Namespace + "/" + policy.Name))
	hash := hex.EncodeToString(sum[:])[:10]

	prefixBudget := maxCronJobNameLength - len(hash) - len(suffix) - 2 // 2 separating hyphens
	prefix := policy.Name
	if len(prefix) > prefixBudget {
		prefix = prefix[:prefixBudget]
	}
	return fmt.Sprintf("%s-%s-%s", prefix, hash, suffix)
}

func (r *PgBackupPolicyReconciler) dumpCronJobName(policy *apiV1.PgBackupPolicy) string {
	return backupWorkerCronJobName(policy, "dump")
}

func (r *PgBackupPolicyReconciler) cleanupCronJobName(policy *apiV1.PgBackupPolicy) string {
	return backupWorkerCronJobName(policy, "cleanup")
}

func (r *PgBackupPolicyReconciler) deleteCronJob(ctx context.Context, name string) error {
	cronJob := &batchv1.CronJob{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: r.OperatorNamespace}}
	if err := r.Delete(ctx, cronJob); err != nil && !kErrors.IsNotFound(err) {
		return err
	}
	return nil
}

// reconcileCronJob creates/updates one CronJob in the operator's own
// namespace, running as the operator's own ServiceAccount - not a
// dynamically-created per-policy ServiceAccount/Role/RoleBinding/
// ClusterRole/ClusterRoleBinding. The operator's existing ClusterRole
// already carries everything the worker needs (secrets/configmaps reads
// for PgProperty resolution, PgDatabase/PgInstance reads, and now
// PgBackupInstance CRUD per this file's RBAC markers) - consolidated in
// one place instead of N dynamically-created copies, and without any
// cluster-scoped object whose name could collide across policies.
func (r *PgBackupPolicyReconciler) reconcileCronJob(ctx context.Context, policy *apiV1.PgBackupPolicy, name string, schedule string, subcommand string) error {
	cronJob := &batchv1.CronJob{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: r.OperatorNamespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cronJob, func() error {
		cronJob.Spec = r.buildCronJobSpec(policy, schedule, subcommand)
		return nil
	})
	return err
}

// buildCronJobSpec carries over the security posture of the production
// cron-pg-backup.yaml CronJob this replaces: Forbid concurrency, a
// non-root read-only-root-filesystem security context, modest resource
// limits, and emptyDir scratch volumes for the dump workspace (pg_dump's
// output plus its encrypted copy can briefly coexist) and $HOME (both
// pg_dump and gopenpgp need writable scratch space under a read-only root
// filesystem).
func (r *PgBackupPolicyReconciler) buildCronJobSpec(policy *apiV1.PgBackupPolicy, schedule string, subcommand string) batchv1.CronJobSpec {
	workspaceSizeLimit := defaultWorkspaceSizeLimit
	if policy.Spec.WorkspaceSizeLimit != nil {
		workspaceSizeLimit = *policy.Spec.WorkspaceSizeLimit
	}

	args := []string{subcommand, "--policy=" + policy.Namespace + "/" + policy.Name}

	return batchv1.CronJobSpec{
		Schedule:                   schedule,
		ConcurrencyPolicy:          batchv1.ForbidConcurrent,
		SuccessfulJobsHistoryLimit: ptr.To(int32(1)),
		FailedJobsHistoryLimit:     ptr.To(int32(3)),
		JobTemplate: batchv1.JobTemplateSpec{
			Spec: batchv1.JobSpec{
				Template: coreV1.PodTemplateSpec{
					Spec: coreV1.PodSpec{
						RestartPolicy:      coreV1.RestartPolicyOnFailure,
						ServiceAccountName: r.OperatorServiceAccountName,
						Containers: []coreV1.Container{{
							Name:    "backup-worker",
							Image:   r.Image,
							Command: append([]string{"/backup-worker"}, args...),
							Env:     []coreV1.EnvVar{{Name: "HOME", Value: "/home/pgdumper"}},
							SecurityContext: &coreV1.SecurityContext{
								ReadOnlyRootFilesystem:   ptr.To(true),
								RunAsNonRoot:             ptr.To(true),
								RunAsUser:                ptr.To(int64(1000)),
								AllowPrivilegeEscalation: ptr.To(false),
							},
							Resources: coreV1.ResourceRequirements{
								Limits: coreV1.ResourceList{
									coreV1.ResourceCPU:    resource.MustParse("100m"),
									coreV1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: coreV1.ResourceList{
									coreV1.ResourceCPU:    resource.MustParse("10m"),
									coreV1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							VolumeMounts: []coreV1.VolumeMount{
								{Name: "workspace", MountPath: "/app/workspace"},
								{Name: "home", MountPath: "/home/pgdumper"},
							},
						}},
						Volumes: []coreV1.Volume{
							{Name: "workspace", VolumeSource: coreV1.VolumeSource{EmptyDir: &coreV1.EmptyDirVolumeSource{SizeLimit: &workspaceSizeLimit}}},
							{Name: "home", VolumeSource: coreV1.VolumeSource{EmptyDir: &coreV1.EmptyDirVolumeSource{}}},
						},
					},
				},
			},
		},
	}
}

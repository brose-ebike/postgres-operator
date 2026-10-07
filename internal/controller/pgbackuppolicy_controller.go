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
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

// PgBackupPolicyReconciler reconciles a PgBackupPolicy object
type PgBackupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Image is the container image reference used for both the dump and
	// cleanup CronJobs - the same image the manager itself is running as,
	// resolved in cmd/manager/main.go (see SetupWithManager callers).
	Image string
}

//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackuppolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackuppolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgres.oebc.tools,resources=pgbackuppolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get

// Reconcile validates the policy's storage/encryption secrets resolve, sets
// the ready condition, and creates/updates the worker RBAC
// (ServiceAccount/Role/RoleBinding) plus the dump and cleanup CronJobs, all
// owned via controllerutil.SetControllerReference so they garbage-collect
// automatically when the policy is deleted. This controller never talks to
// Postgres directly - all dump/retention logic lives in the backup-worker
// binary these CronJobs invoke.
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

	// Handle finalizing. Owned CronJobs/ServiceAccount/Role/RoleBinding
	// garbage-collect automatically via owner references - there is no
	// Postgres-side or storage-side state to tear down here (deleting a
	// policy intentionally does not delete already-stored backup objects,
	// see docs/usage/backup-policy.md). The ClusterRole/ClusterRoleBinding
	// are the one exception: Kubernetes does not honor an owner reference
	// from a cluster-scoped object to a namespaced one, so they are never
	// garbage-collected automatically and must be deleted explicitly here.
	if policy.DeletionTimestamp != nil {
		clusterRoleName := workerClusterRoleName(&policy)
		if err := r.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metaV1.ObjectMeta{Name: clusterRoleName}}); err != nil && !kErrors.IsNotFound(err) {
			logger.Error(err, "Unable to delete worker ClusterRoleBinding", "policy", policy.ToNamespacedName())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		if err := r.Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metaV1.ObjectMeta{Name: clusterRoleName}}); err != nil && !kErrors.IsNotFound(err) {
			logger.Error(err, "Unable to delete worker ClusterRole", "policy", policy.ToNamespacedName())
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

	if r.Image == "" {
		if err := setCondition(ctx, r.Status(), &policy, apiV1.PgBackupPolicyReadyConditionType, false, "ImageNotConfigured", "backup worker image not configured"); err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		logger.Error(fmt.Errorf("backup worker image not configured"), "Cannot reconcile PgBackupPolicy", "policy", policy.ToNamespacedName())
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

	serviceAccountName, err := r.reconcileWorkerRBAC(ctx, &policy)
	if err != nil {
		logger.Error(err, "Unable to reconcile worker RBAC", "policy", policy.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	if err := r.reconcileCronJob(ctx, &policy, r.dumpCronJobName(&policy), policy.Spec.Schedule, "dump", serviceAccountName); err != nil {
		logger.Error(err, "Unable to reconcile dump CronJob", "policy", policy.ToNamespacedName())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	if err := r.reconcileCronJob(ctx, &policy, r.cleanupCronJobName(&policy), cleanupCronSchedule, "cleanup", serviceAccountName); err != nil {
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
		Owns(&batchv1.CronJob{}).
		Owns(&coreV1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}

// validateSecrets resolves spec.storage's secretRef and, if spec.encryption
// is set, its secretKeyRef. The encryption check is skipped entirely when
// spec.encryption is nil.
func (r *PgBackupPolicyReconciler) validateSecrets(ctx context.Context, policy *apiV1.PgBackupPolicy) error {
	switch policy.Spec.Storage.Type {
	case apiV1.PgBackupStorageTypeS3:
		if policy.Spec.Storage.S3 == nil {
			return fmt.Errorf("storage type is %q but s3 configuration is not set", policy.Spec.Storage.Type)
		}
		if err := r.checkSecretExists(ctx, policy.Namespace, policy.Spec.Storage.S3.SecretRef.Name); err != nil {
			return fmt.Errorf("storage secret: %w", err)
		}
	default:
		return fmt.Errorf("unsupported storage type %q", policy.Spec.Storage.Type)
	}

	if policy.Spec.Encryption != nil {
		if err := r.checkSecretExists(ctx, policy.Namespace, policy.Spec.Encryption.SecretKeyRef.Name); err != nil {
			return fmt.Errorf("encryption secret: %w", err)
		}
	}
	return nil
}

func (r *PgBackupPolicyReconciler) checkSecretExists(ctx context.Context, namespace string, name string) error {
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
	return nil
}

// workerServiceAccountName is shared by the RBAC objects and the CronJobs
// that run as this ServiceAccount.
func workerServiceAccountName(policy *apiV1.PgBackupPolicy) string {
	return policy.Name + "-backup-worker"
}

// workerClusterRoleName must be globally unique (ClusterRoles/
// ClusterRoleBindings share one flat, namespace-less name space), unlike
// workerServiceAccountName which only needs to be unique within the
// policy's own namespace.
func workerClusterRoleName(policy *apiV1.PgBackupPolicy) string {
	return policy.Namespace + "-" + policy.Name + "-backup-worker"
}

func (r *PgBackupPolicyReconciler) dumpCronJobName(policy *apiV1.PgBackupPolicy) string {
	return policy.Name + "-dump"
}

func (r *PgBackupPolicyReconciler) cleanupCronJobName(policy *apiV1.PgBackupPolicy) string {
	return policy.Name + "-cleanup"
}

// reconcileWorkerRBAC creates/updates the per-policy ServiceAccount plus two
// kinds of permission:
//   - a namespaced Role/RoleBinding (owned, garbage-collects with the
//     policy) for write access to PgBackupInstance in the policy's own
//     namespace.
//   - a cluster-scoped ClusterRole/ClusterRoleBinding (NOT owned - see the
//     comment in Reconcile's deletion branch) for read-only, cluster-wide
//     access to PgDatabase/PgInstance/secrets/configmaps, since a
//     cross-namespace PgDatabase.spec.backupPolicy reference means a
//     matching database - and the PgInstance/credentials it in turn
//     references - can live in any namespace, not just the policy's own.
//
// Returns the ServiceAccount name the CronJobs should run as.
func (r *PgBackupPolicyReconciler) reconcileWorkerRBAC(ctx context.Context, policy *apiV1.PgBackupPolicy) (string, error) {
	name := workerServiceAccountName(policy)

	sa := &coreV1.ServiceAccount{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: policy.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		return controllerutil.SetControllerReference(policy, sa, r.Scheme)
	}); err != nil {
		return "", fmt.Errorf("service account: %w", err)
	}

	role := &rbacv1.Role{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: policy.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Rules = workerRoleRules()
		return controllerutil.SetControllerReference(policy, role, r.Scheme)
	}); err != nil {
		return "", fmt.Errorf("role: %w", err)
	}

	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: policy.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}
		roleBinding.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: policy.Namespace}}
		return controllerutil.SetControllerReference(policy, roleBinding, r.Scheme)
	}); err != nil {
		return "", fmt.Errorf("role binding: %w", err)
	}

	clusterRoleName := workerClusterRoleName(policy)
	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metaV1.ObjectMeta{Name: clusterRoleName}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRole, func() error {
		clusterRole.Rules = workerClusterRoleRules()
		return nil
	}); err != nil {
		return "", fmt.Errorf("cluster role: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metaV1.ObjectMeta{Name: clusterRoleName}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRoleName}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: policy.Namespace}}
		return nil
	}); err != nil {
		return "", fmt.Errorf("cluster role binding: %w", err)
	}

	return name, nil
}

// workerRoleRules grants namespace-scoped write access to this policy's own
// backup records, plus read access to its own PgBackupPolicy object.
func workerRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{APIGroups: []string{apiV1.GroupVersion.Group}, Resources: []string{"pgbackuppolicies"}, Verbs: []string{"get"}},
		{APIGroups: []string{apiV1.GroupVersion.Group}, Resources: []string{"pgbackupinstances"}, Verbs: []string{"get", "list", "create", "patch", "update", "delete"}},
		{APIGroups: []string{apiV1.GroupVersion.Group}, Resources: []string{"pgbackupinstances/status"}, Verbs: []string{"get", "patch", "update"}},
	}
}

// workerClusterRoleRules grants read-only, cluster-wide access needed
// because a referenced PgDatabase (and the PgInstance/credentials it in
// turn references) can live in any namespace, not just the policy's own -
// get/list is as narrow as this can be scoped: resourceNames can't help
// here since the set of matching objects is only known at runtime. secrets/
// configmaps get is correspondingly broad for the same reason PgProperty
// resolution always has been (it must resolve arbitrary credential refs
// discovered at runtime).
func workerClusterRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}},
		{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}},
		{APIGroups: []string{apiV1.GroupVersion.Group}, Resources: []string{"pgdatabases", "pginstances"}, Verbs: []string{"get", "list"}},
	}
}

func (r *PgBackupPolicyReconciler) reconcileCronJob(ctx context.Context, policy *apiV1.PgBackupPolicy, name string, schedule string, subcommand string, serviceAccountName string) error {
	cronJob := &batchv1.CronJob{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: policy.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cronJob, func() error {
		cronJob.Spec = r.buildCronJobSpec(policy, schedule, subcommand, serviceAccountName)
		return controllerutil.SetControllerReference(policy, cronJob, r.Scheme)
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
func (r *PgBackupPolicyReconciler) buildCronJobSpec(policy *apiV1.PgBackupPolicy, schedule string, subcommand string, serviceAccountName string) batchv1.CronJobSpec {
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
						ServiceAccountName: serviceAccountName,
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

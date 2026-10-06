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

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("PgBackupPolicyReconciler", func() {

	var reconciler *PgBackupPolicyReconciler

	BeforeEach(func() {
		reconciler = &PgBackupPolicyReconciler{
			Client: k8sClient,
			Scheme: scheme.Scheme,
			Image:  "example.com/postgres-operator:test",
		}
	})

	AfterEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var policies apiV1.PgBackupPolicyList
		Expect(k8sClient.List(ctx, &policies)).To(Succeed())
		for i := range policies.Items {
			p := &policies.Items[i]
			p.Finalizers = []string{}
			Expect(k8sClient.Update(ctx, p)).To(Succeed())
		}
		Expect(k8sClient.DeleteAllOf(ctx, &apiV1.PgBackupPolicy{}, client.InNamespace("default"))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &coreV1.Secret{}, client.InNamespace("default"))).To(Succeed())
	})

	createStorageSecret := func(ctx context.Context, name string) {
		secret := &coreV1.Secret{
			ObjectMeta: v1.ObjectMeta{Namespace: "default", Name: name},
			StringData: map[string]string{
				apiV1.PgBackupStorageS3SecretKeyAccessKey: "access",
				apiV1.PgBackupStorageS3SecretKeySecretKey: "secret",
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	}

	newPolicy := func(name string, storageSecretName string, schedule string) *apiV1.PgBackupPolicy {
		return &apiV1.PgBackupPolicy{
			ObjectMeta: v1.ObjectMeta{Namespace: "default", Name: name},
			Spec: apiV1.PgBackupPolicySpec{
				Schedule: schedule,
				Storage: apiV1.PgBackupStorage{
					Type: apiV1.PgBackupStorageTypeS3,
					S3: &apiV1.PgBackupStorageS3{
						Endpoint:  "minio.example.com",
						Bucket:    "pg-backups",
						SecretRef: coreV1.LocalObjectReference{Name: storageSecretName},
					},
				},
				Retention: apiV1.PgBackupRetention{},
			},
		}
	}

	reconcilePolicy := func(ctx context.Context, name string) (reconcile.Result, error) {
		return reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: name}})
	}

	It("creates the dump/cleanup CronJobs and worker RBAC, owned by the policy, when secrets resolve", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		createStorageSecret(ctx, "storage-creds-1")
		policy := newPolicy("policy-1", "storage-creds-1", "0 2 * * *")
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		result, err := reconcilePolicy(ctx, policy.Name)
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())

		var reconciled apiV1.PgBackupPolicy
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name}, &reconciled)).To(Succeed())
		readyCondition := meta.FindStatusCondition(reconciled.Status.Conditions, apiV1.PgBackupPolicyReadyConditionType)
		Expect(readyCondition).ToNot(BeNil())
		Expect(readyCondition.Status).To(Equal(v1.ConditionTrue))
		Expect(reconciled.Finalizers).To(ContainElement(apiV1.DefaultFinalizerPgBackupPolicy))

		var dumpCronJob batchv1.CronJob
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name + "-dump"}, &dumpCronJob)).To(Succeed())
		Expect(dumpCronJob.Spec.Schedule).To(Equal("0 2 * * *"))
		Expect(v1.IsControlledBy(&dumpCronJob, &reconciled)).To(BeTrue())
		Expect(dumpCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(Equal("example.com/postgres-operator:test"))
		Expect(dumpCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"/backup-worker", "dump", "--policy=default/policy-1"}))
		Expect(dumpCronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName).To(Equal("policy-1-backup-worker"))

		var cleanupCronJob batchv1.CronJob
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name + "-cleanup"}, &cleanupCronJob)).To(Succeed())
		Expect(cleanupCronJob.Spec.Schedule).To(Equal(cleanupCronSchedule))
		Expect(v1.IsControlledBy(&cleanupCronJob, &reconciled)).To(BeTrue())

		var sa coreV1.ServiceAccount
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "policy-1-backup-worker"}, &sa)).To(Succeed())
		Expect(v1.IsControlledBy(&sa, &reconciled)).To(BeTrue())

		var role rbacv1.Role
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "policy-1-backup-worker"}, &role)).To(Succeed())
		Expect(v1.IsControlledBy(&role, &reconciled)).To(BeTrue())
		Expect(role.Rules).ToNot(BeEmpty())

		var roleBinding rbacv1.RoleBinding
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "policy-1-backup-worker"}, &roleBinding)).To(Succeed())
		Expect(v1.IsControlledBy(&roleBinding, &reconciled)).To(BeTrue())
		Expect(roleBinding.RoleRef.Name).To(Equal("policy-1-backup-worker"))
		Expect(roleBinding.Subjects).To(HaveLen(1))
		Expect(roleBinding.Subjects[0].Name).To(Equal("policy-1-backup-worker"))
	})

	It("sets ready=false and does not create any CronJob when the storage secret is missing", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		policy := newPolicy("policy-2", "does-not-exist", "0 2 * * *")
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		_, err := reconcilePolicy(ctx, policy.Name)
		Expect(err).To(BeNil())

		var reconciled apiV1.PgBackupPolicy
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name}, &reconciled)).To(Succeed())
		readyCondition := meta.FindStatusCondition(reconciled.Status.Conditions, apiV1.PgBackupPolicyReadyConditionType)
		Expect(readyCondition).ToNot(BeNil())
		Expect(readyCondition.Status).To(Equal(v1.ConditionFalse))

		var dumpCronJob batchv1.CronJob
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name + "-dump"}, &dumpCronJob)
		Expect(err).ToNot(BeNil())
	})

	It("sets ready=false when encryption is configured but its secret is missing, even though storage resolves", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		createStorageSecret(ctx, "storage-creds-3")
		policy := newPolicy("policy-3", "storage-creds-3", "0 2 * * *")
		policy.Spec.Encryption = &apiV1.PgBackupEncryption{
			Type:         apiV1.PgBackupEncryptionTypeGPGAES,
			SecretKeyRef: coreV1.SecretKeySelector{LocalObjectReference: coreV1.LocalObjectReference{Name: "missing-encryption-secret"}, Key: "passphrase"},
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		_, err := reconcilePolicy(ctx, policy.Name)
		Expect(err).To(BeNil())

		var reconciled apiV1.PgBackupPolicy
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name}, &reconciled)).To(Succeed())
		readyCondition := meta.FindStatusCondition(reconciled.Status.Conditions, apiV1.PgBackupPolicyReadyConditionType)
		Expect(readyCondition).ToNot(BeNil())
		Expect(readyCondition.Status).To(Equal(v1.ConditionFalse))
	})

	It("updates the dump CronJob's schedule in place rather than recreating it", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		createStorageSecret(ctx, "storage-creds-4")
		policy := newPolicy("policy-4", "storage-creds-4", "0 2 * * *")
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		_, err := reconcilePolicy(ctx, policy.Name)
		Expect(err).To(BeNil())

		var firstCronJob batchv1.CronJob
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name + "-dump"}, &firstCronJob)).To(Succeed())
		originalUID := firstCronJob.UID

		var toUpdate apiV1.PgBackupPolicy
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name}, &toUpdate)).To(Succeed())
		toUpdate.Spec.Schedule = "0 4 * * *"
		Expect(k8sClient.Update(ctx, &toUpdate)).To(Succeed())

		_, err = reconcilePolicy(ctx, policy.Name)
		Expect(err).To(BeNil())

		var updatedCronJob batchv1.CronJob
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: policy.Name + "-dump"}, &updatedCronJob)).To(Succeed())
		Expect(updatedCronJob.UID).To(Equal(originalUID))
		Expect(updatedCronJob.Spec.Schedule).To(Equal("0 4 * * *"))
	})
})

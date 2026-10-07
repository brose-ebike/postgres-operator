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

package e2e

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/backup/worker"
	"github.com/brose-ebike/postgres-operator/pkg/tcminio"
	"github.com/brose-ebike/postgres-operator/pkg/tcpostgres"
)

// pgDumpCustomFormatMagic is the 5-byte signature every pg_dump
// --format=custom archive starts with - asserting on it confirms a
// downloaded (and, where applicable, decrypted) object is a real archive,
// not just "some bytes landed in the bucket".
var pgDumpCustomFormatMagic = []byte("PGDMP")

var _ = Describe("backup-worker dump and cleanup, end to end", func() {

	var namespace string
	var pg *tcpostgres.PostgresContainer
	var minioContainer *tcminio.MinioContainer
	var minioClient *minio.Client
	var bucket string

	BeforeEach(func() {
		ctx := context.Background()
		namespace = "default"
		bucket = "pg-backups"

		var err error
		pg, err = tcpostgres.SetupPostgres(ctx, tcpostgres.WithInitialDatabase("pgtest", "pgtest", "mydb"))
		Expect(err).NotTo(HaveOccurred())

		minioContainer, err = tcminio.SetupMinio(ctx, tcminio.WithCredentials("minioadmin", "minioadmin"))
		Expect(err).NotTo(HaveOccurred())

		endpoint, err := minioContainer.Endpoint(ctx)
		Expect(err).NotTo(HaveOccurred())
		minioClient, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(minioContainer.AccessKey(), minioContainer.SecretKey(), ""),
			Secure: false,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctx := context.Background()
		Expect(pg.Terminate(ctx)).To(Succeed())
		Expect(minioContainer.Terminate(ctx)).To(Succeed())

		Expect(k8sClient.DeleteAllOf(ctx, &apiV1.PgBackupInstance{}, client.InNamespace(namespace))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &apiV1.PgBackupPolicy{}, client.InNamespace(namespace))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &apiV1.PgDatabase{}, client.InNamespace(namespace))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &apiV1.PgInstance{}, client.InNamespace(namespace))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &coreV1.Secret{}, client.InNamespace(namespace))).To(Succeed())
	})

	createInstanceAndDatabase := func(ctx context.Context, policyName string) {
		host, err := pg.Hostname(ctx)
		Expect(err).NotTo(HaveOccurred())
		port, err := pg.Port(ctx)
		Expect(err).NotTo(HaveOccurred())

		instance := &apiV1.PgInstance{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: "instance"},
			Spec: apiV1.PgInstanceSpec{
				Hostname: apiV1.PgProperty{Value: host},
				Port:     apiV1.PgProperty{Value: strconv.Itoa(port)},
				Username: apiV1.PgProperty{Value: pg.Username()},
				Password: apiV1.PgProperty{Value: pg.Password()},
				Database: apiV1.PgProperty{Value: pg.Database()},
				SSLMode:  apiV1.PgProperty{Value: "disable"},
			},
		}
		Expect(k8sClient.Create(ctx, instance)).To(Succeed())

		database := &apiV1.PgDatabase{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: pg.Database()},
			Spec: apiV1.PgDatabaseSpec{
				Instance:     apiV1.PgInstanceRef{Namespace: namespace, Name: "instance"},
				BackupPolicy: &apiV1.PgInstanceRef{Namespace: namespace, Name: policyName},
			},
		}
		Expect(k8sClient.Create(ctx, database)).To(Succeed())
	}

	createStorageSecret := func(ctx context.Context, name string) {
		secret := &coreV1.Secret{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: name},
			StringData: map[string]string{
				apiV1.PgBackupStorageS3SecretKeyAccessKey: minioContainer.AccessKey(),
				apiV1.PgBackupStorageS3SecretKeySecretKey: minioContainer.SecretKey(),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	}

	createPolicy := func(ctx context.Context, name string, minCount int32, encryption *apiV1.PgBackupEncryption) *apiV1.PgBackupPolicy {
		endpoint, err := minioContainer.Endpoint(ctx)
		Expect(err).NotTo(HaveOccurred())

		policy := &apiV1.PgBackupPolicy{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: apiV1.PgBackupPolicySpec{
				Schedule: "0 2 * * *",
				Storage: apiV1.PgBackupStorage{
					Type: apiV1.PgBackupStorageTypeS3,
					S3: &apiV1.PgBackupStorageS3{
						Endpoint:  endpoint,
						Bucket:    bucket,
						Secure:    false,
						SecretRef: coreV1.LocalObjectReference{Name: name + "-storage"},
					},
				},
				Encryption: encryption,
				Retention:  apiV1.PgBackupRetention{MinCount: &minCount},
				DumpOptions: apiV1.PgBackupDumpOptions{
					Format: apiV1.PgBackupDumpFormatCustom,
				},
			},
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		return policy
	}

	It("dumps an unencrypted database, uploads it, records Success, then cleanup enforces retention for real", func() {
		ctx := context.Background()
		policyName := "e2e-plain"

		createStorageSecret(ctx, policyName+"-storage")
		createInstanceAndDatabase(ctx, policyName)
		createPolicy(ctx, policyName, 1, nil)

		policyRef := types.NamespacedName{Namespace: namespace, Name: policyName}
		Expect(worker.RunDump(ctx, k8sClient, policyRef, GinkgoT().TempDir())).To(Succeed())

		var instances apiV1.PgBackupInstanceList
		Expect(k8sClient.List(ctx, &instances)).To(Succeed())
		Expect(instances.Items).To(HaveLen(1))
		first := instances.Items[0]
		Expect(first.Status.Phase).To(Equal(apiV1.PgBackupInstancePhaseSuccess))
		Expect(first.Status.Location).NotTo(BeNil())
		Expect(first.Status.SizeBytes).NotTo(BeNil())
		Expect(*first.Status.SizeBytes).To(BeNumerically(">", 0))

		targetKey := namespace + "/" + pg.Database() + "/" + first.Name + ".dump"
		obj, err := minioClient.GetObject(ctx, bucket, targetKey, minio.GetObjectOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer obj.Close()
		contents, err := io.ReadAll(obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.HasPrefix(contents, pgDumpCustomFormatMagic)).To(BeTrue(), "uploaded object should be a real pg_dump custom-format archive")

		// Seed a second, older Success instance pointing at a real (but
		// separately uploaded) object, so cleanup's minCount=1 retention
		// has something genuine to delete.
		olderFinishedAt := metaV1.NewTime(time.Now().Add(-48 * time.Hour))
		olderInstance := &apiV1.PgBackupInstance{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: policyName + "-older"},
			Spec: apiV1.PgBackupInstanceSpec{
				Database:     apiV1.PgInstanceRef{Namespace: namespace, Name: pg.Database()},
				BackupPolicy: apiV1.PgInstanceRef{Namespace: namespace, Name: policyName},
			},
		}
		Expect(k8sClient.Create(ctx, olderInstance)).To(Succeed())
		olderTargetKey := namespace + "/" + pg.Database() + "/" + olderInstance.Name + ".dump"
		_, err = minioClient.PutObject(ctx, bucket, olderTargetKey, bytes.NewReader(contents), int64(len(contents)), minio.PutObjectOptions{})
		Expect(err).NotTo(HaveOccurred())
		olderInstance.Status = apiV1.PgBackupInstanceStatus{
			Phase:      apiV1.PgBackupInstancePhaseSuccess,
			FinishedAt: &olderFinishedAt,
			Location: &apiV1.PgBackupStorage{
				Type: apiV1.PgBackupStorageTypeS3,
				S3: &apiV1.PgBackupStorageS3{
					Bucket: bucket,
					URL:    "s3://" + bucket + "/" + olderTargetKey,
				},
			},
		}
		Expect(k8sClient.Status().Update(ctx, olderInstance)).To(Succeed())

		Expect(worker.RunCleanup(ctx, k8sClient, policyRef)).To(Succeed())

		var remaining apiV1.PgBackupInstanceList
		Expect(k8sClient.List(ctx, &remaining)).To(Succeed())
		Expect(remaining.Items).To(HaveLen(1))
		Expect(remaining.Items[0].Name).To(Equal(first.Name), "retention should keep the newer backup and delete the older one")

		_, err = minioClient.StatObject(ctx, bucket, olderTargetKey, minio.StatObjectOptions{})
		Expect(err).To(HaveOccurred(), "the older backup's storage object should have been deleted by cleanup")
	})

	It("dumps an encrypted database and the uploaded object decrypts back to a real pg_dump archive", func() {
		ctx := context.Background()
		policyName := "e2e-encrypted"
		passphrase := "correct-horse-battery-staple"

		createStorageSecret(ctx, policyName+"-storage")
		createInstanceAndDatabase(ctx, policyName)

		encryptionSecret := &coreV1.Secret{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: policyName + "-encryption"},
			StringData: map[string]string{"passphrase": passphrase},
		}
		Expect(k8sClient.Create(ctx, encryptionSecret)).To(Succeed())

		createPolicy(ctx, policyName, 1, &apiV1.PgBackupEncryption{
			Type: apiV1.PgBackupEncryptionTypeGPGAES,
			SecretKeyRef: coreV1.SecretKeySelector{
				LocalObjectReference: coreV1.LocalObjectReference{Name: policyName + "-encryption"},
				Key:                  "passphrase",
			},
		})

		policyRef := types.NamespacedName{Namespace: namespace, Name: policyName}
		Expect(worker.RunDump(ctx, k8sClient, policyRef, GinkgoT().TempDir())).To(Succeed())

		var instances apiV1.PgBackupInstanceList
		Expect(k8sClient.List(ctx, &instances)).To(Succeed())
		Expect(instances.Items).To(HaveLen(1))
		instance := instances.Items[0]
		Expect(instance.Status.Phase).To(Equal(apiV1.PgBackupInstancePhaseSuccess))

		targetKey := namespace + "/" + pg.Database() + "/" + instance.Name + ".dump.gpg"
		obj, err := minioClient.GetObject(ctx, bucket, targetKey, minio.GetObjectOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer obj.Close()
		ciphertext, err := io.ReadAll(obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.HasPrefix(ciphertext, pgDumpCustomFormatMagic)).To(BeFalse(), "uploaded object should be encrypted, not a plain pg_dump archive")

		plaintext, err := decryptWithPassword(ciphertext, passphrase)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.HasPrefix(plaintext, pgDumpCustomFormatMagic)).To(BeTrue(), "decrypting with the configured passphrase should recover a real pg_dump archive")
	})
})

// decryptWithPassword mirrors pkg/backup/encryption's gpg-aes decrypt path,
// used here only to verify the worker's encryption step actually produced
// decryptable ciphertext.
func decryptWithPassword(ciphertext []byte, passphrase string) ([]byte, error) {
	plain, err := crypto.DecryptMessageWithPassword(crypto.NewPGPMessage(ciphertext), []byte(passphrase))
	if err != nil {
		return nil, err
	}
	return plain.GetBinary(), nil
}

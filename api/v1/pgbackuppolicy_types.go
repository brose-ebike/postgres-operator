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

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	coreV1 "k8s.io/api/core/v1"
)

const DefaultFinalizerPgBackupPolicy = "postgres.oebc.tools/pgbackuppolicy"
const PgBackupPolicyReadyConditionType string = "pgbackuppolicy.postgres.oebc.tools/ready"

const (
	PgBackupPolicyReadyConditionReasonSucceeded = "SecretsResolved"
	PgBackupPolicyReadyConditionReasonFailed    = "SecretsResolveFailed"
)

// PgBackupStorageType enumerates the supported backup storage backends.
// Only "s3" is implemented today; "volume" and "ftp" are reserved for future
// extension without a schema break.
// +kubebuilder:validation:Enum=s3
type PgBackupStorageType string

const PgBackupStorageTypeS3 PgBackupStorageType = "s3"

const (
	PgBackupStorageS3SecretKeyAccessKey = "accessKey"
	PgBackupStorageS3SecretKeySecretKey = "secretKey"
)

// PgBackupStorageS3 configures an S3-compatible (MinIO) destination.
// Endpoint/Bucket/SecretRef are required when used in PgBackupPolicySpec.Storage
// (enforced by the reconciler, not by JSON required-ness, since this type is
// also reused for PgBackupInstanceStatus.Location where only URL is set).
type PgBackupStorageS3 struct {
	// Endpoint is the S3-compatible endpoint, host[:port], without scheme.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Bucket is the name of the bucket dumps are stored in. It is created
	// automatically if it does not exist.
	// +optional
	Bucket string `json:"bucket,omitempty"`
	// Prefix is an optional key prefix under which objects are stored.
	// +optional
	Prefix string `json:"prefix,omitempty"`
	// Secure selects whether TLS is used to talk to the endpoint.
	// A *bool, not bool: with a plain bool, encoding/json's "omitempty"
	// drops an explicit "false" from the request body indistinguishably
	// from "unset", so the apiserver's CRD default (true) would silently
	// override it on every create/update.
	// +kubebuilder:default=true
	// +optional
	Secure *bool `json:"secure,omitempty"`
	// SecretRef references a Secret containing the access/secret key pair,
	// under the keys PgBackupStorageS3SecretKeyAccessKey/SecretKey.
	// +optional
	SecretRef coreV1.LocalObjectReference `json:"secretRef,omitempty"`
	// URL is populated only in PgBackupInstanceStatus.Location, giving the
	// fully resolved object address. Unused in PgBackupPolicySpec.Storage.
	// +optional
	URL string `json:"url,omitempty"`
}

// IsSecure reports whether TLS should be used, defaulting to true (matching
// the CRD's kubebuilder default) when unset - relevant for Go-constructed
// values that never round-trip through the apiserver's CRD defaulting.
func (s *PgBackupStorageS3) IsSecure() bool {
	return s.Secure == nil || *s.Secure
}

// PgBackupStorage is a discriminated union selecting the storage backend.
// Exactly the sub-struct matching Type is expected to be populated. This
// same type is reused for PgBackupPolicySpec.Storage and
// PgBackupInstanceStatus.Location.
type PgBackupStorage struct {
	// Type selects which of the sub-structs below is populated.
	Type PgBackupStorageType `json:"type"`
	// S3 holds the configuration when Type is "s3".
	// +optional
	S3 *PgBackupStorageS3 `json:"s3,omitempty"`
}

// +kubebuilder:validation:Enum=gpg-rsa;gpg-aes
type PgBackupEncryptionType string

const (
	PgBackupEncryptionTypeGPGRSA PgBackupEncryptionType = "gpg-rsa"
	PgBackupEncryptionTypeGPGAES PgBackupEncryptionType = "gpg-aes"
)

// PgBackupEncryption configures at-rest encryption of the dump before
// upload. gpg-rsa expects SecretKeyRef to point at an armored public key;
// gpg-aes expects SecretKeyRef to point at a symmetric passphrase.
type PgBackupEncryption struct {
	// Type selects the encryption scheme.
	Type PgBackupEncryptionType `json:"type"`
	// SecretKeyRef selects the key material (public key for gpg-rsa,
	// passphrase for gpg-aes).
	SecretKeyRef coreV1.SecretKeySelector `json:"secretKeyRef"`
}

// PgBackupRetention configures how many / how long backups are kept. See
// pkg/backup/retention for the exact salvage semantics. Retention is
// applied per-database, not shared across a policy's databases.
type PgBackupRetention struct {
	// MinCount is a hard floor: at least this many of the newest backups of
	// each database are always kept, regardless of MaxAge.
	// +optional
	MinCount *int32 `json:"minCount,omitempty"`
	// MaxAge is the maximum age a backup may reach before it becomes
	// eligible for deletion (subject to MinCount salvage).
	// +optional
	MaxAge *metav1.Duration `json:"maxAge,omitempty"`
}

// +kubebuilder:validation:Enum=plain;custom;tar;directory
type PgBackupDumpFormat string

const (
	PgBackupDumpFormatPlain     PgBackupDumpFormat = "plain"
	PgBackupDumpFormatCustom    PgBackupDumpFormat = "custom"
	PgBackupDumpFormatTar       PgBackupDumpFormat = "tar"
	PgBackupDumpFormatDirectory PgBackupDumpFormat = "directory"
)

// PgBackupDumpOptions configures how pg_dump is invoked. It is used both as
// PgBackupPolicySpec.DumpOptions (defaults applied to every dump this
// policy triggers) and snapshotted verbatim into
// PgBackupInstanceStatus.DumpOptions once a dump has run.
type PgBackupDumpOptions struct {
	// Format selects the pg_dump --format value. The "directory" format is
	// tarred and gzipped into a single object before upload.
	// +kubebuilder:default=custom
	// +optional
	Format PgBackupDumpFormat `json:"format,omitempty"`
	// CompressionLevel is passed as pg_dump --compress=<n>.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=9
	// +optional
	CompressionLevel *int32 `json:"compressionLevel,omitempty"`
}

// PgBackupPolicySpec defines the desired state of PgBackupPolicy
type PgBackupPolicySpec struct {
	// Schedule is a cron expression controlling how often the dump CronJob runs.
	Schedule string `json:"schedule"`
	// Storage selects and configures the backup destination.
	Storage PgBackupStorage `json:"storage"`
	// Encryption optionally configures at-rest encryption. Unset means
	// dumps are stored unencrypted.
	// +optional
	Encryption *PgBackupEncryption `json:"encryption,omitempty"`
	// Retention configures how long/how many backups are kept per database.
	Retention PgBackupRetention `json:"retention"`
	// DumpOptions configures defaults applied to every dump this policy
	// triggers.
	// +optional
	DumpOptions PgBackupDumpOptions `json:"dumpOptions,omitempty"`
	// WorkspaceSizeLimit bounds the ephemeral scratch volume used for the
	// dump (and its encrypted copy) during a backup run. Should be sized to
	// roughly 2x the largest target database's on-disk size. Defaults to
	// 10Gi if unset.
	// +kubebuilder:default="10Gi"
	// +optional
	WorkspaceSizeLimit *resource.Quantity `json:"workspaceSizeLimit,omitempty"`
}

// PgBackupPolicyStatus defines the observed state of PgBackupPolicy
type PgBackupPolicyStatus struct {
	// Conditions represent the current state of this policy.
	// Supported Condition Types:
	// - pgbackuppolicy.postgres.oebc.tools/ready true if storage/encryption secrets resolve
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PgBackupPolicy is the Schema for the pgbackuppolicies API
type PgBackupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PgBackupPolicySpec   `json:"spec,omitempty"`
	Status PgBackupPolicyStatus `json:"status,omitempty"`
}

func (p *PgBackupPolicy) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

func (p *PgBackupPolicy) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

func (p *PgBackupPolicy) ToNamespacedName() string {
	return p.Namespace + "/" + p.Name
}

//+kubebuilder:object:root=true

// PgBackupPolicyList contains a list of PgBackupPolicy
type PgBackupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PgBackupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PgBackupPolicy{}, &PgBackupPolicyList{})
}

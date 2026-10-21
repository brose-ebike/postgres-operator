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

// PgBackupStorageS3 configures an S3-compatible (MinIO) destination. Every
// field is a PgProperty (sourceable from a literal value, a ConfigMap, or a
// Secret), matching the convention PgInstance already uses for its own
// connection fields - credentials aren't singled out for special
// treatment, the whole destination config is uniformly resolvable.
// Secure is the one exception: it stays a plain *bool (see IsSecure's doc
// comment) since PgProperty only ever resolves to a string, and there's no
// realistic case for sourcing a TLS on/off toggle from a Secret/ConfigMap.
type PgBackupStorageS3 struct {
	// Endpoint is the S3-compatible endpoint, host[:port], without scheme.
	// +optional
	Endpoint PgProperty `json:"endpoint,omitempty"`
	// Bucket is the name of the bucket dumps are stored in. It is created
	// automatically if it does not exist.
	// +optional
	Bucket PgProperty `json:"bucket,omitempty"`
	// Prefix is an optional key prefix under which objects are stored.
	// +optional
	Prefix PgProperty `json:"prefix,omitempty"`
	// Secure selects whether TLS is used to talk to the endpoint.
	// A *bool, not bool: with a plain bool, encoding/json's "omitempty"
	// drops an explicit "false" from the request body indistinguishably
	// from "unset", so the apiserver's CRD default (true) would silently
	// override it on every create/update.
	// +kubebuilder:default=true
	// +optional
	Secure *bool `json:"secure,omitempty"`
	// AccessKey resolves the S3 access key.
	// +optional
	AccessKey PgProperty `json:"accessKey,omitempty"`
	// SecretKey resolves the S3 secret key.
	// +optional
	SecretKey PgProperty `json:"secretKey,omitempty"`
}

// IsSecure reports whether TLS should be used, defaulting to true (matching
// the CRD's kubebuilder default) when unset - relevant for Go-constructed
// values that never round-trip through the apiserver's CRD defaulting.
func (s *PgBackupStorageS3) IsSecure() bool {
	return s.Secure == nil || *s.Secure
}

// PgBackupStorage is a discriminated union selecting the storage backend.
// Exactly the sub-struct matching Type is expected to be populated. Used by
// PgBackupPolicySpec.Storage; PgBackupInstanceStatus.Location uses the
// separate PgBackupStorageLocation type instead (see its doc comment for
// why they aren't shared).
// +kubebuilder:validation:XValidation:rule="self.type != 's3' || has(self.s3)",message="s3 must be set when type is \"s3\""
type PgBackupStorage struct {
	// Type selects which of the sub-structs below is populated.
	Type PgBackupStorageType `json:"type"`
	// S3 holds the configuration when Type is "s3".
	// +optional
	S3 *PgBackupStorageS3 `json:"s3,omitempty"`
}

// PgBackupStorageS3Location records where a specific backup was actually
// stored - resolved, observed values, not PgProperty references. Unlike
// PgBackupStorageS3, there's no AccessKey/SecretKey here at all: credentials
// always come from the policy's *current* storage config, never a stale
// snapshot (see pkg/backup/worker's locationFor/destinationFromLocation).
type PgBackupStorageS3Location struct {
	// Endpoint is the resolved S3-compatible endpoint used for this backup.
	Endpoint string `json:"endpoint,omitempty"`
	// Bucket is the bucket this backup was stored in.
	Bucket string `json:"bucket,omitempty"`
	// Prefix is the resolved key prefix used for this backup. Required to
	// rebuild the exact Destination this object was stored under later
	// (cleanup must apply the same prefix the original upload did, or it
	// computes the wrong object key and silently fails to delete the real
	// object while still reporting cleanup success).
	Prefix string `json:"prefix,omitempty"`
	// Secure is whether TLS was used for this backup.
	Secure bool `json:"secure,omitempty"`
	// URL gives the fully resolved object address.
	URL string `json:"url,omitempty"`
}

// PgBackupStorageLocation is a discriminated union mirroring PgBackupStorage,
// used for PgBackupInstanceStatus.Location. Deliberately a separate type
// rather than reusing PgBackupStorage: a status field should record
// resolved facts about where a backup landed, not another layer of
// PgProperty references (which would always sit empty here - a status
// object isn't where you configure a Secret/ConfigMap lookup).
type PgBackupStorageLocation struct {
	// Type selects which of the sub-structs below is populated.
	Type PgBackupStorageType `json:"type"`
	// S3 holds the configuration when Type is "s3".
	// +optional
	S3 *PgBackupStorageS3Location `json:"s3,omitempty"`
}

// +kubebuilder:validation:Enum=gpg-rsa;gpg-aes
type PgBackupEncryptionType string

const (
	PgBackupEncryptionTypeGPGRSA PgBackupEncryptionType = "gpg-rsa"
	PgBackupEncryptionTypeGPGAES PgBackupEncryptionType = "gpg-aes"
)

// PgBackupEncryption configures at-rest encryption of the dump before
// upload. Exactly one of PublicKey (gpg-rsa, an armored public key) or
// Passphrase (gpg-aes, a symmetric passphrase) is expected to be populated,
// matching Type - two explicit, named fields rather than one generic
// "key material" property so the CRD schema itself documents which value
// each scheme needs.
// +kubebuilder:validation:XValidation:rule="self.type != 'gpg-rsa' || has(self.publicKey)",message="publicKey must be set when type is \"gpg-rsa\""
// +kubebuilder:validation:XValidation:rule="self.type != 'gpg-aes' || has(self.passphrase)",message="passphrase must be set when type is \"gpg-aes\""
type PgBackupEncryption struct {
	// Type selects the encryption scheme.
	Type PgBackupEncryptionType `json:"type"`
	// PublicKey resolves an armored GPG public key, used when Type is gpg-rsa.
	// +optional
	PublicKey *PgProperty `json:"publicKey,omitempty"`
	// Passphrase resolves a symmetric passphrase, used when Type is gpg-aes.
	// +optional
	Passphrase *PgProperty `json:"passphrase,omitempty"`
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

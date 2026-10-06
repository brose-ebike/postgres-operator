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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultFinalizerPgBackupInstance = "postgres.oebc.tools/pgbackupinstance"

// +kubebuilder:validation:Enum=Pending;Success;Failure
type PgBackupInstancePhase string

const (
	PgBackupInstancePhasePending PgBackupInstancePhase = "Pending"
	PgBackupInstancePhaseSuccess PgBackupInstancePhase = "Success"
	PgBackupInstancePhaseFailure PgBackupInstancePhase = "Failure"
)

// PgBackupInstanceSpec defines the desired state of PgBackupInstance
type PgBackupInstanceSpec struct {
	// Database identifies the PgDatabase this backup was taken of.
	Database PgInstanceRef `json:"database"`
	// BackupPolicy identifies the PgBackupPolicy that triggered this backup.
	BackupPolicy PgInstanceRef `json:"backupPolicy"`
}

// PgBackupInstanceStatus defines the observed state of PgBackupInstance
type PgBackupInstanceStatus struct {
	// Phase is the current lifecycle phase of this backup attempt.
	// +kubebuilder:default=Pending
	// +optional
	Phase PgBackupInstancePhase `json:"phase,omitempty"`
	// Location is where the (possibly encrypted) dump object was stored.
	// Only populated once Phase is Success. Same discriminated-union shape
	// as PgBackupPolicySpec.Storage.
	// +optional
	Location *PgBackupStorage `json:"location,omitempty"`
	// DumpOptions is a snapshot of the options actually used for this
	// backup, independent of later policy edits.
	// +optional
	DumpOptions PgBackupDumpOptions `json:"dumpOptions,omitempty"`
	// SizeBytes is the size in bytes of the stored (possibly encrypted) object.
	// +optional
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
	// StartedAt is when the backup attempt started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// FinishedAt is when the backup attempt finished (success or failure).
	// This is the authoritative timestamp used by the retention/cleanup
	// salvage algorithm.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
	// ExpiresAt is an informational cache computed from FinishedAt and the
	// policy's retention settings at the time of the backup. Cleanup always
	// recomputes retention from FinishedAt against the policy's current
	// settings rather than trusting this field.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
	// Message carries a human readable status/error message.
	// +optional
	Message string `json:"message,omitempty"`
	// Conditions represent the current state of this backup.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PgBackupInstance is the Schema for the pgbackupinstances API
type PgBackupInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PgBackupInstanceSpec   `json:"spec,omitempty"`
	Status PgBackupInstanceStatus `json:"status,omitempty"`
}

func (i *PgBackupInstance) GetConditions() []metav1.Condition {
	return i.Status.Conditions
}

func (i *PgBackupInstance) SetConditions(conditions []metav1.Condition) {
	i.Status.Conditions = conditions
}

func (i *PgBackupInstance) ToNamespacedName() string {
	return i.Namespace + "/" + i.Name
}

//+kubebuilder:object:root=true

// PgBackupInstanceList contains a list of PgBackupInstance
type PgBackupInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PgBackupInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PgBackupInstance{}, &PgBackupInstanceList{})
}

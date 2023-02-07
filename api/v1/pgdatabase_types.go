/*
Copyright 2023 Brose Fahrzeugteile SE & Co. KG, Bamberg.

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
	"k8s.io/apimachinery/pkg/types"
)

// DefaultFinalizerPgDatabase contains the name for the default finalizer
// of the PgDatabase resource
const DefaultFinalizerPgDatabase = "postgres.brose.bike/pgdatabase"
const PgDatabaseExistsConditionType string = "pgdatabase.postgres.brose.bike/exists"
const PgDatabaseExtensionsConditionType string = "pgdatabase.postgres.brose.bike/extensions"

// +kubebuilder:validation:Enum=USAGE;CREATE
type SchemaPrivilege string

// +kubebuilder:validation:Enum=SELECT;INSERT;UPDATE;DELETE;TRUNCATE;REFERENCES;TRIGGER
type TablePrivilege string

// +kubebuilder:validation:Enum=SELECT;UPDATE;USAGE
type SequencePrivilege string

// +kubebuilder:validation:Enum=EXECUTE
type FunctionPrivilege string

// +kubebuilder:validation:Enum=USAGE
type TypePrivilege string

type PgDatabaseDeletion struct {
	// Drop specifies if the database should be dropped on deletion (defaults to false)
	Drop bool `json:"drop,omitempty"`
	// Wait specifies if the finalizer should wait for the database to be deleted manually
	Wait bool `json:"wait,omitempty"`
}

type PgDatabaseDefaultPrivileges struct {
	// Name specifies the name of the schema for which the default privileges should be granted.
	Name string `json:"name"`
	// Roles specifies the name of the roles for which the privileges should be granted
	Roles []string `json:"roles"`
	// SchemaPrivileges specifies the privileges on this schema which should be granted to the roles
	SchemaPrivileges []SchemaPrivilege `json:"privileges,omitempty"`
	// TablePrivileges specifies the name of the privileges on tables which should be granted to the roles
	TablePrivileges []TablePrivilege `json:"tablePrivileges,omitempty"`
	// SequencePrivileges specifies the name of the privileges on tables which should be granted to the roles
	SequencePrivileges []SequencePrivilege `json:"sequencePrivileges,omitempty"`
	// FunctionPrivileges specifies the name of the privileges on tables which should be granted to the roles
	FunctionPrivileges []FunctionPrivilege `json:"functionPrivileges,omitempty"`
	// TypePrivileges specifies the name of the privileges on tables which should be granted to the roles
	TypePrivileges []TypePrivilege `json:"typePrivileges,omitempty"`
}

func (dp *PgDatabaseDefaultPrivileges) PrivilegesStr() []string {
	privileges := make([]string, len(dp.SchemaPrivileges))
	for i := range dp.SchemaPrivileges {
		privileges[i] = string(dp.SchemaPrivileges[i])
	}
	return privileges
}

func (dp *PgDatabaseDefaultPrivileges) TablePrivilegesStr() []string {
	privileges := make([]string, len(dp.TablePrivileges))
	for i := range dp.TablePrivileges {
		privileges[i] = string(dp.TablePrivileges[i])
	}
	return privileges
}

func (dp *PgDatabaseDefaultPrivileges) SequencePrivilegesStr() []string {
	privileges := make([]string, len(dp.SequencePrivileges))
	for i := range dp.SequencePrivileges {
		privileges[i] = string(dp.SequencePrivileges[i])
	}
	return privileges
}

func (dp *PgDatabaseDefaultPrivileges) FunctionPrivilegesStr() []string {
	privileges := make([]string, len(dp.FunctionPrivileges))
	for i := range dp.FunctionPrivileges {
		privileges[i] = string(dp.FunctionPrivileges[i])
	}
	return privileges
}

func (dp *PgDatabaseDefaultPrivileges) TypePrivilegesStr() []string {
	privileges := make([]string, len(dp.TypePrivileges))
	for i := range dp.TypePrivileges {
		privileges[i] = string(dp.TypePrivileges[i])
	}
	return privileges
}

type PgDatabasePublicPrivileges struct {
	// Revoke the public privileges from all database object
	Revoke bool `json:"revoke"`
}
type PgDatabasePublicSchema struct {
	// Revoke the public privileges from all database object
	Drop bool `json:"drop"`
}

// PgDatabaseSpec defines the desired state of PgDatabase
type PgDatabaseSpec struct {
	// Instance identifies the PgInstanceConnection which should be used
	Instance PgInstanceRef `json:"instance"`
	// DeletionBehavior specifies what should happen when the manifest gets deleted
	DeletionBehavior PgDatabaseDeletion `json:"deletion"`
	// Extensions which should exist in this database
	Extensions []string `json:"extensions,omitempty"`
	// DefaultPrivileges defines the default privileges for this database
	DefaultPrivileges []PgDatabaseDefaultPrivileges `json:"defaultPrivileges,omitempty"`
	// PublicPrivileges revokes and Public stuff in postgres
	PublicPrivileges PgDatabasePublicPrivileges `json:"publicPrivileges"`
	// PublicSchema dropped
	PublicSchema PgDatabasePublicSchema `json:"publicSchema"`
}

// PgDatabaseStatus defines the observed state of PgDatabase
type PgDatabaseStatus struct {
	// Conditions represent the current connection state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PgDatabase is the Schema for the pgdatabases API
type PgDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PgDatabaseSpec   `json:"spec,omitempty"`
	Status PgDatabaseStatus `json:"status,omitempty"`
}

func (d *PgDatabase) GetConditions() []metav1.Condition {
	return d.Status.Conditions
}

func (d *PgDatabase) SetConditions(conditions []metav1.Condition) {
	d.Status.Conditions = conditions
}

func (d *PgDatabase) GetInstanceId() types.NamespacedName {
	return d.Spec.Instance.ToNamespacedName()
}

func (d *PgDatabase) GetInstanceIdString() string {
	return d.Spec.Instance.ToNamespacedName().String()
}

func (d *PgDatabase) ToNamespacedName() string {
	return d.Namespace + "/" + d.Name
}

//+kubebuilder:object:root=true

// PgDatabaseList contains a list of PgDatabase
type PgDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PgDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PgDatabase{}, &PgDatabaseList{})
}

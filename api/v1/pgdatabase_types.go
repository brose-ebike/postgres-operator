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
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
const DefaultFinalizerPgDatabase = "postgres.brose.bike/pgdatabase"

type PgDatabaseDeletion struct {
	// Drop specifies if the database should be dropped on deletion (defaults to false)
	Drop bool `json:"drop,omitempty"`
	// Wait specifies if the finalizer should wait for the database to be deleted manually
	Wait bool `json:"wait,omitempty"`
}

type PgDatabaseDefaultPrivileges struct {
	// Roles specifies the name of the roles for which the privileges should be granted
	Roles []string `json:"roles"`
	// Name specifies the name of the schema for which the default privileges should be granted.
	Name string `json:"name"`
	// TablePrivileges specifies the name of the privileges on tables which should be granted to the roles
	TablePrivileges []string `json:"tablePrivileges,omitempty"`
	// SequencePrivileges specifies the name of the privileges on tables which should be granted to the roles
	SequencePrivileges []string `json:"sequencePrivileges,omitempty"`
	// FunctionPrivileges specifies the name of the privileges on tables which should be granted to the roles
	FunctionPrivileges []string `json:"functionPrivileges,omitempty"`
	// RoutinePrivileges specifies the name of the privileges on tables which should be granted to the roles
	RoutinePrivileges []string `json:"routinePrivileges,omitempty"`
	// TypePrivileges specifies the name of the privileges on tables which should be granted to the roles
	TypePrivileges []string `json:"typePrivileges,omitempty"`
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
	// DefaultPrivileges defines the default privileges for this database
	DefaultPrivileges []PgDatabaseDefaultPrivileges `json:"defaultPrivileges"`
	// PublicPrivileges revokes and Public stuff in postgres
	PublicPrivileges PgDatabasePublicPrivileges `json:"publicPrivileges"`
	// PublicSchema dropped
	PublicSchema PgDatabasePublicSchema `json:"publicSchema"`
}

// PgDatabaseStatus defines the observed state of PgDatabase
type PgDatabaseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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

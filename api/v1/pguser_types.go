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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const DefaultFinalizerPgUser = "postgres.brose.bike/pgloginrole"
const PgUserExistsConditionType string = "postgres.brose.bike/login-role-exists"

// +kubebuilder:validation:Enum=CONNECT;CREATE
type DatabasePrivilege string

const (
	// Allows the grantee to connect to the database. This privilege is checked at connection startup (in addition to checking any restrictions imposed by pg_hba.conf).
	ConnectDatabasePrivilege DatabasePrivilege = "CONNECT"

	// Allows new schemas and publications to be created within the database, and allows trusted extensions to be installed within the database.
	CreateDatabasePrivilege DatabasePrivilege = "CREATE"
)

// PgLoginRoleSecret identifies the PgLoginRoleSecret which should be used
type PgUserSecret struct {
	// Name identifies the PgLoginRoleSecret which should be used
	Name string `json:"name,omitempty"`
}

// PgUserDatabase represents the database a user would like to connect to
type PgUserDatabase struct {
	// Name contains the Database Name on the postgres instance
	Name string `json:"name,omitempty"`
	// Owner is the optional value which allows to set this user as owner of a database
	// +optional
	Owner *bool `json:"owner,omitempty"`
	// Privileges contains the names of the privileges the user needs on the database
	Privileges []DatabasePrivilege `json:"privileges"`
	// TODO add schemas, tables, etc.
}

func (d *PgUserDatabase) IsOwner() bool {
	return d.Owner != nil && *d.Owner
}

// PgUserSpec defines the desired state of PgUser
type PgUserSpec struct {
	// Instance identifies the PgInstanceConnection which should be used
	Instance PgInstanceRef `json:"instance"`
	// Secret is an example field of PgLoginRole
	Secret *PgUserSecret `json:"secret,omitempty"`
	// Databases is an example field of PgLoginRole
	Databases []PgUserDatabase `json:"databases,omitempty"`
}

// PgUserStatus defines the observed state of PgUser
type PgUserStatus struct {
	// Conditions represent the current connection state
	// Supported Condition Types:
	// - postgres.brose.bike/login-role-exists true if login role exists false if not
	// - postgres.brose.bike/connected true if the instance is reachable false if not
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PgUser is the Schema for the pgusers API
type PgUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PgUserSpec   `json:"spec,omitempty"`
	Status PgUserStatus `json:"status,omitempty"`
}

func PgUserKind() string {
	obj := &PgUser{}
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Pointer {
		panic("All types must be pointers to structs.")
	}
	return t.Elem().Name()
}

func (u *PgUser) GetConditions() []metav1.Condition {
	return u.Status.Conditions
}

func (u *PgUser) SetConditions(conditions []metav1.Condition) {
	u.Status.Conditions = conditions
}

func (u *PgUser) GetInstanceId() types.NamespacedName {
	return u.Spec.Instance.ToNamespacedName()
}

func (u *PgUser) GetInstanceIdString() string {
	return u.Spec.Instance.ToNamespacedName().String()
}

func (u *PgUser) ToNamespacedName() string {
	return u.Namespace + "/" + u.Name
}

//+kubebuilder:object:root=true

// PgUserList contains a list of PgUser
type PgUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PgUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PgUser{}, &PgUserList{})
}

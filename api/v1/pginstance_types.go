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
	"context"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PgInstanceSpec defines the desired state of PgInstance
type PgInstanceSpec struct {
	// The Hostname of the server which should be managed
	Hostname PgProperty `json:"host,omitempty"`
	// The Port of the server which should be managed, defaults to 5432
	Port PgProperty `json:"port,omitempty"`
	// The Username for the Administrator User which will be used to create, update and delete databases and users
	Username PgProperty `json:"username,omitempty"`
	// The Password for the Administrator User which will be used to create, update and delete databases and users
	Password PgProperty `json:"password,omitempty"`
	// The Maintenance Database which should be used to establish the connection, defaults to 'postgres'
	Database PgProperty `json:"database,omitempty"`
	// The SSLMode which should be used for the connection, defaults to 'none'
	SSLMode PgProperty `json:"sslMode,omitempty"`
}

func (s *PgInstanceSpec) GetHostname(ctx context.Context, r client.Reader, namespace string) (string, error) {
	return s.Hostname.GetPropertyValue(ctx, r, namespace, "host")
}

func (s *PgInstanceSpec) GetPort(ctx context.Context, r client.Reader, namespace string) (int, error) {
	value, err := s.Port.GetPropertyValueWithDefault(ctx, r, namespace, "port", "5432")
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return port, nil
}

func (s *PgInstanceSpec) GetUsername(ctx context.Context, r client.Reader, namespace string) (string, error) {
	return s.Username.GetPropertyValue(ctx, r, namespace, "username")
}

func (s *PgInstanceSpec) GetPassword(ctx context.Context, r client.Reader, namespace string) (string, error) {
	return s.Password.GetPropertyValue(ctx, r, namespace, "password")
}

func (s *PgInstanceSpec) GetDatabase(ctx context.Context, r client.Reader, namespace string) (string, error) {
	return s.Database.GetPropertyValueWithDefault(ctx, r, namespace, "database", "postgres")
}

func (s *PgInstanceSpec) GetSSLMode(ctx context.Context, r client.Reader, namespace string) (string, error) {
	return s.SSLMode.GetPropertyValueWithDefault(ctx, r, namespace, "sslMode", "none")
}

// PgInstanceStatus defines the observed state of PgInstance
type PgInstanceStatus struct {
	// Conditions represent the current connection state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PgInstance is the Schema for the pginstances API
type PgInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PgInstanceSpec   `json:"spec,omitempty"`
	Status PgInstanceStatus `json:"status,omitempty"`
}

func (i *PgInstance) GetConditions() []metav1.Condition {
	return i.Status.Conditions
}

func (i *PgInstance) SetConditions(conditions []metav1.Condition) {
	i.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// PgInstanceList contains a list of PgInstance
type PgInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PgInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PgInstance{}, &PgInstanceList{})
}

//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabase) DeepCopyInto(out *PgDatabase) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabase.
func (in *PgDatabase) DeepCopy() *PgDatabase {
	if in == nil {
		return nil
	}
	out := new(PgDatabase)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PgDatabase) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabaseDefaultPrivileges) DeepCopyInto(out *PgDatabaseDefaultPrivileges) {
	*out = *in
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SchemaPrivileges != nil {
		in, out := &in.SchemaPrivileges, &out.SchemaPrivileges
		*out = make([]SchemaPrivilege, len(*in))
		copy(*out, *in)
	}
	if in.TablePrivileges != nil {
		in, out := &in.TablePrivileges, &out.TablePrivileges
		*out = make([]TablePrivilege, len(*in))
		copy(*out, *in)
	}
	if in.SequencePrivileges != nil {
		in, out := &in.SequencePrivileges, &out.SequencePrivileges
		*out = make([]SequencePrivilege, len(*in))
		copy(*out, *in)
	}
	if in.FunctionPrivileges != nil {
		in, out := &in.FunctionPrivileges, &out.FunctionPrivileges
		*out = make([]FunctionPrivilege, len(*in))
		copy(*out, *in)
	}
	if in.TypePrivileges != nil {
		in, out := &in.TypePrivileges, &out.TypePrivileges
		*out = make([]TypePrivilege, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabaseDefaultPrivileges.
func (in *PgDatabaseDefaultPrivileges) DeepCopy() *PgDatabaseDefaultPrivileges {
	if in == nil {
		return nil
	}
	out := new(PgDatabaseDefaultPrivileges)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabaseDeletion) DeepCopyInto(out *PgDatabaseDeletion) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabaseDeletion.
func (in *PgDatabaseDeletion) DeepCopy() *PgDatabaseDeletion {
	if in == nil {
		return nil
	}
	out := new(PgDatabaseDeletion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabaseList) DeepCopyInto(out *PgDatabaseList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PgDatabase, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabaseList.
func (in *PgDatabaseList) DeepCopy() *PgDatabaseList {
	if in == nil {
		return nil
	}
	out := new(PgDatabaseList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PgDatabaseList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabasePublicPrivileges) DeepCopyInto(out *PgDatabasePublicPrivileges) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabasePublicPrivileges.
func (in *PgDatabasePublicPrivileges) DeepCopy() *PgDatabasePublicPrivileges {
	if in == nil {
		return nil
	}
	out := new(PgDatabasePublicPrivileges)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabasePublicSchema) DeepCopyInto(out *PgDatabasePublicSchema) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabasePublicSchema.
func (in *PgDatabasePublicSchema) DeepCopy() *PgDatabasePublicSchema {
	if in == nil {
		return nil
	}
	out := new(PgDatabasePublicSchema)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabaseSpec) DeepCopyInto(out *PgDatabaseSpec) {
	*out = *in
	out.Instance = in.Instance
	out.DeletionBehavior = in.DeletionBehavior
	if in.Extensions != nil {
		in, out := &in.Extensions, &out.Extensions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DefaultPrivileges != nil {
		in, out := &in.DefaultPrivileges, &out.DefaultPrivileges
		*out = make([]PgDatabaseDefaultPrivileges, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.PublicPrivileges = in.PublicPrivileges
	out.PublicSchema = in.PublicSchema
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabaseSpec.
func (in *PgDatabaseSpec) DeepCopy() *PgDatabaseSpec {
	if in == nil {
		return nil
	}
	out := new(PgDatabaseSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgDatabaseStatus) DeepCopyInto(out *PgDatabaseStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgDatabaseStatus.
func (in *PgDatabaseStatus) DeepCopy() *PgDatabaseStatus {
	if in == nil {
		return nil
	}
	out := new(PgDatabaseStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgInstance) DeepCopyInto(out *PgInstance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgInstance.
func (in *PgInstance) DeepCopy() *PgInstance {
	if in == nil {
		return nil
	}
	out := new(PgInstance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PgInstance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgInstanceList) DeepCopyInto(out *PgInstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PgInstance, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgInstanceList.
func (in *PgInstanceList) DeepCopy() *PgInstanceList {
	if in == nil {
		return nil
	}
	out := new(PgInstanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PgInstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgInstanceRef) DeepCopyInto(out *PgInstanceRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgInstanceRef.
func (in *PgInstanceRef) DeepCopy() *PgInstanceRef {
	if in == nil {
		return nil
	}
	out := new(PgInstanceRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgInstanceSpec) DeepCopyInto(out *PgInstanceSpec) {
	*out = *in
	in.Hostname.DeepCopyInto(&out.Hostname)
	in.Port.DeepCopyInto(&out.Port)
	in.Username.DeepCopyInto(&out.Username)
	in.Password.DeepCopyInto(&out.Password)
	in.Database.DeepCopyInto(&out.Database)
	in.SSLMode.DeepCopyInto(&out.SSLMode)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgInstanceSpec.
func (in *PgInstanceSpec) DeepCopy() *PgInstanceSpec {
	if in == nil {
		return nil
	}
	out := new(PgInstanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgInstanceStatus) DeepCopyInto(out *PgInstanceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgInstanceStatus.
func (in *PgInstanceStatus) DeepCopy() *PgInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(PgInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgProperty) DeepCopyInto(out *PgProperty) {
	*out = *in
	if in.ConfigMapKeyRef != nil {
		in, out := &in.ConfigMapKeyRef, &out.ConfigMapKeyRef
		*out = new(corev1.ConfigMapKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.SecretKeyRef != nil {
		in, out := &in.SecretKeyRef, &out.SecretKeyRef
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgProperty.
func (in *PgProperty) DeepCopy() *PgProperty {
	if in == nil {
		return nil
	}
	out := new(PgProperty)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgUser) DeepCopyInto(out *PgUser) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgUser.
func (in *PgUser) DeepCopy() *PgUser {
	if in == nil {
		return nil
	}
	out := new(PgUser)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PgUser) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgUserDatabase) DeepCopyInto(out *PgUserDatabase) {
	*out = *in
	if in.Owner != nil {
		in, out := &in.Owner, &out.Owner
		*out = new(bool)
		**out = **in
	}
	if in.Privileges != nil {
		in, out := &in.Privileges, &out.Privileges
		*out = make([]DatabasePrivilege, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgUserDatabase.
func (in *PgUserDatabase) DeepCopy() *PgUserDatabase {
	if in == nil {
		return nil
	}
	out := new(PgUserDatabase)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgUserList) DeepCopyInto(out *PgUserList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PgUser, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgUserList.
func (in *PgUserList) DeepCopy() *PgUserList {
	if in == nil {
		return nil
	}
	out := new(PgUserList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PgUserList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgUserSecret) DeepCopyInto(out *PgUserSecret) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgUserSecret.
func (in *PgUserSecret) DeepCopy() *PgUserSecret {
	if in == nil {
		return nil
	}
	out := new(PgUserSecret)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgUserSpec) DeepCopyInto(out *PgUserSpec) {
	*out = *in
	out.Instance = in.Instance
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(PgUserSecret)
		**out = **in
	}
	if in.Databases != nil {
		in, out := &in.Databases, &out.Databases
		*out = make([]PgUserDatabase, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgUserSpec.
func (in *PgUserSpec) DeepCopy() *PgUserSpec {
	if in == nil {
		return nil
	}
	out := new(PgUserSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PgUserStatus) DeepCopyInto(out *PgUserStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PgUserStatus.
func (in *PgUserStatus) DeepCopy() *PgUserStatus {
	if in == nil {
		return nil
	}
	out := new(PgUserStatus)
	in.DeepCopyInto(out)
	return out
}

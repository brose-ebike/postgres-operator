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

	"github.com/brose-ebike/postgres-controller/pkg/brose_errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coreV1 "k8s.io/api/core/v1"
)

const PgConnectedConditionType string = "postgres.brose.bike/connected"

const (
	PgConnectedConditionReasonConSucceeded = "ConnectionSucceeded"
	PgConnectedConditionReasonConFailed    = "ConnectionFailed"
)

type PgProperty struct {
	// The value for this property
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,1,opt,name=value"`
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMapKeyRef *coreV1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty" protobuf:"bytes,2,opt,name=configMapKeyRef"`
	// Selects a key of a secret in the pod's namespace
	// +optional
	SecretKeyRef *coreV1.SecretKeySelector `json:"secretKeyRef,omitempty" protobuf:"bytes,3,opt,name=secretKeyRef"`
}

func (p *PgProperty) GetPropertyValueWithDefault(ctx context.Context, r client.Reader, namespace string, name string, defaultValue string) (string, error) {
	value, err := p.GetPropertyValue(ctx, r, namespace, name)
	if err == nil {
		return value, nil
	}
	if _, ok := err.(*brose_errors.MissingPropertyValueError); ok {
		return defaultValue, nil
	}
	return "", err
}

func (p *PgProperty) GetPropertyValue(ctx context.Context, r client.Reader, namespace string, name string) (string, error) {
	// Read from value
	if p.Value != "" {
		return p.Value, nil
	}
	// Read from config map
	if p.ConfigMapKeyRef != nil {
		var configMap coreV1.ConfigMap
		objectKey := types.NamespacedName{Namespace: namespace, Name: p.ConfigMapKeyRef.Name}
		err := r.Get(ctx, objectKey, &configMap)
		if errors.IsNotFound(err) {
			return "", err
		}
		key := p.ConfigMapKeyRef.Key
		if value, found := configMap.Data[key]; found {
			return value, nil
		}
		return "", brose_errors.NewMapEntryNotFoundError(key)
	}
	// Read from secret
	if p.SecretKeyRef != nil {
		var secret coreV1.Secret
		objectKey := types.NamespacedName{Namespace: namespace, Name: p.SecretKeyRef.Name}
		err := r.Get(ctx, objectKey, &secret)
		if errors.IsNotFound(err) {
			return "", err
		}
		key := p.SecretKeyRef.Key
		valueBase64, foundBase64 := secret.Data[key]
		valueString, foundString := secret.StringData[key]
		if !foundBase64 && !foundString {
			return "", brose_errors.NewMapEntryNotFoundError(key)
		} else if foundString {
			return valueString, nil
		}
		return string(valueBase64), nil
	}
	return "", brose_errors.NewMissingPropertyValueError(name)
}

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
	"testing"

	"github.com/google/go-cmp/cmp"
	coreV1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockReader struct {
	callsGet int
	proxyGet func(key client.ObjectKey, obj client.Object) error
}

func (r *mockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	r.callsGet += 1
	if r.proxyGet != nil {
		return r.proxyGet(key, obj)
	}
	return nil
}

func (r *mockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func TestPgPropertyWithValue(t *testing.T) {
	// given
	property := PgProperty{Value: "value"}

	// and
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// and
	reader := mockReader{}

	// when
	actual, err := property.GetPropertyValue(ctx, &reader, "default", "")

	// then
	if err != nil {
		t.Errorf("Error should be nil: %v", err)
	}
	// and
	expected := "value"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Value is incorrect (-want +got):\n%s", diff)
	}
	if reader.callsGet != 0 {
		t.Errorf("Unexpected call to reader, expected 0 calls, got %d", reader.callsGet)
	}
}

func TestPgPropertyWithConfigMap(t *testing.T) {
	// given
	property := PgProperty{
		ConfigMapKeyRef: &coreV1.ConfigMapKeySelector{
			Key:                  "key",
			LocalObjectReference: coreV1.LocalObjectReference{Name: "test"},
		},
	}

	// and
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// and
	reader := mockReader{}
	reader.proxyGet = func(key client.ObjectKey, obj client.Object) error {
		configMap := obj.(*coreV1.ConfigMap)
		configMap.Data = map[string]string{
			"key": "value",
		}
		return nil
	}

	// when
	actual, err := property.GetPropertyValue(ctx, &reader, "default", "")

	// then
	if err != nil {
		t.Errorf("Error should be nil: %v", err)
	}
	// and
	expected := "value"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Value is incorrect (-want +got):\n%s", diff)
	}
	if reader.callsGet != 1 {
		t.Errorf("Unexpected call to reader, expected 1 calls, got %d", reader.callsGet)
	}
}

func TestPgPropertyWithSecretData(t *testing.T) {
	// given
	property := PgProperty{
		SecretKeyRef: &coreV1.SecretKeySelector{
			Key:                  "key",
			LocalObjectReference: coreV1.LocalObjectReference{Name: "test"},
		},
	}

	// and
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// and
	reader := mockReader{}
	reader.proxyGet = func(key client.ObjectKey, obj client.Object) error {
		secret := obj.(*coreV1.Secret)
		secret.Data = map[string][]byte{
			"key": []byte("value"),
		}
		return nil
	}

	// when
	actual, err := property.GetPropertyValue(ctx, &reader, "default", "")

	// then
	if err != nil {
		t.Errorf("Error should be nil: %v", err)
	}
	// and
	expected := "value"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Value is incorrect (-want +got):\n%s", diff)
	}
	if reader.callsGet != 1 {
		t.Errorf("Unexpected call to reader, expected 1 calls, got %d", reader.callsGet)
	}
}

func TestPgPropertyWithSecretStringData(t *testing.T) {
	// given
	property := PgProperty{
		SecretKeyRef: &coreV1.SecretKeySelector{
			Key:                  "key",
			LocalObjectReference: coreV1.LocalObjectReference{Name: "test"},
		},
	}

	// and
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// and
	reader := mockReader{}
	reader.proxyGet = func(key client.ObjectKey, obj client.Object) error {
		secret := obj.(*coreV1.Secret)
		secret.StringData = map[string]string{
			"key": "value",
		}
		return nil
	}

	// when
	actual, err := property.GetPropertyValue(ctx, &reader, "default", "")

	// then
	if err != nil {
		t.Errorf("Error should be nil: %v", err)
	}
	// and
	expected := "value"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Value is incorrect (-want +got):\n%s", diff)
	}
	if reader.callsGet != 1 {
		t.Errorf("Unexpected call to reader, expected 1 calls, got %d", reader.callsGet)
	}
}

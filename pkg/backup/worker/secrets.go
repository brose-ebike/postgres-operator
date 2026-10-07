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

package worker

import (
	"context"
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resolveSecretValue reads a single key out of a namespaced Secret. Mirrors
// api/v1.PgProperty.GetPropertyValue's secret-reading branch, inlined here
// since this package has no need for the ConfigMapKeyRef fallback.
func resolveSecretValue(ctx context.Context, c client.Reader, namespace string, name string, key string) (string, error) {
	var secret coreV1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &secret); err != nil {
		return "", fmt.Errorf("unable to read secret %q: %w", name, err)
	}
	if value, ok := secret.Data[key]; ok {
		return string(value), nil
	}
	if value, ok := secret.StringData[key]; ok {
		return value, nil
	}
	return "", fmt.Errorf("secret %q has no key %q", name, key)
}

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

package services

import (
	"context"
	"net"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("NewPgInstanceAPI", func() {

	It("generates a new valid instance", func() {
		// given:
		ctx := context.TODO()
		r := mockReader{}
		instance := apiV1.PgInstance{
			Spec: apiV1.PgInstanceSpec{
				Hostname: apiV1.PgProperty{Value: "localhost"},
				Port:     apiV1.PgProperty{Value: "5432"},
				Username: apiV1.PgProperty{Value: "admin"},
				Password: apiV1.PgProperty{Value: "admin"},
				Database: apiV1.PgProperty{Value: "postgres"},
				SSLMode:  apiV1.PgProperty{Value: "none"},
			},
		}

		// when:
		_, err := NewPgInstanceAPI(ctx, &r, &instance)

		// then: OpError with tcp connect failed, because not database is running
		Expect(err).ToNot(BeNil())
		Expect(err).To(BeAssignableToTypeOf(&net.OpError{}))
	})

})

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PgInstanceSpec", func() {

	It("get from value returns the stored values", func() {
		// given:
		instanceSpec := PgInstanceSpec{
			Hostname: PgProperty{Value: "hostname"},
			Port:     PgProperty{Value: "1234"},
			Username: PgProperty{Value: "username"},
			Password: PgProperty{Value: "password"},
			Database: PgProperty{Value: "database"},
			SSLMode:  PgProperty{Value: "sslmode"},
		}
		// when:
		ctx := context.TODO()
		r := mockReader{}

		// and:
		hostname, _ := instanceSpec.GetHostname(ctx, &r, "default")
		port, _ := instanceSpec.GetPort(ctx, &r, "default")
		username, _ := instanceSpec.GetUsername(ctx, &r, "default")
		password, _ := instanceSpec.GetPassword(ctx, &r, "default")
		database, _ := instanceSpec.GetDatabase(ctx, &r, "default")
		sslmode, _ := instanceSpec.GetSSLMode(ctx, &r, "default")

		// then:
		Expect(hostname).To(Equal("hostname"))
		Expect(port).To(Equal(1234))
		Expect(username).To(Equal("username"))
		Expect(password).To(Equal("password"))
		Expect(database).To(Equal("database"))
		Expect(sslmode).To(Equal("sslmode"))
	})

	It("get from secret returns the secret values", func() {
		// given:
		instanceSpec := PgInstanceSpec{
			Hostname: PgProperty{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"}, Key: "hostname"}},
			Port:     PgProperty{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"}, Key: "port"}},
			Username: PgProperty{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"}, Key: "username"}},
			Password: PgProperty{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"}, Key: "password"}},
			Database: PgProperty{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"}, Key: "database"}},
			SSLMode:  PgProperty{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"}, Key: "sslmode"}},
		}
		// and:
		ctx := context.TODO()
		r := mockReader{}

		// and:
		r.proxyGet = func(key client.ObjectKey, obj client.Object) error {
			if key.Name != "my-secret" || key.Namespace != "default" {
				panic("Invalid key name or namespace")
			}
			secret := obj.(*v1.Secret)
			secret.StringData = map[string]string{
				"hostname": "hostname+hash",
				"port":     "1234",
				"username": "username+hash",
				"password": "password+hash",
				"database": "database+hash",
				"sslmode":  "sslmode+hash",
			}
			return nil
		}

		// when:
		hostname, _ := instanceSpec.GetHostname(ctx, &r, "default")
		port, _ := instanceSpec.GetPort(ctx, &r, "default")
		username, _ := instanceSpec.GetUsername(ctx, &r, "default")
		password, _ := instanceSpec.GetPassword(ctx, &r, "default")
		database, _ := instanceSpec.GetDatabase(ctx, &r, "default")
		sslmode, _ := instanceSpec.GetSSLMode(ctx, &r, "default")

		// then:
		Expect(r.callsGet).To(Equal(6))

		// and:
		Expect(hostname).To(Equal("hostname+hash"))
		Expect(port).To(Equal(1234))
		Expect(username).To(Equal("username+hash"))
		Expect(password).To(Equal("password+hash"))
		Expect(database).To(Equal("database+hash"))
		Expect(sslmode).To(Equal("sslmode+hash"))
	})

	It("get from config map returns the config map entries", func() {
		// given:
		instanceSpec := PgInstanceSpec{
			Hostname: PgProperty{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-config-map"}, Key: "hostname"}},
			Port:     PgProperty{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-config-map"}, Key: "port"}},
			Username: PgProperty{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-config-map"}, Key: "username"}},
			Password: PgProperty{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-config-map"}, Key: "password"}},
			Database: PgProperty{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-config-map"}, Key: "database"}},
			SSLMode:  PgProperty{ConfigMapKeyRef: &v1.ConfigMapKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "my-config-map"}, Key: "sslmode"}},
		}
		// and:
		ctx := context.TODO()
		r := mockReader{}

		// and:
		r.proxyGet = func(key client.ObjectKey, obj client.Object) error {
			if key.Name != "my-config-map" || key.Namespace != "default" {
				panic("Invalid key name or namespace")
			}
			secret := obj.(*v1.ConfigMap)
			secret.Data = map[string]string{
				"hostname": "hostname+hash",
				"port":     "1234",
				"username": "username+hash",
				"password": "password+hash",
				"database": "database+hash",
				"sslmode":  "sslmode+hash",
			}
			return nil
		}

		// when:
		hostname, _ := instanceSpec.GetHostname(ctx, &r, "default")
		port, _ := instanceSpec.GetPort(ctx, &r, "default")
		username, _ := instanceSpec.GetUsername(ctx, &r, "default")
		password, _ := instanceSpec.GetPassword(ctx, &r, "default")
		database, _ := instanceSpec.GetDatabase(ctx, &r, "default")
		sslmode, _ := instanceSpec.GetSSLMode(ctx, &r, "default")

		// then:
		Expect(r.callsGet).To(Equal(6))

		// and:
		Expect(hostname).To(Equal("hostname+hash"))
		Expect(port).To(Equal(1234))
		Expect(username).To(Equal("username+hash"))
		Expect(password).To(Equal("password+hash"))
		Expect(database).To(Equal("database+hash"))
		Expect(sslmode).To(Equal("sslmode+hash"))
	})
})

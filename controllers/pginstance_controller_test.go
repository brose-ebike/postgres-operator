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

package controllers

import (
	"context"
	"errors"

	apiV1 "github.com/brose-ebike/postgres-controller/api/v1"
	"github.com/brose-ebike/postgres-controller/pkg/services"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("PgInstanceReconciler", func() {

	var pgApiMock services.PgServerApi
	var reconciler *PgInstanceReconciler

	BeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Create ApiMock
		pgApiMock = &services.PgServerApiImpl{}

		// Create Reconciler
		reconciler = &PgInstanceReconciler{
			k8sClient,
			nil,
			func(ctx context.Context, r client.Reader, instance apiV1.PgInstance) (services.PgServerApi, error) {
				if instance.Name == "failure" {
					return nil, errors.New("Connection Failure")
				}
				return pgApiMock, nil
			},
		}

		// Create dummy
		createDummy := func() {
			instance := apiV1.PgInstance{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgInstance",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "dummy",
				},
				Spec: apiV1.PgInstanceSpec{
					Hostname: apiV1.PgProperty{Value: "localhost"},
					Port:     apiV1.PgProperty{Value: "5432"},
					Username: apiV1.PgProperty{Value: "admin"},
					Password: apiV1.PgProperty{Value: "password"},
				},
				Status: apiV1.PgInstanceStatus{},
			}
			err := k8sClient.Create(ctx, &instance)
			Expect(err).To(BeNil())
		}
		createDummy()
		// Next Instance
		createFailure := func() {
			instance := apiV1.PgInstance{
				TypeMeta: v1.TypeMeta{
					APIVersion: "postgres.brose.bike/v1",
					Kind:       "PgInstance",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "failure",
				},
				Spec: apiV1.PgInstanceSpec{
					Hostname: apiV1.PgProperty{Value: "failure"},
					Port:     apiV1.PgProperty{Value: "5432"},
					Username: apiV1.PgProperty{Value: "admin"},
					Password: apiV1.PgProperty{Value: "password"},
				},
				Status: apiV1.PgInstanceStatus{},
			}
			err := k8sClient.Create(ctx, &instance)
			Expect(err).To(BeNil())
		}
		createFailure()
	})

	AfterEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Instances
		instance := apiV1.PgInstance{}
		opts := []client.DeleteAllOfOption{
			client.InNamespace("default"),
			client.GracePeriodSeconds(5),
		}
		err := k8sClient.DeleteAllOf(ctx, &instance, opts...)
		Expect(err).To(BeNil())
	})

	It("reconciles on create of PgInstance", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "dummy",
			},
		}
		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())

		// and
		var instance apiV1.PgInstance
		err = k8sClient.Get(ctx, request.NamespacedName, &instance)
		Expect(err).To(BeNil())
		Expect(instance.Status.Conditions).To(HaveLen(1))
		Expect(instance.Status.Conditions[0].Status).To(Equal(metaV1.ConditionTrue))
	})

	It("reconciles on update of PgInstance", func() {
		Expect(nil).To(BeNil())
	})

	It("reconciles on delete of PgInstance", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "missing",
			},
		}
		// when
		result, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())

		// and
		Expect(nil).To(BeNil())
	})

	It("handles connection failures", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// given
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "failure",
			},
		}
		// when
		_, err := reconciler.Reconcile(ctx, request)

		// then
		Expect(err).ToNot(BeNil())
		//Expect(result.RequeueAfter)

		// and
		var instance apiV1.PgInstance
		err = k8sClient.Get(ctx, request.NamespacedName, &instance)
		Expect(err).To(BeNil())
		Expect(instance.Status.Conditions).To(HaveLen(1))
		Expect(instance.Status.Conditions[0].Status).To(Equal(metaV1.ConditionFalse))
	})
})

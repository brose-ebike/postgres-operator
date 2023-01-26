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
	"testing"

	tc "github.com/brose-ebike/postgres-operator/pkg/tcpostgres"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var container *tc.PostgresContainer
var suiteCancel func()

func TestPgServerApi(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Services Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithCancel(context.Background())
	suiteCancel = cancel
	// Setup logger
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	// Setup container
	pgContainer, err := tc.SetupPostgres(ctx, tc.WithInitialDatabase("pgtest", "pgtest", "postgres"))
	Expect(err).To(BeNil())
	// Update Suite
	container = pgContainer
})

var _ = AfterSuite(func() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer suiteCancel()
	// Exit if no container exists
	if container == nil {
		return
	}
	// Cleanup container
	err := container.Terminate(ctx)
	Expect(err).To(BeNil())
})

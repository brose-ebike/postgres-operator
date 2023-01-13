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
package pgapi

import (
	"context"
	"testing"

	tc "github.com/brose-ebike/postgres-controller/pkg/tcpostgres"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var container *tc.PostgresContainer
var pgApi PgInstanceAPI
var suiteCancel func()

func TestPgServerApi(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "PgServerApi Suite")
}

func ConnectionStringFromContainer(ctx context.Context, pgc *tc.PostgresContainer) (*PgConnectionString, error) {
	// Get Hostname
	hostname, err := pgc.Hostname(ctx)
	if err != nil {
		return nil, err
	}

	// Get Mapped Port
	containerPort, err := pgc.Port(ctx)
	if err != nil {
		return nil, err
	}

	// Create new Connection String
	return NewPgConnectionString(
		hostname,
		containerPort,
		pgc.Username(),
		pgc.Password(),
		pgc.Database(),
		"disable",
	)
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
	// Generate Connection String
	connectionString, err := ConnectionStringFromContainer(ctx, container)
	Expect(err).To(BeNil())
	// Generate PgServerApi Object
	pgApi, err = NewPgInstanceAPI(ctx, "test", connectionString)
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer suiteCancel()
	if pgApi != nil && pgApi.IsConnected() {
		pgApi.(*pgInstanceAPIImpl).disconnect()
	}
	// Exit if no container exists
	if container == nil {
		return
	}
	// Cleanup container
	err := container.Terminate(ctx)
	Expect(err).To(BeNil())
})

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

package pgapi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PostgresAPI Connection Handling", func() {

	It("can establishes a connection to the postgres database", func() {
		err := pgApi.TestConnection()
		Expect(err).To(BeNil())
	})

	It("connect opens the connection pool", func() {
		// Test Server Connection
		err := pgApi.(*pgInstanceAPIImpl).connect()
		Expect(err).To(BeNil())
		Expect(pgApi.IsConnected()).To(BeTrue())
	})

	It("disconnect closes the connection pool", func() {
		// Create Connection
		err := pgApi.(*pgInstanceAPIImpl).connect()
		Expect(err).To(BeNil())
		Expect(pgApi.IsConnected()).To(BeTrue())
		// Close Server Connection
		err = pgApi.(*pgInstanceAPIImpl).disconnect()
		Expect(err).To(BeNil())
		Expect(pgApi.IsConnected()).To(BeFalse())
		// Create Connection
		err = pgApi.(*pgInstanceAPIImpl).connect()
		Expect(err).To(BeNil())

	})

	It("reuses the same connection pool for the same database", func() {
		impl := pgApi.(*pgInstanceAPIImpl)
		databaseName := "dummy_db_pool_reuse"

		err := pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())

		// impl.databases is shared with the rest of the suite, so assert on
		// the change in size rather than an absolute count.
		impl.databasesMu.Lock()
		initialCount := len(impl.databases)
		impl.databasesMu.Unlock()

		db1, err := impl.databaseConn(databaseName)
		Expect(err).To(BeNil())

		impl.databasesMu.Lock()
		afterFirstCount := len(impl.databases)
		impl.databasesMu.Unlock()
		Expect(afterFirstCount).To(Equal(initialCount + 1))

		db2, err := impl.databaseConn(databaseName)
		Expect(err).To(BeNil())

		impl.databasesMu.Lock()
		afterSecondCount := len(impl.databases)
		impl.databasesMu.Unlock()
		Expect(afterSecondCount).To(Equal(afterFirstCount))

		// Same *sql.DB instance both times - a genuinely reused pool, not
		// just a coincidentally equal count.
		Expect(db2).To(BeIdenticalTo(db1))
	})

	It("disconnect drains and closes the per-database connection pool cache", func() {
		impl := pgApi.(*pgInstanceAPIImpl)
		databaseName := "dummy_db_pool_drain"

		err := pgApi.CreateDatabase(databaseName)
		Expect(err).To(BeNil())

		// Populate the per-database pool cache
		_, err = impl.databaseConn(databaseName)
		Expect(err).To(BeNil())

		impl.databasesMu.Lock()
		cachedDb, ok := impl.databases[databaseName]
		impl.databasesMu.Unlock()
		Expect(ok).To(BeTrue())

		// Disconnect should drain and close every cached pool
		err = impl.disconnect()
		Expect(err).To(BeNil())

		impl.databasesMu.Lock()
		_, stillCached := impl.databases[databaseName]
		remainingCount := len(impl.databases)
		impl.databasesMu.Unlock()
		Expect(stillCached).To(BeFalse())
		Expect(remainingCount).To(Equal(0))

		// The cached pool should actually be closed, not just removed from
		// the map - a closed *sql.DB rejects further use.
		pingErr := cachedDb.Ping()
		Expect(pingErr).ToNot(BeNil())

		// Reconnect so the shared pgApi instance remains usable by later
		// tests, matching the same pattern the existing disconnect test uses.
		err = impl.connect()
		Expect(err).To(BeNil())
	})

	It("connection string returns the current connection string", func() {
		// Test Server Connection
		cs := pgApi.ConnectionString()
		Expect(cs.database).To(Equal(container.Database()))
	})
})

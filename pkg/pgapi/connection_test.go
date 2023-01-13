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

	It("connection string returns the current connection string", func() {
		// Test Server Connection
		cs := pgApi.ConnectionString()
		Expect(cs.database).To(Equal(container.Database()))
	})
})

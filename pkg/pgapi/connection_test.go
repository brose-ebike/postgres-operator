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

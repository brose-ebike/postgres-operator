package pgapi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PostgresAPI Login Role Handling", func() {

	It("can create new login role", func() {
		// Create new role
		err := pgApi.CreateLoginRole("dummy_role_0")
		Expect(err).To(BeNil())
		// Check if role exists
		exists, err := pgApi.IsLoginRoleExisting("dummy_role_0")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	It("can delete login role", func() {
		// Create new role
		err := pgApi.CreateLoginRole("dummy_role_1")
		Expect(err).To(BeNil())
		// Check if role exists
		exists, err := pgApi.IsLoginRoleExisting("dummy_role_1")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		// Delete role
		err = pgApi.DeleteLoginRole("dummy_role_1")
		Expect(err).To(BeNil())
	})

	It("can update role password", func() {
		// Create new role
		err := pgApi.CreateLoginRole("dummy_role_2")
		Expect(err).To(BeNil())
		// Check if role exists
		exists, err := pgApi.IsLoginRoleExisting("dummy_role_2")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		// Update Password
		err = pgApi.UpdateLoginRolePassword("dummy_role_2", "super-secret-password")
		Expect(err).To(BeNil())
	})

})

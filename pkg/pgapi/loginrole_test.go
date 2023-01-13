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

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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PgUserDatabase", func() {

	It("gets serialized with owner is nil", func() {
		// given:
		instanceSpec := PgUserDatabase{
			Name:       "mydb",
			Owner:      nil,
			Privileges: []string{"CONNECT"},
		}
		// when:
		data, err := json.Marshal(instanceSpec)
		textual := string(data)

		// then:
		Expect(err).To(BeNil())
		Expect(textual).To(Equal("{\"name\":\"mydb\",\"privileges\":[\"CONNECT\"]}"))
	})

	It("gets serialized with owner is true", func() {
		// given:
		cTrue := true
		instanceSpec := PgUserDatabase{
			Name:       "mydb",
			Owner:      &cTrue,
			Privileges: []string{"CONNECT"},
		}
		// when:
		data, err := json.Marshal(instanceSpec)
		textual := string(data)

		// then:
		Expect(err).To(BeNil())
		Expect(textual).To(Equal("{\"name\":\"mydb\",\"owner\":true,\"privileges\":[\"CONNECT\"]}"))
	})

	It("gets serialized with owner is false", func() {
		// given:
		cFalse := false
		instanceSpec := PgUserDatabase{
			Name:       "mydb",
			Owner:      &cFalse,
			Privileges: []string{"CONNECT"},
		}
		// when:
		data, err := json.Marshal(instanceSpec)
		textual := string(data)

		// then:
		Expect(err).To(BeNil())
		Expect(textual).To(Equal("{\"name\":\"mydb\",\"owner\":false,\"privileges\":[\"CONNECT\"]}"))
	})

	It("IsOwner returns correct value", func() {
		// given:
		cFalse := false
		cTrue := true
		instanceSpec0 := PgUserDatabase{
			Name:       "mydb",
			Owner:      nil,
			Privileges: []string{"CONNECT"},
		}
		instanceSpec1 := PgUserDatabase{
			Name:       "mydb",
			Owner:      &cFalse,
			Privileges: []string{"CONNECT"},
		}
		instanceSpec2 := PgUserDatabase{
			Name:       "mydb",
			Owner:      &cTrue,
			Privileges: []string{"CONNECT"},
		}
		// then:
		Expect(instanceSpec0.IsOwner()).To(BeFalse())
		Expect(instanceSpec1.IsOwner()).To(BeFalse())
		Expect(instanceSpec2.IsOwner()).To(BeTrue())
	})
})

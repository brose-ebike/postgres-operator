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

package security

import "testing"

func TestGeneratePasswordLength(t *testing.T) {
	password := GeneratePassword()
	if len(password) < 24 {
		t.Errorf("Password is too short, got: '%s'", password)

	}
	if len(password) > 32 {
		t.Errorf("Password is too long, got: '%s'", password)
	}
}

func TestGeneratePasswordRandomness(t *testing.T) {
	password0 := GeneratePassword()
	password1 := GeneratePassword()
	if password0 == password1 {
		t.Errorf("Two Passwords are equals, password0: '%s', password1: '%s'", password0, password1)
	}
}

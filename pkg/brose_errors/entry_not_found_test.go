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

package brose_errors

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMapEntryNotFoundErrorMessage(t *testing.T) {
	err := NewMapEntryNotFoundError("test", nil)
	actual := err.Error()
	expected := "Entry for key 'test' was not found in map"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Message is incorrect (-want +got):\n%s", diff)
	}
}

func TestMapEntryNotFoundErrorUnwrap(t *testing.T) {
	// given
	inner := errors.New("to-be-wrapped")
	err := NewMapEntryNotFoundError("test", inner)
	// when
	actual := err.Unwrap().Error()
	// then
	expected := "to-be-wrapped"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Message is incorrect (-want +got):\n%s", diff)
	}
}

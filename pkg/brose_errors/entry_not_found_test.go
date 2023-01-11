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

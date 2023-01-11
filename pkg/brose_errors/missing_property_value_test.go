package brose_errors

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMissingPropertyValueErrorMessage(t *testing.T) {
	err := NewMissingPropertyValueError("test", nil)
	actual := err.Error()
	expected := "The property 'test' has no value"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Message is incorrect (-want +got):\n%s", diff)
	}
}

func TestMissingPropertyValueErrorUnwrap(t *testing.T) {
	// given
	inner := errors.New("to-be-wrapped")
	err := NewMissingPropertyValueError("test", inner)
	// when
	actual := err.Unwrap().Error()
	// then
	expected := "to-be-wrapped"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Message is incorrect (-want +got):\n%s", diff)
	}
}

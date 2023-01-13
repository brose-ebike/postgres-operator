package brose_errors

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIllegalArgumentErrorMessage(t *testing.T) {
	err := NewIllegalArgumentError("test", "dummy", nil)
	actual := err.Error()
	expected := "The argument 'test' has an illegal value of 'dummy'"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Message is incorrect (-want +got):\n%s", diff)
	}
}

func TestIllegalArgumentErrorUnwrap(t *testing.T) {
	// given
	inner := errors.New("to-be-wrapped")
	err := NewIllegalArgumentError("test", "dummy", inner)
	// when
	actual := err.Unwrap().Error()
	// then
	expected := "to-be-wrapped"
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Message is incorrect (-want +got):\n%s", diff)
	}
}

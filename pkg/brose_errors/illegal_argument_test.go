package brose_errors

import "testing"

func TestNewIllegalArgumentError(t *testing.T) {
	err := NewIllegalArgumentError("custom message")
	if err == nil {
		t.Errorf("Error is not allowed to be nil")
	}
}

func TestIllegalArgumentError(t *testing.T) {
	err := NewIllegalArgumentError("test")
	actual := err.Error()
	expected := "test"
	if actual != expected {
		t.Errorf("Message is incorrect, got: '%s' want: '%s'", actual, expected)
	}
}

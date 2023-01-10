package brose_errors

import "testing"

func TestNewMissingPropertyValueError(t *testing.T) {
	err := NewMissingPropertyValueError("test")
	if err == nil {
		t.Errorf("Error is not allowed to be nil")
	}
}

func TestMissingPropertyValueError(t *testing.T) {
	err := NewMissingPropertyValueError("test")
	actual := err.Error()
	expected := "the value for the property 'test' is missing"
	if actual != expected {
		t.Errorf("Message is incorrect, got: '%s' want: '%s'", actual, expected)
	}
}

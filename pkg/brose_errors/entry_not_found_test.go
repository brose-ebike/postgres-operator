package brose_errors

import "testing"

func TestNewMapEntryNotFoundError(t *testing.T) {
	err := NewMapEntryNotFoundError("test")
	if err == nil {
		t.Errorf("Error is not allowed to be nil")
	}
}

func TestMapEntryNotFoundErrorMessage(t *testing.T) {
	err := NewMapEntryNotFoundError("test")
	actual := err.Error()
	expected := "Entry for key 'test' was not found in map"
	if actual != expected {
		t.Errorf("Message is incorrect, got: '%s' want: '%s'", actual, expected)
	}
}

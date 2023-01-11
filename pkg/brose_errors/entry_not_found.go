package brose_errors

import "errors"

type MapEntryNotFoundError struct {
	msg string
	err error
}

// NewMapEntryNotFoundError creates a new MapEntryNotFound error
func NewMapEntryNotFoundError(key string, err error) *MapEntryNotFoundError {
	merr := MapEntryNotFoundError{msg: "Entry for key '" + key + "' was not found in map", err: err}
	if err == nil {
		merr.err = errors.New("Entry for key '" + key + "' was not found in map")
	}
	return &merr
}

func (e *MapEntryNotFoundError) Error() string { return e.msg }
func (e *MapEntryNotFoundError) Unwrap() error { return e.err }

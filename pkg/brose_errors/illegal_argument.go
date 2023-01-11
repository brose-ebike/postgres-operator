package brose_errors

import (
	"errors"
	"fmt"
)

type IllegalArgumentError struct {
	msg      string
	argument string
	value    any
	err      error
}

func NewIllegalArgumentError(argument string, value any, err error) *IllegalArgumentError {
	msg := fmt.Sprintf("The argument '%s' has an illegal value of '%v'", argument, value)
	iar := IllegalArgumentError{
		msg:      msg,
		argument: argument,
		value:    value,
		err:      err,
	}
	if iar.err == nil {
		iar.err = errors.New(msg)
	}
	return &iar
}

func (e *IllegalArgumentError) Error() string { return e.msg }
func (e *IllegalArgumentError) Unwrap() error { return e.err }

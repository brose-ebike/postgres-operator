package brose_errors

import "errors"

type MissingPropertyValueError struct {
	msg      string
	property string
	err      error
}

func NewMissingPropertyValueError(property string, err error) *MissingPropertyValueError {
	msg := "The property '" + property + "' has no value"
	mpv := MissingPropertyValueError{
		msg,
		property,
		err,
	}
	if mpv.err == nil {
		mpv.err = errors.New(msg)
	}
	return &mpv
}

func (e *MissingPropertyValueError) Error() string { return e.msg }
func (e *MissingPropertyValueError) Unwrap() error { return e.err }

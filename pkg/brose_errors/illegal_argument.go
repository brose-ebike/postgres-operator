package brose_errors

type IllegalArgumentError struct {
	Message string
}

func NewIllegalArgumentError(msg string) *IllegalArgumentError {
	return &IllegalArgumentError{msg}
}

func (e *IllegalArgumentError) Error() string {
	return e.Message
}

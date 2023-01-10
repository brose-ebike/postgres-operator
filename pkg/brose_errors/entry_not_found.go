package brose_errors

type MapEntryNotFoundError struct {
	msg string
}

func NewMapEntryNotFoundError(key string) *MapEntryNotFoundError {
	return &MapEntryNotFoundError{
		"Entry for key '" + key + "' was not found in map",
	}
}

func (e *MapEntryNotFoundError) Error() string {
	return e.msg
}

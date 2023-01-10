package brose_errors

type MissingPropertyValueError struct {
	property string
}

func NewMissingPropertyValueError(property string) *MissingPropertyValueError {
	return &MissingPropertyValueError{
		property,
	}
}

func (e *MissingPropertyValueError) Error() string {
	return "the value for the property '" + e.property + "' is missing"
}

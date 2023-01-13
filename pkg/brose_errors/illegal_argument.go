/*
Copyright 2023 Brose Fahrzeugteile SE & Co. KG, Bamberg.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

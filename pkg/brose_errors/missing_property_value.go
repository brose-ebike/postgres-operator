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

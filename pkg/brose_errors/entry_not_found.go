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

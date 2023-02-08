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

package pgapi

import "strings"

type SqlExecutionError struct {
	msg   string
	query string
	args  []string
	err   error
}

func WrapSqlExecutionError(err error, query string, args ...string) error {
	if err == nil {
		return nil
	}
	return &SqlExecutionError{
		msg:   "Unable to execute query '" + query + "' with arguments '" + strings.Join(args, "','") + "'\n" + err.Error(),
		query: query,
		args:  args,
		err:   err,
	}
}

func (e *SqlExecutionError) Error() string { return e.msg }
func (e *SqlExecutionError) Unwrap() error { return e.err }

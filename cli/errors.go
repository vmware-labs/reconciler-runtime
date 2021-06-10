/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

var SilentError = &silentError{}

type silentError struct {
	err error
}

func (e *silentError) Error() string {
	return e.err.Error()
}

func (e *silentError) Unwrap() error {
	return e.err
}

func (e *silentError) Is(err error) bool {
	_, ok := err.(*silentError)
	return ok
}

func SilenceError(err error) error {
	return &silentError{err: err}
}

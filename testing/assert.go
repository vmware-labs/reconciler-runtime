/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"fmt"
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
)

// Deprecated
func AssertErrorEqual(expected error) VerifyFunc {
	return func(t *testing.T, result controllerruntime.Result, err error) {
		if err != expected {
			t.Errorf("Unexpected error: expected %v, actual %v", expected, err)
		}
	}
}

// Deprecated
func AssertErrorMessagef(message string, a ...interface{}) VerifyFunc {
	return func(t *testing.T, result controllerruntime.Result, err error) {
		expected := fmt.Sprintf(message, a...)
		actual := ""
		if err != nil {
			actual = err.Error()
		}
		if actual != expected {
			t.Errorf("Unexpected error message: expected %v, actual %v", expected, actual)
		}
	}
}

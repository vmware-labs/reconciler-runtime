/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package validation_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	rtesting "github.com/vmware-labs/reconciler-runtime/cli/testing"
	"github.com/vmware-labs/reconciler-runtime/cli/validation"
)

func TestPort(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		value    string
	}{{
		name:     "nan",
		expected: validation.ErrInvalidValue("http", rtesting.TestField),
		value:    "http",
	}, {
		name:     "valid",
		expected: validation.FieldErrors{},
		value:    "8888",
	}, {
		name:     "low",
		expected: validation.FieldErrors{},
		value:    "1",
	}, {
		name:     "high",
		expected: validation.FieldErrors{},
		value:    "65535",
	}, {
		name:     "zero",
		expected: validation.ErrInvalidValue("0", rtesting.TestField),
		value:    "0",
	}, {
		name:     "too low",
		expected: validation.ErrInvalidValue("-1", rtesting.TestField),
		value:    "-1",
	}, {
		name:     "too high",
		expected: validation.ErrInvalidValue("65536", rtesting.TestField),
		value:    "65536",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.Port(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestPortNumber(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		value    int32
	}{{
		name:     "valid",
		expected: validation.FieldErrors{},
		value:    8888,
	}, {
		name:     "low",
		expected: validation.FieldErrors{},
		value:    1,
	}, {
		name:     "high",
		expected: validation.FieldErrors{},
		value:    65535,
	}, {
		name:     "zero",
		expected: validation.ErrInvalidValue("0", rtesting.TestField),
		value:    0,
	}, {
		name:     "too low",
		expected: validation.ErrInvalidValue("-1", rtesting.TestField),
		value:    -1,
	}, {
		name:     "too high",
		expected: validation.ErrInvalidValue("65536", rtesting.TestField),
		value:    65536,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.PortNumber(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

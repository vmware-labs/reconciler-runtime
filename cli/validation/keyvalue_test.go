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

func TestKeyValue(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		value    string
	}{{
		name:     "valid",
		expected: validation.FieldErrors{},
		value:    "MY_VAR=my-value",
	}, {
		name:     "empty",
		expected: validation.ErrInvalidValue("", rtesting.TestField),
		value:    "",
	}, {
		name:     "missing name",
		expected: validation.ErrInvalidValue("=my-value", rtesting.TestField),
		value:    "=my-value",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.KeyValue(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestKeyValues(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		values   []string
	}{{
		name:     "valid, empty",
		expected: validation.FieldErrors{},
		values:   []string{},
	}, {
		name:     "valid, not empty",
		expected: validation.FieldErrors{},
		values:   []string{"MY_VAR=my-value"},
	}, {
		name:     "invalid",
		expected: validation.ErrInvalidValue("", validation.CurrentField).ViaFieldIndex(rtesting.TestField, 0),
		values:   []string{""},
	}, {
		name: "multiple invalid",
		expected: validation.FieldErrors{}.Also(
			validation.ErrInvalidValue("", validation.CurrentField).ViaFieldIndex(rtesting.TestField, 0),
			validation.ErrInvalidValue("", validation.CurrentField).ViaFieldIndex(rtesting.TestField, 1),
		),
		values: []string{"", ""},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.KeyValues(test.values, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

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

func TestEnvVar(t *testing.T) {
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
			actual := validation.EnvVar(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestEnvVars(t *testing.T) {
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
			actual := validation.EnvVars(test.values, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestEnvVarFrom(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		value    string
	}{{
		name:     "valid configmap",
		expected: validation.FieldErrors{},
		value:    "MY_VAR=configMapKeyRef:my-configmap:my-key",
	}, {
		name:     "valid secret",
		expected: validation.FieldErrors{},
		value:    "MY_VAR=secretKeyRef:my-secret:my-key",
	}, {
		name:     "empty",
		expected: validation.ErrInvalidValue("", rtesting.TestField),
		value:    "",
	}, {
		name:     "missing name",
		expected: validation.ErrInvalidValue("=configMapKeyRef:my-configmap:my-key", rtesting.TestField),
		value:    "=configMapKeyRef:my-configmap:my-key",
	}, {
		name:     "unknown type",
		expected: validation.ErrInvalidValue("MY_VAR=otherKeyRef:my-other:my-key", rtesting.TestField),
		value:    "MY_VAR=otherKeyRef:my-other:my-key",
	}, {
		name:     "missing resource",
		expected: validation.ErrInvalidValue("MY_VAR=configMapKeyRef::my-key", rtesting.TestField),
		value:    "MY_VAR=configMapKeyRef::my-key",
	}, {
		name:     "missing key",
		expected: validation.ErrInvalidValue("MY_VAR=configMapKeyRef:my-configmap", rtesting.TestField),
		value:    "MY_VAR=configMapKeyRef:my-configmap",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.EnvVarFrom(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestEnvVarFroms(t *testing.T) {
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
		values:   []string{"MY_VAR=configMapKeyRef:my-configmap:my-key"},
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
			actual := validation.EnvVarFroms(test.values, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

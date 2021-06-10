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

func TestK8sName(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		value    string
	}{{
		name:     "valid",
		expected: validation.FieldErrors{},
		value:    "my-resource",
	}, {
		name:     "empty",
		expected: validation.ErrInvalidValue("", rtesting.TestField),
		value:    "",
	}, {
		name:     "invalid",
		expected: validation.ErrInvalidValue("/", rtesting.TestField),
		value:    "/",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.K8sName(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestK8sNames(t *testing.T) {
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
		values:   []string{"my-resource"},
	}, {
		name:     "invalid",
		expected: validation.ErrInvalidValue("", validation.CurrentField).ViaFieldIndex(rtesting.TestField, 0),
		values:   []string{""},
	}, {
		name:     "invalid",
		expected: validation.ErrInvalidValue("/", validation.CurrentField).ViaFieldIndex(rtesting.TestField, 0),
		values:   []string{"/"},
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
			actual := validation.K8sNames(test.values, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

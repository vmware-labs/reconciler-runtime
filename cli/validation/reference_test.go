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

func TestObjectReference(t *testing.T) {
	tests := []struct {
		name     string
		expected validation.FieldErrors
		value    string
	}{{
		name:     "valid",
		expected: validation.FieldErrors{},
		value:    "example.com/v1alpha1:FooBar:blah",
	}, {
		name:     "valid core/v1",
		expected: validation.FieldErrors{},
		value:    "v1:ConfigMap:blah",
	}, {
		name:     "empty",
		expected: validation.ErrInvalidValue("", rtesting.TestField),
		value:    "",
	}, {
		name:     "invalid",
		expected: validation.ErrInvalidValue("/", rtesting.TestField),
		value:    "/",
	}, {
		name:     "missing api version",
		expected: validation.ErrInvalidValue(":FooBar:blah", rtesting.TestField),
		value:    ":FooBar:blah",
	}, {
		name:     "missing api group",
		expected: validation.ErrInvalidValue("v1alpha1:FooBar:blah", rtesting.TestField),
		value:    "v1alpha1:FooBar:blah",
	}, {
		name:     "missing kind",
		expected: validation.ErrInvalidValue("example.com/v1alpha1::blah", rtesting.TestField),
		value:    "example.com/v1alpha1::blah",
	}, {
		name:     "invalid name",
		expected: validation.ErrInvalidValue("-", rtesting.TestField),
		value:    "v1:ConfigMap:-",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := validation.ObjectReference(test.value, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestObjectReferences(t *testing.T) {
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
		values:   []string{"example.com/v1alpha1:FooBar:blah"},
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
			actual := validation.ObjectReferences(test.values, rtesting.TestField)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

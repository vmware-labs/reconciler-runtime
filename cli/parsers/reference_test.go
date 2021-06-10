/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parsers_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/cli/parsers"
	corev1 "k8s.io/api/core/v1"
)

func TestObjectReference(t *testing.T) {
	tests := []struct {
		name     string
		expected corev1.ObjectReference
		value    string
	}{{
		name:  "valid",
		value: "example.com/v1alpha1:FooBar:blah",
		expected: corev1.ObjectReference{
			APIVersion: "example.com/v1alpha1",
			Kind:       "FooBar",
			Name:       "blah",
		},
	}, {
		name:  "valid core/v1",
		value: "v1:ConfigMap:blah",
		expected: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Name:       "blah",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := parsers.ObjectReference(test.value)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

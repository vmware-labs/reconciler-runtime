/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parsers_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/cli/parsers"
)

func TestKeyValue(t *testing.T) {
	tests := []struct {
		name     string
		expected []string
		value    string
	}{{
		name:     "valid",
		value:    "MY_VAR=my-value",
		expected: []string{"MY_VAR", "my-value"},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := parsers.KeyValue(test.value)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

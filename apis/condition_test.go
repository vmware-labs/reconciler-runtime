/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package apis

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestConditionStatus(t *testing.T) {
	tests := []struct {
		name            string
		condition       *Condition
		expectedTrue    bool
		expectedFalse   bool
		expectedUnknown bool
	}{
		{
			name:            "true",
			condition:       &Condition{Status: corev1.ConditionTrue},
			expectedTrue:    true,
			expectedFalse:   false,
			expectedUnknown: false,
		},
		{
			name:            "false",
			condition:       &Condition{Status: corev1.ConditionFalse},
			expectedTrue:    false,
			expectedFalse:   true,
			expectedUnknown: false,
		},
		{
			name:            "unknown",
			condition:       &Condition{Status: corev1.ConditionUnknown},
			expectedTrue:    false,
			expectedFalse:   false,
			expectedUnknown: true,
		},
		{
			name:            "unset",
			condition:       &Condition{},
			expectedTrue:    false,
			expectedFalse:   false,
			expectedUnknown: false,
		},
		{
			name:            "nil",
			condition:       nil,
			expectedTrue:    false,
			expectedFalse:   false,
			expectedUnknown: true,
		},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if expected, actual := c.expectedTrue, c.condition.IsTrue(); expected != actual {
				t.Errorf("%s: IsTrue() actually = %v, expected %v", c.name, actual, expected)
			}
			if expected, actual := c.expectedFalse, c.condition.IsFalse(); expected != actual {
				t.Errorf("%s: IsFalse() actually = %v, expected %v", c.name, actual, expected)
			}
			if expected, actual := c.expectedUnknown, c.condition.IsUnknown(); expected != actual {
				t.Errorf("%s: IsUnknown() actually = %v, expected %v", c.name, actual, expected)
			}
		})
	}
}

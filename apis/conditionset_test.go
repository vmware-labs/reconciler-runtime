/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package apis

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionStatus(t *testing.T) {
	tests := []struct {
		name            string
		condition       *metav1.Condition
		expectedTrue    bool
		expectedFalse   bool
		expectedUnknown bool
	}{
		{
			name:            "true",
			condition:       &metav1.Condition{Status: metav1.ConditionTrue},
			expectedTrue:    true,
			expectedFalse:   false,
			expectedUnknown: false,
		},
		{
			name:            "false",
			condition:       &metav1.Condition{Status: metav1.ConditionFalse},
			expectedTrue:    false,
			expectedFalse:   true,
			expectedUnknown: false,
		},
		{
			name:            "unknown",
			condition:       &metav1.Condition{Status: metav1.ConditionUnknown},
			expectedTrue:    false,
			expectedFalse:   false,
			expectedUnknown: true,
		},
		{
			name:            "unset",
			condition:       &metav1.Condition{},
			expectedTrue:    false,
			expectedFalse:   false,
			expectedUnknown: true,
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
			if expected, actual := c.expectedTrue, ConditionIsTrue(c.condition); expected != actual {
				t.Errorf("%s: IsTrue() actually = %v, expected %v", c.name, actual, expected)
			}
			if expected, actual := c.expectedFalse, ConditionIsFalse(c.condition); expected != actual {
				t.Errorf("%s: IsFalse() actually = %v, expected %v", c.name, actual, expected)
			}
			if expected, actual := c.expectedUnknown, ConditionIsUnknown(c.condition); expected != actual {
				t.Errorf("%s: IsUnknown() actually = %v, expected %v", c.name, actual, expected)
			}
		})
	}
}

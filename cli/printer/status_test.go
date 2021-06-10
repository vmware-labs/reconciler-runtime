/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/cli/printer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceStatus(t *testing.T) {
	tests := []struct {
		name      string
		condition *metav1.Condition
		output    string
	}{{
		name: "nil",
		output: `
# test: <unknown>
`,
	}, {
		name:      "empty",
		condition: &metav1.Condition{},
		output: `
# test: <unknown>
---
lastTransitionTime: null
message: ""
reason: ""
status: ""
type: ""
`,
	}, {
		name: "unknown",
		condition: &metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionUnknown,
			Reason:  "HangOn",
			Message: "a hopefully informative message about what's in flight",
			LastTransitionTime: metav1.Time{
				Time: time.Date(2019, 6, 29, 01, 44, 05, 0, time.UTC),
			},
		},
		output: `
# test: Unknown
---
lastTransitionTime: "2019-06-29T01:44:05Z"
message: a hopefully informative message about what's in flight
reason: HangOn
status: Unknown
type: Ready
`,
	}, {
		name: "ready",
		condition: &metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{
				Time: time.Date(2019, 6, 29, 01, 44, 05, 0, time.UTC),
			},
		},
		output: `
# test: Ready
---
lastTransitionTime: "2019-06-29T01:44:05Z"
message: ""
reason: ""
status: "True"
type: Ready
`,
	}, {
		name: "failure",
		condition: &metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "OopsieDoodle",
			Message: "a hopefully informative message about what went wrong",
			LastTransitionTime: metav1.Time{
				Time: time.Date(2019, 6, 29, 01, 44, 05, 0, time.UTC),
			},
		},
		output: `
# test: OopsieDoodle
---
lastTransitionTime: "2019-06-29T01:44:05Z"
message: a hopefully informative message about what went wrong
reason: OopsieDoodle
status: "False"
type: Ready
`,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := printer.ResourceStatus("test", test.condition)
			expected, actual := strings.TrimSpace(test.output), strings.TrimSpace(output)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Unexpected output (-expected, +actual): %s", diff)
			}
		})
	}
}

func TestFindCondition(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		conditionType string
		output        *metav1.Condition
	}{{
		name:          "missing",
		conditions:    []metav1.Condition{},
		conditionType: "Ready",
		output:        nil,
	}, {
		name: "found",
		conditions: []metav1.Condition{
			{
				Type:   "NotReady",
				Status: metav1.ConditionTrue,
			},
			{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
			},
		},
		conditionType: "Ready",
		output: &metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := printer.FindCondition(test.conditions, test.conditionType)
			expected, actual := test.output, output
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Unexpected output (-expected, +actual): %s", diff)
			}
		})
	}
}

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer_test

import (
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/vmware-labs/reconciler-runtime/cli/printer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTimestampSince(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	now := time.Now()

	tests := []struct {
		name   string
		input  metav1.Time
		output string
	}{{
		name:   "empty",
		output: printer.Swarnf("<unknown>"),
	}, {
		name:   "now",
		input:  metav1.Time{Time: now},
		output: "0s",
	}, {
		name:   "1 minute ago",
		input:  metav1.Time{Time: now.Add(-1 * time.Minute)},
		output: "60s",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if expected, actual := test.output, printer.TimestampSince(test.input, now); expected != actual {
				t.Errorf("Expected formated string to be %q, actually %q", expected, actual)
			}
		})
	}
}

func TestEmptyString(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = noColor }()

	tests := []struct {
		name   string
		input  string
		output string
	}{{
		name:   "empty",
		output: printer.Sfaintf("<empty>"),
	}, {
		name:   "not empty",
		input:  "hello",
		output: "hello",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if expected, actual := test.output, printer.EmptyString(test.input); expected != actual {
				t.Errorf("Expected formated string to be %q, actually %q", expected, actual)
			}
		})
	}
}

func TestConditionStatus(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = noColor }()

	tests := []struct {
		name   string
		input  *metav1.Condition
		output string
	}{{
		name:   "empty",
		output: printer.Swarnf("<unknown>"),
	}, {
		name: "status true",
		input: &metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		},
		output: printer.Ssuccessf("Ready"),
	}, {
		name: "status false",
		input: &metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionFalse,
			Reason: "uh-oh",
		},
		output: printer.Serrorf("uh-oh"),
	}, {
		name: "status false, no reason",
		input: &metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionFalse,
		},
		output: printer.Serrorf("not-Ready"),
	}, {
		name: "status unknown",
		input: &metav1.Condition{
			Type:   "Ready",
			Status: metav1.ConditionUnknown,
		},
		output: printer.Sinfof("Unknown"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if expected, actual := test.output, printer.ConditionStatus(test.input); expected != actual {
				t.Errorf("Expected formated string to be %q, actually %q", expected, actual)
			}
		})
	}
}

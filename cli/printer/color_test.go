/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer_test

import (
	"testing"

	"github.com/fatih/color"
	"github.com/vmware-labs/reconciler-runtime/cli/printer"
)

func TestScolorf(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	tests := []struct {
		name    string
		format  string
		args    []interface{}
		printer func(format string, a ...interface{}) string
		output  string
	}{{
		name:    "Sfaintf",
		format:  "%s",
		args:    []interface{}{"hello"},
		printer: printer.Sfaintf,
		output:  printer.FaintColor.Sprint("hello"),
	}, {
		name:    "Sinfof",
		format:  "%s",
		args:    []interface{}{"hello"},
		printer: printer.Sinfof,
		output:  printer.InfoColor.Sprint("hello"),
	}, {
		name:    "Ssuccessf",
		format:  "%s",
		args:    []interface{}{"hello"},
		printer: printer.Ssuccessf,
		output:  printer.SuccessColor.Sprint("hello"),
	}, {
		name:    "Swarnf",
		format:  "%s",
		args:    []interface{}{"hello"},
		printer: printer.Swarnf,
		output:  printer.WarnColor.Sprint("hello"),
	}, {
		name:    "Serrorf",
		format:  "%s",
		args:    []interface{}{"hello"},
		printer: printer.Serrorf,
		output:  printer.ErrorColor.Sprint("hello"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if expected, actual := test.output, test.printer(test.format, test.args...); expected != actual {
				t.Errorf("Expected output to be %q, actually %q", expected, actual)
			}
		})
	}
}

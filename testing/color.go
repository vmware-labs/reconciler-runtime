/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"os"
	"strings"

	"github.com/fatih/color"
)

func init() {
	if _, ok := os.LookupEnv("COLOR_DIFF"); ok {
		DiffAddedColor.EnableColor()
		DiffRemovedColor.EnableColor()
	}
}

var (
	DiffAddedColor   = color.New(color.FgGreen)
	DiffRemovedColor = color.New(color.FgRed)
)

func ColorizeDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			b.WriteString(DiffAddedColor.Sprint(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(DiffRemovedColor.Sprint(line))
		default:
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

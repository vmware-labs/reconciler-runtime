/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"github.com/fatih/color"
)

var (
	FaintColor   = color.New(color.Faint)
	InfoColor    = color.New(color.FgCyan)
	SuccessColor = color.New(color.FgGreen)
	WarnColor    = color.New(color.FgYellow)
	ErrorColor   = color.New(color.FgRed)
)

func Sfaintf(format string, a ...interface{}) string {
	return FaintColor.Sprintf(format, a...)
}

func Sinfof(format string, a ...interface{}) string {
	return InfoColor.Sprintf(format, a...)
}

func Ssuccessf(format string, a ...interface{}) string {
	return SuccessColor.Sprintf(format, a...)
}

func Swarnf(format string, a ...interface{}) string {
	return WarnColor.Sprintf(format, a...)
}

func Serrorf(format string, a ...interface{}) string {
	return ErrorColor.Sprintf(format, a...)
}

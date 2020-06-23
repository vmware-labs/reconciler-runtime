/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	gotesting "testing"

	"github.com/go-logr/logr"
	logrtesting "github.com/go-logr/logr/testing"
)

// TestLogger gets a logger to use in unit and end to end tests
func TestLogger(t *gotesting.T) logr.Logger {
	return logrtesting.TestLogger{T: t}
}

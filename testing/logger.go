/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"testing"

	"github.com/go-logr/logr"
	logrtesting "github.com/go-logr/logr/testing"
)

// Deprecated TestLogger gets a logger to use in unit and end to end tests.
// Use https://pkg.go.dev/github.com/go-logr/logr@v1.2.2/testing#NewTestLogger instead
func TestLogger(t *testing.T) logr.Logger {
	return logrtesting.NewTestLogger(t)
}

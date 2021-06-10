/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/vmware-labs/reconciler-runtime/cli"
)

func TestSilenceError(t *testing.T) {
	err := fmt.Errorf("test error")
	silentErr := cli.SilenceError(err)

	if errors.Is(err, cli.SilentError) {
		t.Errorf("expected error to not be silent, got %#v", err)
	}
	if !errors.Is(silentErr, cli.SilentError) {
		t.Errorf("expected error to be silent, got %#v", err)
	}
	if expected, actual := err, errors.Unwrap(silentErr); expected != actual {
		t.Errorf("errors expected to match, expected %v, actually %v", expected, actual)
	}
	if expected, actual := err.Error(), silentErr.Error(); expected != actual {
		t.Errorf("errors expected to match, expected %q, actually %q", expected, actual)
	}
}

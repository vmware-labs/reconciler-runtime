/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
)

func TestDryRunResource(t *testing.T) {
	stdout := &bytes.Buffer{}
	ctx := withStdout(context.Background(), stdout)
	resource := &rtesting.TestResource{}

	DryRunResource(ctx, resource, rtesting.GroupVersion.WithKind("TestResource"))

	expected := strings.TrimSpace(`
---
apiVersion: testing.reconciler.runtime/v1
kind: TestResource
metadata:
  creationTimestamp: null
spec:
  template:
    metadata:
      creationTimestamp: null
    spec:
      containers: null
status: {}
`)
	actual := strings.TrimSpace(stdout.String())
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Unexpected stdout (-expected, +actual): %s", diff)
	}

}

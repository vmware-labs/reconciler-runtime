/*
Copyright 2024 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/internal/resources/dies"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
)

type TestResourceUnexportedSpec struct {
	spec resources.TestResourceUnexportedFieldsSpec
}

func TestIgnoreAllUnexported(t *testing.T) {
	tests := map[string]struct {
		a          interface{}
		b          interface{}
		shouldDiff bool
	}{
		"nil is equivalent": {
			a:          nil,
			b:          nil,
			shouldDiff: false,
		},
		"different exported fields have a difference": {
			a: dies.TestResourceUnexportedFieldsSpecBlank.
				AddField("name", "hello").
				DieRelease(),
			b: dies.TestResourceUnexportedFieldsSpecBlank.
				AddField("name", "world").
				DieRelease(),
			shouldDiff: true,
		},
		"different unexported fields do not have a difference": {
			a: dies.TestResourceUnexportedFieldsSpecBlank.
				AddUnexportedField("name", "hello").
				DieRelease(),
			b: dies.TestResourceUnexportedFieldsSpecBlank.
				AddUnexportedField("name", "world").
				DieRelease(),
			shouldDiff: false,
		},
		"different exported fields nested in an unexported field do not have a difference": {
			a: TestResourceUnexportedSpec{
				spec: dies.TestResourceUnexportedFieldsSpecBlank.
					AddField("name", "hello").
					DieRelease(),
			},
			b: TestResourceUnexportedSpec{
				spec: dies.TestResourceUnexportedFieldsSpecBlank.
					AddField("name", "world").
					DieRelease(),
			},
			shouldDiff: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if name[0:1] == "#" {
				t.SkipNow()
			}

			diff := cmp.Diff(tc.a, tc.b, reconcilers.IgnoreAllUnexported)
			hasDiff := diff != ""
			shouldDiff := tc.shouldDiff

			if !hasDiff && shouldDiff {
				t.Errorf("expected equality, found diff")
			}
			if hasDiff && !shouldDiff {
				t.Errorf("found diff, expected equality: %s", diff)
			}
		})
	}
}

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/vmware-labs/reconciler-runtime/validation"
)

type OptionsTestSuite []OptionsTestCase

type OptionsTestCase struct {
	// Name is used to identify the record in the test results. A sub-test is created for each
	// record with this name.
	Name string
	// Skip suppresses the execution of this test record.
	Skip bool
	// Focus executes this record skipping all unfocused records. The containing test will fail to
	// prevent accidental check-in.
	Focus bool

	// inputs

	// Options to validate
	Options validation.Validatable

	// outputs

	// ExpectFieldErrors are the errors that should be returned from the validator.
	ExpectFieldErrors validation.FieldErrors

	// ShouldValidate is true if the options are valid
	ShouldValidate bool
}

func (ts OptionsTestSuite) Run(t *testing.T) {
	focused := OptionsTestSuite{}
	for _, tc := range ts {
		if tc.Focus && !tc.Skip {
			focused = append(focused, tc)
		}
	}
	if len(focused) != 0 {
		for _, tc := range focused {
			tc.Run(t)
		}
		t.Errorf("test run focused on %d record(s), skipped %d record(s)", len(focused), len(ts)-len(focused))
		return
	}

	for _, tc := range ts {
		tc.Run(t)
	}
}

func (tc OptionsTestCase) Run(t *testing.T) {
	t.Run(tc.Name, func(t *testing.T) {
		if tc.Skip {
			t.SkipNow()
		}

		errs := tc.Options.Validate(context.Background())

		if tc.ExpectFieldErrors != nil {
			actual := errs
			expected := tc.ExpectFieldErrors
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Unexpected errors (-expected, +actual): %s", diff)
			}
		}

		if expected, actual := tc.ShouldValidate, len(errs) == 0; expected != actual {
			if expected {
				t.Errorf("expected options to validate, actual %q", errs)
			} else {
				t.Errorf("expected options not to validate, actual %q", errs)
			}
		}

		if !tc.ShouldValidate && len(tc.ExpectFieldErrors) == 0 {
			t.Error("one of ShouldValidate=true or ExpectFieldErrors is required")
		}
	})
}

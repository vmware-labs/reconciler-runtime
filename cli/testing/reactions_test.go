/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	clitesting "github.com/vmware-labs/reconciler-runtime/cli/testing"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
)

func TestValidateCreates(t *testing.T) {
	tests := []struct {
		name      string
		action    clientgotesting.Action
		handled   bool
		returned  runtime.Object
		shouldErr bool
	}{{
		name: "valid create",
		action: clientgotesting.NewCreateAction(
			rtesting.SchemeBuilder.GroupVersion.WithResource("TestResource"),
			"default",
			&rtesting.TestResource{},
		),
		handled:   false,
		shouldErr: false,
	}, {
		name: "invalid create",
		action: clientgotesting.NewCreateAction(
			rtesting.SchemeBuilder.GroupVersion.WithResource("TestResource"),
			"default",
			&rtesting.TestResource{
				Spec: rtesting.TestResourceSpec{
					Fields: map[string]string{
						"invalid": "true",
					},
				},
			},
		),
		handled:   true,
		shouldErr: true,
	}, {
		name: "not validatable",
		action: clientgotesting.NewCreateAction(
			rtesting.SchemeBuilder.GroupVersion.WithResource("TestResourceNoStatus"),
			"default",
			&rtesting.TestResourceNoStatus{},
		),
		handled:   false,
		shouldErr: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()
			handled, returned, err := clitesting.ValidateCreates(ctx, test.action)
			if expected, actual := test.handled, handled; expected != actual {
				t.Errorf("Expected handled %v, actually %v", expected, actual)
			}
			if diff := cmp.Diff(test.returned, returned); diff != "" {
				t.Errorf("Unexpected returned object (-expected, +actual): %s", diff)
			}
			if (err != nil) != test.shouldErr {
				t.Errorf("Expected error %v, actually %q", test.shouldErr, err)
			}
		})
	}
}

func TestValidateUpdates(t *testing.T) {
	tests := []struct {
		name      string
		action    clientgotesting.Action
		handled   bool
		returned  runtime.Object
		shouldErr bool
	}{{
		name: "valid create",
		action: clientgotesting.NewUpdateAction(
			rtesting.SchemeBuilder.GroupVersion.WithResource("TestResource"),
			"default",
			&rtesting.TestResource{},
		),
		handled:   false,
		shouldErr: false,
	}, {
		name: "invalid create",
		action: clientgotesting.NewUpdateAction(
			rtesting.SchemeBuilder.GroupVersion.WithResource("TestResource"),
			"default",
			&rtesting.TestResource{
				Spec: rtesting.TestResourceSpec{
					Fields: map[string]string{
						"invalid": "true",
					},
				},
			},
		),
		handled:   true,
		shouldErr: true,
	}, {
		name: "not validatable",
		action: clientgotesting.NewUpdateAction(
			rtesting.SchemeBuilder.GroupVersion.WithResource("TestResourceNoStatus"),
			"default",
			&rtesting.TestResourceNoStatus{},
		),
		handled:   false,
		shouldErr: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()
			handled, returned, err := clitesting.ValidateUpdates(ctx, test.action)
			if expected, actual := test.handled, handled; expected != actual {
				t.Errorf("Expected handled %v, actually %v", expected, actual)
			}
			if diff := cmp.Diff(test.returned, returned); diff != "" {
				t.Errorf("Unexpected returned object (-expected, +actual): %s", diff)
			}
			if (err != nil) != test.shouldErr {
				t.Errorf("Expected error %v, actually %q", test.shouldErr, err)
			}
		})
	}
}

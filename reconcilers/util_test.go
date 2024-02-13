/*
Copyright 2024 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	corev1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestExtractItems(t *testing.T) {
	tests := map[string]struct {
		list        client.ObjectList
		expected    []*resources.TestResource
		shouldPanic bool
	}{
		"empty": {
			list: &resources.TestResourceList{
				Items: []resources.TestResource{},
			},
			expected: []*resources.TestResource{},
		},
		"struct items": {
			list: &resources.TestResourceList{
				Items: []resources.TestResource{
					{
						ObjectMeta: corev1.ObjectMeta{
							Name: "obj1",
						},
					},
					{
						ObjectMeta: corev1.ObjectMeta{
							Name: "obj2",
						},
					},
				},
			},
			expected: []*resources.TestResource{
				{
					ObjectMeta: corev1.ObjectMeta{
						Name: "obj1",
					},
				},
				{
					ObjectMeta: corev1.ObjectMeta{
						Name: "obj2",
					},
				},
			},
		},
		"struct-pointer items": {
			list: &resources.TestResourcePointerList{
				Items: []*resources.TestResource{
					{
						ObjectMeta: corev1.ObjectMeta{
							Name: "obj1",
						},
					},
					{
						ObjectMeta: corev1.ObjectMeta{
							Name: "obj2",
						},
					},
				},
			},
			expected: []*resources.TestResource{
				{
					ObjectMeta: corev1.ObjectMeta{
						Name: "obj1",
					},
				},
				{
					ObjectMeta: corev1.ObjectMeta{
						Name: "obj2",
					},
				},
			},
		},
		"interface items": {
			list: &resources.TestResourceInterfaceList{
				Items: []client.Object{
					&resources.TestResource{
						ObjectMeta: corev1.ObjectMeta{
							Name: "obj1",
						},
					},
					&resources.TestResource{
						ObjectMeta: corev1.ObjectMeta{
							Name: "obj2",
						},
					},
				},
			},
			expected: []*resources.TestResource{
				{
					ObjectMeta: corev1.ObjectMeta{
						Name: "obj1",
					},
				},
				{
					ObjectMeta: corev1.ObjectMeta{
						Name: "obj2",
					},
				},
			},
		},
		"invalid items": {
			list: &resources.TestResourceInvalidList{
				Items: []string{
					"boom",
				},
			},
			shouldPanic: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tc.shouldPanic {
						t.Errorf("unexpected panic: %s", r)
					}
				}
			}()
			actual := extractItems[*resources.TestResource](tc.list)
			if tc.shouldPanic {
				t.Errorf("expected to panic")
			}
			expected := tc.expected
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("expected items to match actual items: %s", diff)
			}
		})
	}
}

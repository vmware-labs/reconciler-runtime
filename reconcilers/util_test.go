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
		list     client.ObjectList
		expected []*resources.TestResource
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := extractItems[*resources.TestResource](tc.list)
			expected := tc.expected
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("expected items to match actual items: %s", diff)
			}
		})
	}
}

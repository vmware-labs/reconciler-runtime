/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPatch(t *testing.T) {
	tests := []struct {
		name           string
		base           apis.Object
		update         apis.Object
		rebase         apis.Object
		expected       apis.Object
		newShouldErr   bool
		applyShouldErr bool
	}{
		{
			name:     "identity",
			base:     &corev1.Pod{},
			update:   &corev1.Pod{},
			rebase:   &corev1.Pod{},
			expected: &corev1.Pod{},
		},
		{
			name: "rebase",
			base: &corev1.Pod{},
			update: &corev1.Pod{
				Spec: corev1.PodSpec{
					DNSPolicy: corev1.DNSClusterFirst,
				},
			},
			rebase: &corev1.Pod{
				Spec: corev1.PodSpec{
					DNSConfig: &corev1.PodDNSConfig{
						Nameservers: []string{"1.1.1.1"},
					},
				},
			},
			expected: &corev1.Pod{
				Spec: corev1.PodSpec{
					DNSConfig: &corev1.PodDNSConfig{
						Nameservers: []string{"1.1.1.1"},
					},
					DNSPolicy: corev1.DNSClusterFirst,
				},
			},
		},
		{
			name:         "bad base",
			base:         &boom{ShouldErr: true},
			update:       &boom{},
			newShouldErr: true,
		},
		{
			name:         "bad update",
			base:         &boom{},
			update:       &boom{ShouldErr: true},
			newShouldErr: true,
		},
		{
			name:           "bad rebase",
			base:           &boom{},
			update:         &boom{},
			rebase:         &boom{ShouldErr: true},
			applyShouldErr: true,
		},
		{
			name:   "generation mismatch",
			base:   &corev1.Pod{},
			update: &corev1.Pod{},
			rebase: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
			},
			expected:       &corev1.Pod{},
			applyShouldErr: true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			patch, newErr := reconcilers.NewPatch(c.base, c.update)
			if actual, expected := newErr != nil, c.newShouldErr; actual != expected {
				t.Errorf("%s: unexpected new error, actually = %v, expected = %v", c.name, actual, expected)
			}
			if c.newShouldErr {
				return
			}

			applyErr := patch.Apply(c.rebase)
			if actual, expected := applyErr != nil, c.applyShouldErr; actual != expected {
				t.Errorf("%s: unexpected apply error, actually = %v, expected = %v", c.name, actual, expected)
			}
			if c.applyShouldErr {
				return
			}

			if diff := cmp.Diff(c.expected, c.rebase); diff != "" {
				t.Errorf("%s: unexpected value (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

type boom struct {
	metav1.ObjectMeta `json:"metadata"`
	ShouldErr         bool `json:"shouldErr"`
}

func (b *boom) MarshalJSON() ([]byte, error) {
	if b.ShouldErr {
		return nil, fmt.Errorf("object asked to err")
	}
	return json.Marshal(b.ObjectMeta)
}

func (b *boom) UnmarshalJSON(data []byte) error {
	if b.ShouldErr {
		return fmt.Errorf("object asked to err")
	}
	return json.Unmarshal(data, b.ObjectMeta)
}

func (b *boom) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (b *boom) DeepCopyObject() runtime.Object {
	return nil
}

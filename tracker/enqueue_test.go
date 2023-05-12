/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tracker_test

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTracker(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	referrer := &resources.TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-name",
		},
	}
	referrerOther := &resources.TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "other-name",
		},
	}

	referent := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-name",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}

	type track struct {
		ref tracker.Reference
		obj client.Object
	}

	tests := map[string]struct {
		lease    time.Duration
		tracks   []track
		obj      client.Object
		expected []types.NamespacedName
	}{
		"empty tracker matches nothing": {
			lease:    time.Hour,
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"match by name": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Name:      "test-name",
					},
					obj: referrer,
				},
			},
			obj: referent,
			expected: []types.NamespacedName{
				{Namespace: "test-namespace", Name: "test-name"},
			},
		},
		"multiple matches by name": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Name:      "test-name",
					},
					obj: referrer,
				},
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Name:      "test-name",
					},
					obj: referrerOther,
				},
			},
			obj: referent,
			expected: []types.NamespacedName{
				{Namespace: "test-namespace", Name: "other-name"},
				{Namespace: "test-namespace", Name: "test-name"},
			},
		},
		"does not match other names": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Name:      "other-name",
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"does not match other namespaces": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "other-namespace",
						Name:      "test-name",
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"does not match other groups": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "fake",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Name:      "test-name",
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"does not match other kinds": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "Secret",
						Namespace: "test-namespace",
						Name:      "test-name",
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"match by selector": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup: "",
						Kind:     "ConfigMap",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "test",
						}),
					},
					obj: referrer,
				},
			},
			obj: referent,
			expected: []types.NamespacedName{
				{Namespace: "test-namespace", Name: "test-name"},
			},
		},
		"match by selector in namespace": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "test",
						}),
					},
					obj: referrer,
				},
			},
			obj: referent,
			expected: []types.NamespacedName{
				{Namespace: "test-namespace", Name: "test-name"},
			},
		},
		"no match by selector in wrong namespace": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "other-namespace",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "test",
						}),
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"no match by selector missing label": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "other",
						}),
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"multiple matches by selector": {
			lease: time.Hour,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup: "",
						Kind:     "ConfigMap",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "test",
						}),
					},
					obj: referrer,
				},
				{
					ref: tracker.Reference{
						APIGroup: "",
						Kind:     "ConfigMap",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "test",
						}),
					},
					obj: referrerOther,
				},
			},
			obj: referent,
			expected: []types.NamespacedName{
				{Namespace: "test-namespace", Name: "other-name"},
				{Namespace: "test-namespace", Name: "test-name"},
			},
		},
		"no match by name for expired lease": {
			lease: time.Nanosecond,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup:  "",
						Kind:      "ConfigMap",
						Namespace: "test-namespace",
						Name:      "test-name",
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
		"no match by selector for expired lease": {
			lease: time.Nanosecond,
			tracks: []track{
				{
					ref: tracker.Reference{
						APIGroup: "",
						Kind:     "ConfigMap",
						Selector: labels.SelectorFromSet(map[string]string{
							"app": "test",
						}),
					},
					obj: referrer,
				},
			},
			obj:      referent,
			expected: []types.NamespacedName{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tracker := tracker.New(scheme, tc.lease)
			for _, track := range tc.tracks {
				tracker.TrackReference(track.ref, track.obj)
			}

			actual, _ := tracker.GetObservers(tc.obj)
			sort.Slice(actual, func(i, j int) bool {
				if in, jn := actual[i].Namespace, actual[j].Namespace; in != jn {
					return in < jn
				}
				return actual[i].Name < actual[j].Name
			})
			expected := tc.expected
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("expected observers to match actual observers: %s", diff)
			}
		})
	}
}

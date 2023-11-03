/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestResourceReconciler_validate_TestResource(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *ResourceReconciler[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "valid",
			reconciler: &ResourceReconciler[*resources.TestResource]{
				Reconciler: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "with type",
			reconciler: &ResourceReconciler[*resources.TestResource]{
				Name:       "with type",
				Type:       &resources.TestResource{},
				Reconciler: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "missing reconciler",
			reconciler: &ResourceReconciler[*resources.TestResource]{
				Name: "missing reconciler",
			},
			shouldErr: `ResourceReconciler "missing reconciler" must define Reconciler`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestResourceReconciler_validate_TestResourceNoStatus(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *ResourceReconciler[*resources.TestResourceNoStatus]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "type has no status",
			reconciler: &ResourceReconciler[*resources.TestResourceNoStatus]{
				Reconciler: Sequence[*resources.TestResourceNoStatus]{},
			},
			expectedLogs: []string{
				"resource missing status field, operations related to status will be skipped",
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestResourceReconciler_validate_TestResourceEmptyStatus(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *ResourceReconciler[*resources.TestResourceEmptyStatus]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "type has empty status",
			reconciler: &ResourceReconciler[*resources.TestResourceEmptyStatus]{
				Reconciler: Sequence[*resources.TestResourceEmptyStatus]{},
			},
			expectedLogs: []string{
				"resource status missing ObservedGeneration field of type int64, generation will not be managed",
				"resource status missing InitializeConditions(context.Context) method, conditions will not be auto-initialized",
				"resource status is missing field Conditions of type []metav1.Condition, condition timestamps will not be managed",
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestResourceReconciler_validate_TestResourceNilableStatus(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *ResourceReconciler[*resources.TestResourceNilableStatus]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "type has nilable status",
			reconciler: &ResourceReconciler[*resources.TestResourceNilableStatus]{
				Reconciler: Sequence[*resources.TestResourceNilableStatus]{},
			},
			expectedLogs: []string{
				"resource status is nilable, status is typically a struct",
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestAggregateReconciler_validate(t *testing.T) {
	req := Request{
		NamespacedName: types.NamespacedName{
			Namespace: "my-namespace",
			Name:      "my-name",
		},
	}

	tests := []struct {
		name         string
		reconciler   *AggregateReconciler[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name:       "empty",
			reconciler: &AggregateReconciler[*resources.TestResource]{},
			shouldErr:  `AggregateReconciler "" must define Request`,
		},
		{
			name: "valid",
			reconciler: &AggregateReconciler[*resources.TestResource]{
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence[*resources.TestResource]{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
		},
		{
			name: "Type missing",
			reconciler: &AggregateReconciler[*resources.TestResource]{
				Name: "Type missing",
				// Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence[*resources.TestResource]{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
		},
		{
			name: "Request missing",
			reconciler: &AggregateReconciler[*resources.TestResource]{
				Name: "Request missing",
				Type: &resources.TestResource{},
				// Request:           req,
				Reconciler:        Sequence[*resources.TestResource]{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
			shouldErr: `AggregateReconciler "Request missing" must define Request`,
		},
		{
			name: "Reconciler missing",
			reconciler: &AggregateReconciler[*resources.TestResource]{
				Name:    "Reconciler missing",
				Type:    &resources.TestResource{},
				Request: req,
				// Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
			shouldErr: `AggregateReconciler "Reconciler missing" must define Reconciler and/or DesiredResource`,
		},
		{
			name: "DesiredResource",
			reconciler: &AggregateReconciler[*resources.TestResource]{
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence[*resources.TestResource]{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				DesiredResource: func(ctx context.Context, resource *resources.TestResource) (*resources.TestResource, error) {
					return nil, nil
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestSyncReconciler_validate(t *testing.T) {
	tests := []struct {
		name       string
		resource   client.Object
		reconciler *SyncReconciler[*corev1.ConfigMap]
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{},
			shouldErr:  `SyncReconciler "" must implement Sync or SyncWithResult`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
			},
		},
		{
			name:     "valid SyncWithResult",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				SyncWithResult: func(ctx context.Context, resource *corev1.ConfigMap) (Result, error) {
					return Result{}, nil
				},
			},
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
			},
		},
		{
			name:     "invalid Sync and SyncWithResult",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				SyncWithResult: func(ctx context.Context, resource *corev1.ConfigMap) (Result, error) {
					return Result{}, nil
				},
			},
			shouldErr: `SyncReconciler "" may not implement both Sync and SyncWithResult`,
		},
		{
			name:     "valid Finalize",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
			},
		},
		{
			name:     "valid Finalize with result",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				FinalizeWithResult: func(ctx context.Context, resource *corev1.ConfigMap) (Result, error) {
					return Result{}, nil
				},
			},
		},
		{
			name:     "invalid Finalize and FinalizeWithResult",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler[*corev1.ConfigMap]{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				FinalizeWithResult: func(ctx context.Context, resource *corev1.ConfigMap) (Result, error) {
					return Result{}, nil
				},
			},
			shouldErr: `SyncReconciler "" may not implement both Finalize and FinalizeWithResult`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashResourceType(context.TODO(), c.resource)
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
		})
	}
}

func TestChildReconciler_validate(t *testing.T) {
	tests := []struct {
		name       string
		parent     *corev1.ConfigMap
		reconciler *ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{},
			shouldErr:  `ChildReconciler "" must implement DesiredChild`,
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
			},
		},
		{
			name:   "ChildType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name: "ChildType missing",
				// ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
			},
		},
		{
			name:   "ChildListType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:      "ChildListType missing",
				ChildType: &corev1.Pod{},
				// ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
			},
		},
		{
			name:   "DesiredChild missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:          "DesiredChild missing",
				ChildType:     &corev1.Pod{},
				ChildListType: &corev1.PodList{},
				// DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "DesiredChild missing" must implement DesiredChild`,
		},
		{
			name:   "ReflectChildStatusOnParent missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:          "ReflectChildStatusOnParent missing",
				ChildType:     &corev1.Pod{},
				ChildListType: &corev1.PodList{},
				DesiredChild:  func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				// ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate: func(current, desired *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent missing" must implement ReflectChildStatusOnParent`,
		},
		{
			name:   "MergeBeforeUpdate missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:                       "MergeBeforeUpdate missing",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				//MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "MergeBeforeUpdate missing" must implement MergeBeforeUpdate`,
		},
		{
			name:   "ListOptions",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				ListOptions:                func(ctx context.Context, parent *corev1.ConfigMap) []client.ListOption { return []client.ListOption{} },
			},
		},
		{
			name:   "Finalizer without OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:                       "Finalizer without OurChild",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				Finalizer:                  "my-finalizer",
			},
			shouldErr: `ChildReconciler "Finalizer without OurChild" must implement OurChild since owner references are not used`,
		},
		{
			name:   "SkipOwnerReference without OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:                       "SkipOwnerReference without OurChild",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SkipOwnerReference:         true,
			},
			shouldErr: `ChildReconciler "SkipOwnerReference without OurChild" must implement OurChild since owner references are not used`,
		},
		{
			name:   "OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				OurChild:                   func(parent *corev1.ConfigMap, child *corev1.Pod) bool { return false },
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashResourceType(context.TODO(), c.parent)
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err.Error(), c.shouldErr)
			}
		})
	}
}

func TestChildSetReconciler_validate(t *testing.T) {
	tests := []struct {
		name       string
		parent     *corev1.ConfigMap
		reconciler *ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{},
			shouldErr:  `ChildSetReconciler "" must implement DesiredChildren`,
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				ChildType:                     &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
		},
		{
			name:   "ChildType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name: "ChildType missing",
				// ChildType:                  &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
		},
		{
			name:   "ChildListType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:      "ChildListType missing",
				ChildType: &corev1.Pod{},
				// ChildListType:              &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
		},
		{
			name:   "DesiredChildren missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:          "DesiredChildren missing",
				ChildType:     &corev1.Pod{},
				ChildListType: &corev1.PodList{},
				// DesiredChildren:            func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
			shouldErr: `ChildSetReconciler "DesiredChildren missing" must implement DesiredChildren`,
		},
		{
			name:   "ReflectChildrenStatusOnParent missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:            "ReflectChildrenStatusOnParent missing",
				ChildType:       &corev1.Pod{},
				ChildListType:   &corev1.PodList{},
				DesiredChildren: func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				// ReflectChildrenStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate: func(current, desired *corev1.Pod) {},
				IdentifyChild:     func(child *corev1.Pod) string { return "" },
			},
			shouldErr: `ChildSetReconciler "ReflectChildrenStatusOnParent missing" must implement ReflectChildrenStatusOnParent`,
		},
		{
			name:   "IdentifyChild missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:                          "IdentifyChild missing",
				ChildType:                     &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				// IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
			shouldErr: `ChildSetReconciler "IdentifyChild missing" must implement IdentifyChild`,
		},
		{
			name:   "ListOptions",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				ChildType:                     &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				ListOptions:                   func(ctx context.Context, parent *corev1.ConfigMap) []client.ListOption { return []client.ListOption{} },
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
		},
		{
			name:   "Finalizer without OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:                          "Finalizer without OurChild",
				ChildType:                     &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				Finalizer:                     "my-finalizer",
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
			shouldErr: `ChildSetReconciler "Finalizer without OurChild" must implement OurChild since owner references are not used`,
		},
		{
			name:   "SkipOwnerReference without OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				Name:                          "SkipOwnerReference without OurChild",
				ChildType:                     &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				SkipOwnerReference:            true,
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
			shouldErr: `ChildSetReconciler "SkipOwnerReference without OurChild" must implement OurChild since owner references are not used`,
		},
		{
			name:   "OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildSetReconciler[*corev1.ConfigMap, *corev1.Pod, *corev1.PodList]{
				ChildType:                     &corev1.Pod{},
				ChildListType:                 &corev1.PodList{},
				DesiredChildren:               func(ctx context.Context, parent *corev1.ConfigMap) ([]*corev1.Pod, error) { return nil, nil },
				ReflectChildrenStatusOnParent: func(ctx context.Context, parent *corev1.ConfigMap, result ChildSetResult[*corev1.Pod]) {},
				MergeBeforeUpdate:             func(current, desired *corev1.Pod) {},
				OurChild:                      func(parent *corev1.ConfigMap, child *corev1.Pod) bool { return false },
				IdentifyChild:                 func(child *corev1.Pod) string { return "" },
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashResourceType(context.TODO(), c.parent)
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err.Error(), c.shouldErr)
			}
		})
	}
}

func TestCastResource_validate(t *testing.T) {
	tests := []struct {
		name       string
		resource   client.Object
		reconciler *CastResource[*corev1.ConfigMap, *corev1.Secret]
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &CastResource[*corev1.ConfigMap, *corev1.Secret]{},
			shouldErr:  `CastResource "" must define Reconciler`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &CastResource[*corev1.ConfigMap, *corev1.Secret]{
				Reconciler: &SyncReconciler[*corev1.Secret]{
					Sync: func(ctx context.Context, resource *corev1.Secret) error {
						return nil
					},
				},
			},
		},
		{
			name:     "missing reconciler",
			resource: &corev1.ConfigMap{},
			reconciler: &CastResource[*corev1.ConfigMap, *corev1.Secret]{
				Name:       "missing reconciler",
				Reconciler: nil,
			},
			shouldErr: `CastResource "missing reconciler" must define Reconciler`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashResourceType(context.TODO(), c.resource)
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
		})
	}
}

func TestWithConfig_validate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	config := Config{
		Tracker: tracker.New(scheme, 0),
	}

	tests := []struct {
		name       string
		resource   client.Object
		reconciler *WithConfig[*corev1.ConfigMap]
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &WithConfig[*corev1.ConfigMap]{},
			shouldErr:  `WithConfig "" must define Config`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &WithConfig[*corev1.ConfigMap]{
				Reconciler: &Sequence[*corev1.ConfigMap]{},
				Config: func(ctx context.Context, c Config) (Config, error) {
					return config, nil
				},
			},
		},
		{
			name:     "missing config",
			resource: &corev1.ConfigMap{},
			reconciler: &WithConfig[*corev1.ConfigMap]{
				Name:       "missing config",
				Reconciler: &Sequence[*corev1.ConfigMap]{},
			},
			shouldErr: `WithConfig "missing config" must define Config`,
		},
		{
			name:     "missing reconciler",
			resource: &corev1.ConfigMap{},
			reconciler: &WithConfig[*corev1.ConfigMap]{
				Name: "missing reconciler",
				Config: func(ctx context.Context, c Config) (Config, error) {
					return config, nil
				},
			},
			shouldErr: `WithConfig "missing reconciler" must define Reconciler`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
		})
	}
}

func TestWithFinalizer_validate(t *testing.T) {
	tests := []struct {
		name       string
		resource   client.Object
		reconciler *WithFinalizer[*corev1.ConfigMap]
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &WithFinalizer[*corev1.ConfigMap]{},
			shouldErr:  `WithFinalizer "" must define Finalizer`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &WithFinalizer[*corev1.ConfigMap]{
				Reconciler: &Sequence[*corev1.ConfigMap]{},
				Finalizer:  "my-finalizer",
			},
		},
		{
			name:     "missing finalizer",
			resource: &corev1.ConfigMap{},
			reconciler: &WithFinalizer[*corev1.ConfigMap]{
				Name:       "missing finalizer",
				Reconciler: &Sequence[*corev1.ConfigMap]{},
			},
			shouldErr: `WithFinalizer "missing finalizer" must define Finalizer`,
		},
		{
			name:     "missing reconciler",
			resource: &corev1.ConfigMap{},
			reconciler: &WithFinalizer[*corev1.ConfigMap]{
				Name:      "missing reconciler",
				Finalizer: "my-finalizer",
			},
			shouldErr: `WithFinalizer "missing reconciler" must define Reconciler`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
		})
	}
}

func TestResourceManager_validate(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *ResourceManager[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name:       "empty",
			reconciler: &ResourceManager[*resources.TestResource]{},
			shouldErr:  `ResourceManager "" must define MergeBeforeUpdate`,
		},
		{
			name: "valid",
			reconciler: &ResourceManager[*resources.TestResource]{
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
		},
		{
			name: "Type missing",
			reconciler: &ResourceManager[*resources.TestResource]{
				Name: "Type missing",
				// Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
		},
		{
			name: "MergeBeforeUpdate missing",
			reconciler: &ResourceManager[*resources.TestResource]{
				Name: "MergeBeforeUpdate missing",
				Type: &resources.TestResource{},
				// MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
			},
			shouldErr: `ResourceManager "MergeBeforeUpdate missing" must define MergeBeforeUpdate`,
		},
		{
			name: "HarmonizeImmutableFields",
			reconciler: &ResourceManager[*resources.TestResource]{
				Type:                     &resources.TestResource{},
				MergeBeforeUpdate:        func(current, desired *resources.TestResource) {},
				HarmonizeImmutableFields: func(current, desired *resources.TestResource) {},
			},
		},
		{
			name: "Sanitize",
			reconciler: &ResourceManager[*resources.TestResource]{
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				Sanitize:          func(child *resources.TestResource) interface{} { return child.Spec },
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestIfThen_validate(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *IfThen[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "valid",
			reconciler: &IfThen[*resources.TestResource]{
				If: func(ctx context.Context, resource *resources.TestResource) bool {
					return false
				},
				Then: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "missing if",
			reconciler: &IfThen[*resources.TestResource]{
				Name: "missing if",
				Then: Sequence[*resources.TestResource]{},
			},
			shouldErr: `IfThen "missing if" must implement If`,
		},
		{
			name: "missing then",
			reconciler: &IfThen[*resources.TestResource]{
				Name: "missing then",
				If: func(ctx context.Context, resource *resources.TestResource) bool {
					return false
				},
			},
			shouldErr: `IfThen "missing then" must implement Then`,
		},
		{
			name: "with else",
			reconciler: &IfThen[*resources.TestResource]{
				If: func(ctx context.Context, resource *resources.TestResource) bool {
					return false
				},
				Then: Sequence[*resources.TestResource]{},
				Else: Sequence[*resources.TestResource]{},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestWhile_validate(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *While[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "valid",
			reconciler: &While[*resources.TestResource]{
				Condition: func(ctx context.Context, resource *resources.TestResource) bool {
					return false
				},
				Reconciler: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "missing condition",
			reconciler: &While[*resources.TestResource]{
				Name:       "missing condition",
				Reconciler: Sequence[*resources.TestResource]{},
			},
			shouldErr: `While "missing condition" must implement Condition`,
		},
		{
			name: "missing reconciler",
			reconciler: &While[*resources.TestResource]{
				Name: "missing reconciler",
				Condition: func(ctx context.Context, resource *resources.TestResource) bool {
					return false
				},
			},
			shouldErr: `While "missing reconciler" must implement Reconciler`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestTryCatch_validate(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *TryCatch[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "valid",
			reconciler: &TryCatch[*resources.TestResource]{
				Try: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "with catch",
			reconciler: &TryCatch[*resources.TestResource]{
				Try: Sequence[*resources.TestResource]{},
				Catch: func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					return result, err
				},
			},
		},
		{
			name: "with finally",
			reconciler: &TryCatch[*resources.TestResource]{
				Try:     Sequence[*resources.TestResource]{},
				Finally: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "with catch and finally",
			reconciler: &TryCatch[*resources.TestResource]{
				Try: Sequence[*resources.TestResource]{},
				Catch: func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					return result, err
				},
				Finally: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "missing try",
			reconciler: &TryCatch[*resources.TestResource]{
				Name: "missing try",
			},
			shouldErr: `TryCatch "missing try" must implement Try`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

func TestOverrideSetup_validate(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *OverrideSetup[*resources.TestResource]
		shouldErr    string
		expectedLogs []string
	}{
		{
			name: "with reconciler",
			reconciler: &OverrideSetup[*resources.TestResource]{
				Reconciler: Sequence[*resources.TestResource]{},
			},
		},
		{
			name: "with setup",
			reconciler: &OverrideSetup[*resources.TestResource]{
				Setup: func(ctx context.Context, mgr manager.Manager, bldr *builder.Builder) error {
					return nil
				},
			},
		},
		{
			name: "with reconciler and setup",
			reconciler: &OverrideSetup[*resources.TestResource]{
				Reconciler: Sequence[*resources.TestResource]{},
				Setup: func(ctx context.Context, mgr manager.Manager, bldr *builder.Builder) error {
					return nil
				},
			},
		},
		{
			name: "missing reconciler or setup",
			reconciler: &OverrideSetup[*resources.TestResource]{
				Name: "missing reconciler or setup",
			},
			shouldErr: `OverrideSetup "missing reconciler or setup" must implement at least one of Setup or Reconciler`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			sink := &bufferedSink{}
			ctx := logr.NewContext(context.TODO(), logr.New(sink))
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err, c.shouldErr)
			}
			if diff := cmp.Diff(c.expectedLogs, sink.Lines); diff != "" {
				t.Errorf("%s: unexpected logs (-expected, +actual): %s", c.name, diff)
			}
		})
	}
}

var _ logr.LogSink = &bufferedSink{}

type bufferedSink struct {
	Lines []string
}

func (s *bufferedSink) Init(info logr.RuntimeInfo) {}
func (s *bufferedSink) Enabled(level int) bool {
	return true
}
func (s *bufferedSink) Info(level int, msg string, keysAndValues ...interface{}) {
	s.Lines = append(s.Lines, msg)
}
func (s *bufferedSink) Error(err error, msg string, keysAndValues ...interface{}) {
	s.Lines = append(s.Lines, msg)
}
func (s *bufferedSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return s
}
func (s *bufferedSink) WithName(name string) logr.LogSink {
	return s
}

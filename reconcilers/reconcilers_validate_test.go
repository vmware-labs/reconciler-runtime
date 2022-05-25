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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestResourceReconciler_validate(t *testing.T) {
	tests := []struct {
		name         string
		reconciler   *ResourceReconciler
		shouldErr    string
		expectedLogs []string
	}{
		{
			name:       "empty",
			reconciler: &ResourceReconciler{},
			shouldErr:  `ResourceReconciler "" must define Type`,
		},
		{
			name: "valid",
			reconciler: &ResourceReconciler{
				Type:       &resources.TestResource{},
				Reconciler: Sequence{},
			},
		},
		{
			name: "missing type",
			reconciler: &ResourceReconciler{
				Name:       "missing type",
				Reconciler: Sequence{},
			},
			shouldErr: `ResourceReconciler "missing type" must define Type`,
		},
		{
			name: "missing reconciler",
			reconciler: &ResourceReconciler{
				Name: "missing reconciler",
				Type: &resources.TestResource{},
			},
			shouldErr: `ResourceReconciler "missing reconciler" must define Reconciler`,
		},
		{
			name: "type has no status",
			reconciler: &ResourceReconciler{
				Type:       &resources.TestResourceNoStatus{},
				Reconciler: Sequence{},
			},
			expectedLogs: []string{
				"resource missing status field, operations related to status will be skipped",
			},
		},
		{
			name: "type has empty status",
			reconciler: &ResourceReconciler{
				Type:       &resources.TestResourceEmptyStatus{},
				Reconciler: Sequence{},
			},
			expectedLogs: []string{
				"resource status missing ObservedGeneration field of type int64, generation will not be managed",
				"resource status missing InitializeConditions() method, conditions will not be auto-initialized",
				"resource status is missing field Conditions of type []metav1.Condition, condition timestamps will not be managed",
			},
		},
		{
			name: "type has nilable status",
			reconciler: &ResourceReconciler{
				Type:       &resources.TestResourceNilableStatus{},
				Reconciler: Sequence{},
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
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "my-namespace",
			Name:      "my-name",
		},
	}

	tests := []struct {
		name         string
		reconciler   *AggregateReconciler
		shouldErr    string
		expectedLogs []string
	}{
		{
			name:       "empty",
			reconciler: &AggregateReconciler{},
			shouldErr:  `AggregateReconciler "" must define Type`,
		},
		{
			name: "valid",
			reconciler: &AggregateReconciler{
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
		},
		{
			name: "Type missing",
			reconciler: &AggregateReconciler{
				Name: "Type missing",
				// Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `AggregateReconciler "Type missing" must define Type`,
		},
		{
			name: "Request missing",
			reconciler: &AggregateReconciler{
				Name: "Request missing",
				Type: &resources.TestResource{},
				// Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `AggregateReconciler "Request missing" must define Request`,
		},
		{
			name: "Reconciler missing",
			reconciler: &AggregateReconciler{
				Name:    "Reconciler missing",
				Type:    &resources.TestResource{},
				Request: req,
				// Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `AggregateReconciler "Reconciler missing" must define Reconciler and/or DesiredResource`,
		},
		{
			name: "DesiredResource",
			reconciler: &AggregateReconciler{
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource: func(ctx context.Context, resource *resources.TestResource) (*resources.TestResource, error) {
					return nil, nil
				},
			},
		},
		{
			name: "DesiredResource num in",
			reconciler: &AggregateReconciler{
				Name:              "DesiredResource num in",
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource: func() (*resources.TestResource, error) {
					return nil, nil
				},
			},
			shouldErr: `AggregateReconciler "DesiredResource num in" must implement DesiredResource: nil | func(context.Context, *resources.TestResource) (*resources.TestResource, error), found: func() (*resources.TestResource, error)`,
		},
		{
			name: "DesiredResource in 0",
			reconciler: &AggregateReconciler{
				Name:              "DesiredResource in 0",
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource: func(err error, resource *resources.TestResource) (*resources.TestResource, error) {
					return nil, nil
				},
			},
			shouldErr: `AggregateReconciler "DesiredResource in 0" must implement DesiredResource: nil | func(context.Context, *resources.TestResource) (*resources.TestResource, error), found: func(error, *resources.TestResource) (*resources.TestResource, error)`,
		},
		{
			name: "DesiredResource in 1",
			reconciler: &AggregateReconciler{
				Name:              "DesiredResource in 1",
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource: func(ctx context.Context, resource *corev1.Pod) (*resources.TestResource, error) {
					return nil, nil
				},
			},
			shouldErr: `AggregateReconciler "DesiredResource in 1" must implement DesiredResource: nil | func(context.Context, *resources.TestResource) (*resources.TestResource, error), found: func(context.Context, *v1.Pod) (*resources.TestResource, error)`,
		},
		{
			name: "DesiredResource num out",
			reconciler: &AggregateReconciler{
				Name:              "DesiredResource num out",
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource:   func(ctx context.Context, resource *resources.TestResource) {},
			},
			shouldErr: `AggregateReconciler "DesiredResource num out" must implement DesiredResource: nil | func(context.Context, *resources.TestResource) (*resources.TestResource, error), found: func(context.Context, *resources.TestResource)`,
		},
		{
			name: "DesiredResource out 0",
			reconciler: &AggregateReconciler{
				Name:              "DesiredResource out 0",
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource: func(ctx context.Context, resource *resources.TestResource) (*corev1.Pod, error) {
					return nil, nil
				},
			},
			shouldErr: `AggregateReconciler "DesiredResource out 0" must implement DesiredResource: nil | func(context.Context, *resources.TestResource) (*resources.TestResource, error), found: func(context.Context, *resources.TestResource) (*v1.Pod, error)`,
		},
		{
			name: "DesiredResource out 1",
			reconciler: &AggregateReconciler{
				Name:              "DesiredResource out 1",
				Type:              &resources.TestResource{},
				Request:           req,
				Reconciler:        Sequence{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				DesiredResource: func(ctx context.Context, resource *resources.TestResource) (*resources.TestResource, string) {
					return nil, ""
				},
			},
			shouldErr: `AggregateReconciler "DesiredResource out 1" must implement DesiredResource: nil | func(context.Context, *resources.TestResource) (*resources.TestResource, error), found: func(context.Context, *resources.TestResource) (*resources.TestResource, string)`,
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
		reconciler *SyncReconciler
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &SyncReconciler{},
			shouldErr:  `SyncReconciler "" must implement Sync`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
			},
		},
		{
			name:     "valid Sync with result",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) (ctrl.Result, error) {
					return ctrl.Result{}, nil
				},
			},
		},
		{
			name:     "Sync num in",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync num in",
				Sync: func() error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Sync num in" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func() error`,
		},
		{
			name:     "Sync in 1",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync in 1",
				Sync: func(ctx context.Context, resource *corev1.Secret) error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Sync in 1" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.Secret) error`,
		},
		{
			name:     "Sync num out",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync num out",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) {
				},
			},
			shouldErr: `SyncReconciler "Sync num out" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap)`,
		},
		{
			name:     "Sync out 1",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync out 1",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) string {
					return ""
				},
			},
			shouldErr: `SyncReconciler "Sync out 1" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) string`,
		},
		{
			name:     "Sync result out 1",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync result out 1",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) (ctrl.Result, string) {
					return ctrl.Result{}, ""
				},
			},
			shouldErr: `SyncReconciler "Sync result out 1" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) (reconcile.Result, string)`,
		},
		{
			name:     "valid Finalize",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
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
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.ConfigMap) (ctrl.Result, error) {
					return ctrl.Result{}, nil
				},
			},
		},
		{
			name:     "Finalize num in",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize num in",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func() error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Finalize num in" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func() error`,
		},
		{
			name:     "Finalize in 1",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize in 1",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.Secret) error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Finalize in 1" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.Secret) error`,
		},
		{
			name:     "Finalize num out",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize num out",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.ConfigMap) {
				},
			},
			shouldErr: `SyncReconciler "Finalize num out" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap)`,
		},
		{
			name:     "Finalize out 1",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize out 1",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.ConfigMap) string {
					return ""
				},
			},
			shouldErr: `SyncReconciler "Finalize out 1" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) string`,
		},
		{
			name:     "Finalize result out 1",
			resource: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize result out 1",
				Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, resource *corev1.ConfigMap) (ctrl.Result, string) {
					return ctrl.Result{}, ""
				},
			},
			shouldErr: `SyncReconciler "Finalize result out 1" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) (reconcile.Result, string)`,
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
		parent     client.Object
		reconciler *ChildReconciler
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &ChildReconciler{},
			shouldErr:  `ChildReconciler "" must define ChildType`,
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
		},
		{
			name:   "ChildType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name: "ChildType missing",
				// ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ChildType missing" must define ChildType`,
		},
		{
			name:   "ChildListType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:      "ChildListType missing",
				ChildType: &corev1.Pod{},
				// ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ChildListType missing" must define ChildListType`,
		},
		{
			name:   "DesiredChild missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:          "DesiredChild missing",
				ChildType:     &corev1.Pod{},
				ChildListType: &corev1.PodList{},
				// DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild missing" must implement DesiredChild`,
		},
		{
			name:   "DesiredChild num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "DesiredChild num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func() (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild num in" must implement DesiredChild: func(context.Context, *v1.ConfigMap) (*v1.Pod, error), found: func() (*v1.Pod, error)`,
		},
		{
			name:   "DesiredChild in 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "DesiredChild in 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx string, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild in 0" must implement DesiredChild: func(context.Context, *v1.ConfigMap) (*v1.Pod, error), found: func(string, *v1.ConfigMap) (*v1.Pod, error)`,
		},
		{
			name:   "DesiredChild in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "DesiredChild in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.Secret) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild in 1" must implement DesiredChild: func(context.Context, *v1.ConfigMap) (*v1.Pod, error), found: func(context.Context, *v1.Secret) (*v1.Pod, error)`,
		},
		{
			name:   "DesiredChild num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "DesiredChild num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) {},
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild num out" must implement DesiredChild: func(context.Context, *v1.ConfigMap) (*v1.Pod, error), found: func(context.Context, *v1.ConfigMap)`,
		},
		{
			name:   "DesiredChild out 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "DesiredChild out 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Secret, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild out 0" must implement DesiredChild: func(context.Context, *v1.ConfigMap) (*v1.Pod, error), found: func(context.Context, *v1.ConfigMap) (*v1.Secret, error)`,
		},
		{
			name:   "DesiredChild out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "DesiredChild out 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, string) { return nil, "" },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "DesiredChild out 1" must implement DesiredChild: func(context.Context, *v1.ConfigMap) (*v1.Pod, error), found: func(context.Context, *v1.ConfigMap) (*v1.Pod, string)`,
		},
		{
			name:   "ReflectChildStatusOnParent missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:          "ReflectChildStatusOnParent missing",
				ChildType:     &corev1.Pod{},
				ChildListType: &corev1.PodList{},
				DesiredChild:  func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				// ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate: func(current, desired *corev1.Pod) {},
				SemanticEquals:    func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent missing" must implement ReflectChildStatusOnParent`,
		},
		{
			name:   "ReflectChildStatusOnParent num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ReflectChildStatusOnParent num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func() {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent num in" must implement ReflectChildStatusOnParent: func(*v1.ConfigMap, *v1.Pod, error), found: func()`,
		},
		{
			name:   "ReflectChildStatusOnParent in 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ReflectChildStatusOnParent in 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.Secret, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent in 0" must implement ReflectChildStatusOnParent: func(*v1.ConfigMap, *v1.Pod, error), found: func(*v1.Secret, *v1.Pod, error)`,
		},
		{
			name:   "ReflectChildStatusOnParent in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ReflectChildStatusOnParent in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Secret, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent in 1" must implement ReflectChildStatusOnParent: func(*v1.ConfigMap, *v1.Pod, error), found: func(*v1.ConfigMap, *v1.Secret, error)`,
		},
		{
			name:   "ReflectChildStatusOnParent in 2",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ReflectChildStatusOnParent in 2",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, ctx context.Context) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent in 2" must implement ReflectChildStatusOnParent: func(*v1.ConfigMap, *v1.Pod, error), found: func(*v1.ConfigMap, *v1.Pod, context.Context)`,
		},
		{
			name:   "ReflectChildStatusOnParent num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ReflectChildStatusOnParent num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) error { return nil },
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "ReflectChildStatusOnParent num out" must implement ReflectChildStatusOnParent: func(*v1.ConfigMap, *v1.Pod, error), found: func(*v1.ConfigMap, *v1.Pod, error) error`,
		},
		{
			name:   "ListOptions",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				ListOptions:                func(ctx context.Context, parent *corev1.ConfigMap) []client.ListOption { return []client.ListOption{} },
			},
		},
		{
			name:   "ListOptions num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ListOptions num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				ListOptions:                func() []client.ListOption { return []client.ListOption{} },
			},
			shouldErr: `ChildReconciler "ListOptions num in" must implement ListOptions: nil | func(context.Context, *v1.ConfigMap) []client.ListOption, found: func() []client.ListOption`,
		},
		{
			name:   "ListOptions in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ListOptions in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				ListOptions:                func(child *corev1.Secret, parent *corev1.ConfigMap) []client.ListOption { return []client.ListOption{} },
			},
			shouldErr: `ChildReconciler "ListOptions in 1" must implement ListOptions: nil | func(context.Context, *v1.ConfigMap) []client.ListOption, found: func(*v1.Secret, *v1.ConfigMap) []client.ListOption`,
		},
		{
			name:   "ListOptions in 2",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ListOptions in 2",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				ListOptions:                func(ctx context.Context, parent *corev1.Secret) []client.ListOption { return []client.ListOption{} },
			},
			shouldErr: `ChildReconciler "ListOptions in 2" must implement ListOptions: nil | func(context.Context, *v1.ConfigMap) []client.ListOption, found: func(context.Context, *v1.Secret) []client.ListOption`,
		},
		{
			name:   "ListOptions num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ListOptions num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				ListOptions:                func(ctx context.Context, parent *corev1.ConfigMap) {},
			},
			shouldErr: `ChildReconciler "ListOptions num out" must implement ListOptions: nil | func(context.Context, *v1.ConfigMap) []client.ListOption, found: func(context.Context, *v1.ConfigMap)`,
		},
		{
			name:   "ListOptions out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "ListOptions out 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				ListOptions:                func(ctx context.Context, parent *corev1.ConfigMap) client.ListOptions { return client.ListOptions{} },
			},
			shouldErr: `ChildReconciler "ListOptions out 1" must implement ListOptions: nil | func(context.Context, *v1.ConfigMap) []client.ListOption, found: func(context.Context, *v1.ConfigMap) client.ListOptions`,
		},
		{
			name:   "Finalizer without OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "Finalizer without OurChild",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				Finalizer:                  "my-finalizer",
			},
			shouldErr: `ChildReconciler "Finalizer without OurChild" must implement OurChild since owner references are not used`,
		},
		{
			name:   "SkipOwnerReference without OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SkipOwnerReference without OurChild",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				SkipOwnerReference:         true,
			},
			shouldErr: `ChildReconciler "SkipOwnerReference without OurChild" must implement OurChild since owner references are not used`,
		},
		{
			name:   "OurChild",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				OurChild:                   func(parent *corev1.ConfigMap, child *corev1.Pod) bool { return false },
			},
		},
		{
			name:   "OurChild num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "OurChild num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				OurChild:                   func() bool { return false },
			},
			shouldErr: `ChildReconciler "OurChild num in" must implement OurChild: nil | func(*v1.ConfigMap, *v1.Pod) bool, found: func() bool`,
		},
		{
			name:   "OurChild in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "OurChild in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				OurChild:                   func(parent *corev1.ConfigMap, child *corev1.Secret) bool { return false },
			},
			shouldErr: `ChildReconciler "OurChild in 1" must implement OurChild: nil | func(*v1.ConfigMap, *v1.Pod) bool, found: func(*v1.ConfigMap, *v1.Secret) bool`,
		},
		{
			name:   "OurChild in 2",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "OurChild in 2",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				OurChild:                   func(parent *corev1.Secret, child *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "OurChild in 2" must implement OurChild: nil | func(*v1.ConfigMap, *v1.Pod) bool, found: func(*v1.Secret, *v1.Pod) bool`,
		},
		{
			name:   "OurChild num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "OurChild num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				OurChild:                   func(parent *corev1.ConfigMap, child *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "OurChild num out" must implement OurChild: nil | func(*v1.ConfigMap, *v1.Pod) bool, found: func(*v1.ConfigMap, *v1.Pod)`,
		},
		{
			name:   "OurChild out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "OurChild out 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				OurChild:                   func(parent *corev1.ConfigMap, child *corev1.Pod) *corev1.Pod { return child },
			},
			shouldErr: `ChildReconciler "OurChild out 1" must implement OurChild: nil | func(*v1.ConfigMap, *v1.Pod) bool, found: func(*v1.ConfigMap, *v1.Pod) *v1.Pod`,
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
		reconciler *CastResource
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &CastResource{},
			shouldErr:  `CastResource "" must define Type`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &CastResource{
				Type: &corev1.Secret{},
				Reconciler: &SyncReconciler{
					Sync: func(ctx context.Context, resource *corev1.Secret) error {
						return nil
					},
				},
			},
		},
		{
			name:     "missing type",
			resource: &corev1.ConfigMap{},
			reconciler: &CastResource{
				Name: "missing type",
				Type: nil,
				Reconciler: &SyncReconciler{
					Sync: func(ctx context.Context, resource *corev1.Secret) error {
						return nil
					},
				},
			},
			shouldErr: `CastResource "missing type" must define Type`,
		},
		{
			name:     "missing reconciler",
			resource: &corev1.ConfigMap{},
			reconciler: &CastResource{
				Name:       "missing reconciler",
				Type:       &corev1.Secret{},
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
	config := Config{
		Tracker: tracker.New(0),
	}

	tests := []struct {
		name       string
		resource   client.Object
		reconciler *WithConfig
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &WithConfig{},
			shouldErr:  `WithConfig "" must define Config`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &WithConfig{
				Reconciler: &Sequence{},
				Config: func(ctx context.Context, c Config) (Config, error) {
					return config, nil
				},
			},
		},
		{
			name:     "missing config",
			resource: &corev1.ConfigMap{},
			reconciler: &WithConfig{
				Name:       "missing config",
				Reconciler: &Sequence{},
			},
			shouldErr: `WithConfig "missing config" must define Config`,
		},
		{
			name:     "missing reconciler",
			resource: &corev1.ConfigMap{},
			reconciler: &WithConfig{
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
		reconciler *WithFinalizer
		shouldErr  string
	}{
		{
			name:       "empty",
			resource:   &corev1.ConfigMap{},
			reconciler: &WithFinalizer{},
			shouldErr:  `WithFinalizer "" must define Finalizer`,
		},
		{
			name:     "valid",
			resource: &corev1.ConfigMap{},
			reconciler: &WithFinalizer{
				Reconciler: &Sequence{},
				Finalizer:  "my-finalizer",
			},
		},
		{
			name:     "missing finalizer",
			resource: &corev1.ConfigMap{},
			reconciler: &WithFinalizer{
				Name:       "missing finalizer",
				Reconciler: &Sequence{},
			},
			shouldErr: `WithFinalizer "missing finalizer" must define Finalizer`,
		},
		{
			name:     "missing reconciler",
			resource: &corev1.ConfigMap{},
			reconciler: &WithFinalizer{
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
		reconciler   *ResourceManager
		shouldErr    string
		expectedLogs []string
	}{
		{
			name:       "empty",
			reconciler: &ResourceManager{},
			shouldErr:  `ResourceManager "" must define Type`,
		},
		{
			name: "valid",
			reconciler: &ResourceManager{
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
		},
		{
			name: "Type missing",
			reconciler: &ResourceManager{
				Name: "Type missing",
				// Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "Type missing" must define Type`,
		},
		{
			name: "MergeBeforeUpdate missing",
			reconciler: &ResourceManager{
				Name: "MergeBeforeUpdate missing",
				Type: &resources.TestResource{},
				// MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals: func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "MergeBeforeUpdate missing" must define MergeBeforeUpdate`,
		},
		{
			name: "MergeBeforeUpdate num in",
			reconciler: &ResourceManager{
				Name:              "MergeBeforeUpdate num in",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func() {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "MergeBeforeUpdate num in" must implement MergeBeforeUpdate: func(*resources.TestResource, *resources.TestResource), found: func()`,
		},
		{
			name: "MergeBeforeUpdate in 0",
			reconciler: &ResourceManager{
				Name:              "MergeBeforeUpdate in 0",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current *corev1.Pod, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "MergeBeforeUpdate in 0" must implement MergeBeforeUpdate: func(*resources.TestResource, *resources.TestResource), found: func(*v1.Pod, *resources.TestResource)`,
		},
		{
			name: "MergeBeforeUpdate in 1",
			reconciler: &ResourceManager{
				Name:              "MergeBeforeUpdate in 1",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current *resources.TestResource, desired *corev1.Pod) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "MergeBeforeUpdate in 1" must implement MergeBeforeUpdate: func(*resources.TestResource, *resources.TestResource), found: func(*resources.TestResource, *v1.Pod)`,
		},
		{
			name: "MergeBeforeUpdate num out",
			reconciler: &ResourceManager{
				Name:              "MergeBeforeUpdate num out",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) error { return nil },
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "MergeBeforeUpdate num out" must implement MergeBeforeUpdate: func(*resources.TestResource, *resources.TestResource), found: func(*resources.TestResource, *resources.TestResource) error`,
		},
		{
			name: "SemanticEquals missing",
			reconciler: &ResourceManager{
				Name:              "SemanticEquals missing",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				// SemanticEquals: func(a1, a2 *resources.TestResource) bool { return true },
			},
			shouldErr: `ResourceManager "SemanticEquals missing" must define SemanticEquals`,
		},
		{
			name: "SemanticEquals num in",
			reconciler: &ResourceManager{
				Name:              "SemanticEquals num in",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func() bool { return false },
			},
			shouldErr: `ResourceManager "SemanticEquals num in" must implement SemanticEquals: func(*resources.TestResource, *resources.TestResource) bool, found: func() bool`,
		},
		{
			name: "SemanticEquals in 0",
			reconciler: &ResourceManager{
				Name:              "SemanticEquals in 0",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1 *corev1.Pod, a2 *resources.TestResource) bool { return false },
			},
			shouldErr: `ResourceManager "SemanticEquals in 0" must implement SemanticEquals: func(*resources.TestResource, *resources.TestResource) bool, found: func(*v1.Pod, *resources.TestResource) bool`,
		},
		{
			name: "SemanticEquals in 1",
			reconciler: &ResourceManager{
				Name:              "SemanticEquals in 1",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1 *resources.TestResource, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ResourceManager "SemanticEquals in 1" must implement SemanticEquals: func(*resources.TestResource, *resources.TestResource) bool, found: func(*resources.TestResource, *v1.Pod) bool`,
		},
		{
			name: "SemanticEquals num out",
			reconciler: &ResourceManager{
				Name:              "SemanticEquals num out",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) {},
			},
			shouldErr: `ResourceManager "SemanticEquals num out" must implement SemanticEquals: func(*resources.TestResource, *resources.TestResource) bool, found: func(*resources.TestResource, *resources.TestResource)`,
		},
		{
			name: "SemanticEquals out 0",
			reconciler: &ResourceManager{
				Name:              "SemanticEquals out 0",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) error { return nil },
			},
			shouldErr: `ResourceManager "SemanticEquals out 0" must implement SemanticEquals: func(*resources.TestResource, *resources.TestResource) bool, found: func(*resources.TestResource, *resources.TestResource) error`,
		},
		{
			name: "HarmonizeImmutableFields",
			reconciler: &ResourceManager{
				Type:                     &resources.TestResource{},
				MergeBeforeUpdate:        func(current, desired *resources.TestResource) {},
				SemanticEquals:           func(a1, a2 *resources.TestResource) bool { return true },
				HarmonizeImmutableFields: func(current, desired *resources.TestResource) {},
			},
		},
		{
			name: "HarmonizeImmutableFields num in",
			reconciler: &ResourceManager{
				Name:                     "HarmonizeImmutableFields num in",
				Type:                     &resources.TestResource{},
				MergeBeforeUpdate:        func(current, desired *resources.TestResource) {},
				SemanticEquals:           func(a1, a2 *resources.TestResource) bool { return true },
				HarmonizeImmutableFields: func() {},
			},
			shouldErr: `ResourceManager "HarmonizeImmutableFields num in" must implement HarmonizeImmutableFields: nil | func(*resources.TestResource, *resources.TestResource), found: func()`,
		},
		{
			name: "HarmonizeImmutableFields in 0",
			reconciler: &ResourceManager{
				Name:                     "HarmonizeImmutableFields in 0",
				Type:                     &resources.TestResource{},
				MergeBeforeUpdate:        func(current, desired *resources.TestResource) {},
				SemanticEquals:           func(a1, a2 *resources.TestResource) bool { return true },
				HarmonizeImmutableFields: func(current *corev1.Pod, desired *resources.TestResource) {},
			},
			shouldErr: `ResourceManager "HarmonizeImmutableFields in 0" must implement HarmonizeImmutableFields: nil | func(*resources.TestResource, *resources.TestResource), found: func(*v1.Pod, *resources.TestResource)`,
		},
		{
			name: "HarmonizeImmutableFields in 1",
			reconciler: &ResourceManager{
				Name:                     "HarmonizeImmutableFields in 1",
				Type:                     &resources.TestResource{},
				MergeBeforeUpdate:        func(current, desired *resources.TestResource) {},
				SemanticEquals:           func(a1, a2 *resources.TestResource) bool { return true },
				HarmonizeImmutableFields: func(current *resources.TestResource, desired *corev1.Pod) {},
			},
			shouldErr: `ResourceManager "HarmonizeImmutableFields in 1" must implement HarmonizeImmutableFields: nil | func(*resources.TestResource, *resources.TestResource), found: func(*resources.TestResource, *v1.Pod)`,
		},
		{
			name: "HarmonizeImmutableFields num out",
			reconciler: &ResourceManager{
				Name:                     "HarmonizeImmutableFields num out",
				Type:                     &resources.TestResource{},
				MergeBeforeUpdate:        func(current, desired *resources.TestResource) {},
				SemanticEquals:           func(a1, a2 *resources.TestResource) bool { return true },
				HarmonizeImmutableFields: func(current, desired *resources.TestResource) error { return nil },
			},
			shouldErr: `ResourceManager "HarmonizeImmutableFields num out" must implement HarmonizeImmutableFields: nil | func(*resources.TestResource, *resources.TestResource), found: func(*resources.TestResource, *resources.TestResource) error`,
		},
		{
			name: "Sanitize",
			reconciler: &ResourceManager{
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				Sanitize:          func(child *resources.TestResource) resources.TestResourceSpec { return child.Spec },
			},
		},
		{
			name: "Sanitize num in",
			reconciler: &ResourceManager{
				Name:              "Sanitize num in",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				Sanitize:          func() resources.TestResourceSpec { return resources.TestResourceSpec{} },
			},
			shouldErr: `ResourceManager "Sanitize num in" must implement Sanitize: nil | func(*resources.TestResource) interface{}, found: func() resources.TestResourceSpec`,
		},
		{
			name: "Sanitize in 1",
			reconciler: &ResourceManager{
				Name:              "Sanitize in 1",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				Sanitize:          func(child *corev1.Pod) corev1.PodSpec { return child.Spec },
			},
			shouldErr: `ResourceManager "Sanitize in 1" must implement Sanitize: nil | func(*resources.TestResource) interface{}, found: func(*v1.Pod) v1.PodSpec`,
		},
		{
			name: "Sanitize num out",
			reconciler: &ResourceManager{
				Name:              "Sanitize num out",
				Type:              &resources.TestResource{},
				MergeBeforeUpdate: func(current, desired *resources.TestResource) {},
				SemanticEquals:    func(a1, a2 *resources.TestResource) bool { return true },
				Sanitize:          func(child *resources.TestResource) {},
			},
			shouldErr: `ResourceManager "Sanitize num out" must implement Sanitize: nil | func(*resources.TestResource) interface{}, found: func(*resources.TestResource)`,
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

/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"testing"

	"github.com/vmware-labs/reconciler-runtime/tracker"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSyncReconciler_validate(t *testing.T) {
	tests := []struct {
		name       string
		parent     client.Object
		reconciler *SyncReconciler
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &SyncReconciler{Name: "empty"},
			shouldErr:  `SyncReconciler "empty" must implement Sync`,
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
			},
		},
		{
			name:   "valid Sync with result",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) (ctrl.Result, error) {
					return ctrl.Result{}, nil
				},
			},
		},
		{
			name:   "Sync num in",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync num in",
				Sync: func() error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Sync num in" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func() error`,
		},
		{
			name:   "Sync in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync in 1",
				Sync: func(ctx context.Context, parent *corev1.Secret) error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Sync in 1" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.Secret) error`,
		},
		{
			name:   "Sync num out",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync num out",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) {
				},
			},
			shouldErr: `SyncReconciler "Sync num out" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap)`,
		},
		{
			name:   "Sync out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync out 1",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) string {
					return ""
				},
			},
			shouldErr: `SyncReconciler "Sync out 1" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) string`,
		},
		{
			name:   "Sync result out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Sync result out 1",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) (ctrl.Result, string) {
					return ctrl.Result{}, ""
				},
			},
			shouldErr: `SyncReconciler "Sync result out 1" must implement Sync: func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) (reconcile.Result, string)`,
		},
		{
			name:   "valid Finalize",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
			},
		},
		{
			name:   "valid Finalize with result",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, parent *corev1.ConfigMap) (ctrl.Result, error) {
					return ctrl.Result{}, nil
				},
			},
		},
		{
			name:   "Finalize num in",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize num in",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func() error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Finalize num in" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func() error`,
		},
		{
			name:   "Finalize in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize in 1",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, parent *corev1.Secret) error {
					return nil
				},
			},
			shouldErr: `SyncReconciler "Finalize in 1" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.Secret) error`,
		},
		{
			name:   "Finalize num out",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize num out",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, parent *corev1.ConfigMap) {
				},
			},
			shouldErr: `SyncReconciler "Finalize num out" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap)`,
		},
		{
			name:   "Finalize out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize out 1",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, parent *corev1.ConfigMap) string {
					return ""
				},
			},
			shouldErr: `SyncReconciler "Finalize out 1" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) string`,
		},
		{
			name:   "Finalize result out 1",
			parent: &corev1.ConfigMap{},
			reconciler: &SyncReconciler{
				Name: "Finalize result out 1",
				Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
					return nil
				},
				Finalize: func(ctx context.Context, parent *corev1.ConfigMap) (ctrl.Result, string) {
					return ctrl.Result{}, ""
				},
			},
			shouldErr: `SyncReconciler "Finalize result out 1" must implement Finalize: nil | func(context.Context, *v1.ConfigMap) error | func(context.Context, *v1.ConfigMap) (ctrl.Result, error), found: func(context.Context, *v1.ConfigMap) (reconcile.Result, string)`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashCastParentType(context.TODO(), c.parent)
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
			shouldErr:  "ChildType must be defined",
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
				// ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: "ChildType must be defined",
		},
		{
			name:   "ChildListType missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				ChildType: &corev1.Pod{},
				// ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: "ChildListType must be defined",
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
			name:   "MergeBeforeUpdate missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "MergeBeforeUpdate missing",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				// MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals: func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "MergeBeforeUpdate missing" must implement MergeBeforeUpdate`,
		},
		{
			name:   "MergeBeforeUpdate num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "MergeBeforeUpdate num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func() {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "MergeBeforeUpdate num in" must implement MergeBeforeUpdate: nil | func(*v1.Pod, *v1.Pod), found: func()`,
		},
		{
			name:   "MergeBeforeUpdate in 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "MergeBeforeUpdate in 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current *corev1.Secret, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "MergeBeforeUpdate in 0" must implement MergeBeforeUpdate: nil | func(*v1.Pod, *v1.Pod), found: func(*v1.Secret, *v1.Pod)`,
		},
		{
			name:   "MergeBeforeUpdate in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "MergeBeforeUpdate in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current *corev1.Pod, desired *corev1.Secret) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "MergeBeforeUpdate in 1" must implement MergeBeforeUpdate: nil | func(*v1.Pod, *v1.Pod), found: func(*v1.Pod, *v1.Secret)`,
		},
		{
			name:   "MergeBeforeUpdate num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "MergeBeforeUpdate num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) error { return nil },
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "MergeBeforeUpdate num out" must implement MergeBeforeUpdate: nil | func(*v1.Pod, *v1.Pod), found: func(*v1.Pod, *v1.Pod) error`,
		},
		{
			name:   "SemanticEquals missing",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SemanticEquals missing",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				// SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "SemanticEquals missing" must implement SemanticEquals`,
		},
		{
			name:   "SemanticEquals num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SemanticEquals num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func() bool { return false },
			},
			shouldErr: `ChildReconciler "SemanticEquals num in" must implement SemanticEquals: nil | func(*v1.Pod, *v1.Pod) bool, found: func() bool`,
		},
		{
			name:   "SemanticEquals in 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SemanticEquals in 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1 *corev1.Secret, a2 *corev1.Pod) bool { return false },
			},
			shouldErr: `ChildReconciler "SemanticEquals in 0" must implement SemanticEquals: nil | func(*v1.Pod, *v1.Pod) bool, found: func(*v1.Secret, *v1.Pod) bool`,
		},
		{
			name:   "SemanticEquals in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SemanticEquals in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1 *corev1.Pod, a2 *corev1.Secret) bool { return false },
			},
			shouldErr: `ChildReconciler "SemanticEquals in 1" must implement SemanticEquals: nil | func(*v1.Pod, *v1.Pod) bool, found: func(*v1.Pod, *v1.Secret) bool`,
		},
		{
			name:   "SemanticEquals num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SemanticEquals num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "SemanticEquals num out" must implement SemanticEquals: nil | func(*v1.Pod, *v1.Pod) bool, found: func(*v1.Pod, *v1.Pod)`,
		},
		{
			name:   "SemanticEquals out 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "SemanticEquals out 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) error { return nil },
			},
			shouldErr: `ChildReconciler "SemanticEquals out 0" must implement SemanticEquals: nil | func(*v1.Pod, *v1.Pod) bool, found: func(*v1.Pod, *v1.Pod) error`,
		},
		{
			name:   "HarmonizeImmutableFields",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				HarmonizeImmutableFields:   func(current, desired *corev1.Pod) {},
			},
		},
		{
			name:   "HarmonizeImmutableFields num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "HarmonizeImmutableFields num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				HarmonizeImmutableFields:   func() {},
			},
			shouldErr: `ChildReconciler "HarmonizeImmutableFields num in" must implement HarmonizeImmutableFields: nil | func(*v1.Pod, *v1.Pod), found: func()`,
		},
		{
			name:   "HarmonizeImmutableFields in 0",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "HarmonizeImmutableFields in 0",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				HarmonizeImmutableFields:   func(current *corev1.Secret, desired *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "HarmonizeImmutableFields in 0" must implement HarmonizeImmutableFields: nil | func(*v1.Pod, *v1.Pod), found: func(*v1.Secret, *v1.Pod)`,
		},
		{
			name:   "HarmonizeImmutableFields in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "HarmonizeImmutableFields in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				HarmonizeImmutableFields:   func(current *corev1.Pod, desired *corev1.Secret) {},
			},
			shouldErr: `ChildReconciler "HarmonizeImmutableFields in 1" must implement HarmonizeImmutableFields: nil | func(*v1.Pod, *v1.Pod), found: func(*v1.Pod, *v1.Secret)`,
		},
		{
			name:   "HarmonizeImmutableFields num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "HarmonizeImmutableFields num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				HarmonizeImmutableFields:   func(current, desired *corev1.Pod) error { return nil },
			},
			shouldErr: `ChildReconciler "HarmonizeImmutableFields num out" must implement HarmonizeImmutableFields: nil | func(*v1.Pod, *v1.Pod), found: func(*v1.Pod, *v1.Pod) error`,
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
		{
			name:   "Sanitize",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				Sanitize:                   func(child *corev1.Pod) corev1.PodSpec { return child.Spec },
			},
		},
		{
			name:   "Sanitize num in",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "Sanitize num in",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				Sanitize:                   func() corev1.PodSpec { return corev1.PodSpec{} },
			},
			shouldErr: `ChildReconciler "Sanitize num in" must implement Sanitize: nil | func(*v1.Pod) interface{}, found: func() v1.PodSpec`,
		},
		{
			name:   "Sanitize in 1",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "Sanitize in 1",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				Sanitize:                   func(child *corev1.Secret) corev1.PodSpec { return corev1.PodSpec{} },
			},
			shouldErr: `ChildReconciler "Sanitize in 1" must implement Sanitize: nil | func(*v1.Pod) interface{}, found: func(*v1.Secret) v1.PodSpec`,
		},
		{
			name:   "Sanitize num out",
			parent: &corev1.ConfigMap{},
			reconciler: &ChildReconciler{
				Name:                       "Sanitize num out",
				ChildType:                  &corev1.Pod{},
				ChildListType:              &corev1.PodList{},
				DesiredChild:               func(ctx context.Context, parent *corev1.ConfigMap) (*corev1.Pod, error) { return nil, nil },
				ReflectChildStatusOnParent: func(parent *corev1.ConfigMap, child *corev1.Pod, err error) {},
				MergeBeforeUpdate:          func(current, desired *corev1.Pod) {},
				SemanticEquals:             func(a1, a2 *corev1.Pod) bool { return false },
				Sanitize:                   func(child *corev1.Pod) {},
			},
			shouldErr: `ChildReconciler "Sanitize num out" must implement Sanitize: nil | func(*v1.Pod) interface{}, found: func(*v1.Pod)`,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashCastParentType(context.TODO(), c.parent)
			err := c.reconciler.validate(ctx)
			if (err != nil) != (c.shouldErr != "") || (c.shouldErr != "" && c.shouldErr != err.Error()) {
				t.Errorf("validate() error = %q, shouldErr %q", err.Error(), c.shouldErr)
			}
		})
	}
}

func TestCastParent_validate(t *testing.T) {
	tests := []struct {
		name       string
		parent     client.Object
		reconciler *CastParent
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &CastParent{},
			shouldErr:  "Type must be defined",
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &CastParent{
				Type: &corev1.Secret{},
				Reconciler: &SyncReconciler{
					Sync: func(ctx context.Context, parent *corev1.Secret) error {
						return nil
					},
				},
			},
		},
		{
			name:   "missing type",
			parent: &corev1.ConfigMap{},
			reconciler: &CastParent{
				Type: nil,
				Reconciler: &SyncReconciler{
					Sync: func(ctx context.Context, parent *corev1.Secret) error {
						return nil
					},
				},
			},
			shouldErr: "Type must be defined",
		},
		{
			name:   "missing reconciler",
			parent: &corev1.ConfigMap{},
			reconciler: &CastParent{
				Type:       &corev1.Secret{},
				Reconciler: nil,
			},
			shouldErr: "Reconciler must be defined",
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := StashCastParentType(context.TODO(), c.parent)
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
		parent     client.Object
		reconciler *WithConfig
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &WithConfig{},
			shouldErr:  "Config must be defined",
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &WithConfig{
				Reconciler: &Sequence{},
				Config: func(ctx context.Context, c Config) (Config, error) {
					return config, nil
				},
			},
		},
		{
			name:   "missing config",
			parent: &corev1.ConfigMap{},
			reconciler: &WithConfig{
				Reconciler: &Sequence{},
			},
			shouldErr: "Config must be defined",
		},
		{
			name:   "missing reconciler",
			parent: &corev1.ConfigMap{},
			reconciler: &WithConfig{
				Config: func(ctx context.Context, c Config) (Config, error) {
					return config, nil
				},
			},
			shouldErr: "Reconciler must be defined",
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
		parent     client.Object
		reconciler *WithFinalizer
		shouldErr  string
	}{
		{
			name:       "empty",
			parent:     &corev1.ConfigMap{},
			reconciler: &WithFinalizer{},
			shouldErr:  "Finalizer must be defined",
		},
		{
			name:   "valid",
			parent: &corev1.ConfigMap{},
			reconciler: &WithFinalizer{
				Reconciler: &Sequence{},
				Finalizer:  "my-finalizer",
			},
		},
		{
			name:   "missing finalizer",
			parent: &corev1.ConfigMap{},
			reconciler: &WithFinalizer{
				Reconciler: &Sequence{},
			},
			shouldErr: "Finalizer must be defined",
		},
		{
			name:   "missing reconciler",
			parent: &corev1.ConfigMap{},
			reconciler: &WithFinalizer{
				Finalizer: "my-finalizer",
			},
			shouldErr: "Reconciler must be defined",
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

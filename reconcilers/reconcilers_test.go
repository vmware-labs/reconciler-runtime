/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/testing/factories"
	ftesting "github.com/vmware-labs/reconciler-runtime/testing/factorytesting"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestParentReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}

	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)

	resource := factories.TestResource().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om factories.ObjectMeta) {
			om.Created(1)
		}).
		StatusConditions(
			factories.Condition().Type(apis.ConditionReady).Unknown(),
		)

	rts := rtesting.ReconcilerTestSuite{{
		Name: "resource does not exist",
		Key:  testKey,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						t.Error("should not be called")
						return nil
					},
				}
			},
		},
	}, {
		Name: "ignore deleted resource",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource.ObjectMeta(func(om factories.ObjectMeta) {
				om.Deleted(1)
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						t.Error("should not be called")
						return nil
					},
				}
			},
		},
	}, {
		Name: "error fetching resource",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource.ObjectMeta(func(om factories.ObjectMeta) {
				om.Deleted(1)
			}),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "TestResource"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						t.Error("should not be called")
						return nil
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "resource is defaulted",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						if expected, actual := "ran", parent.Spec.Fields["Defaulter"]; expected != actual {
							t.Errorf("unexpected default value, actually = %v, expected = %v", expected, actual)
						}
						return nil
					},
				}
			},
		},
	}, {
		Name: "status conditions are initialized",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource.StatusConditions(),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						expected := apis.Conditions{
							{Type: apis.ConditionReady, Status: corev1.ConditionUnknown},
						}
						if diff := cmp.Diff(expected, parent.Status.Conditions, rtesting.IgnoreLastTransitionTime); diff != "" {
							t.Errorf("Unexpected condition (-expected, +actual): %s", diff)
						}
						return nil
					},
				}
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "StatusUpdated",
				`Updated status`),
		},
		ExpectStatusUpdates: []ftesting.Factory{
			resource,
		},
	}, {
		Name: "reconciler mutated status",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						if parent.Status.Fields == nil {
							parent.Status.Fields = map[string]string{}
						}
						parent.Status.Fields["Reconciler"] = "ran"
						return nil
					},
				}
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "StatusUpdated",
				`Updated status`),
		},
		ExpectStatusUpdates: []ftesting.Factory{
			resource.AddStatusField("Reconciler", "ran"),
		},
	}, {
		Name: "sub reconciler erred",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						return fmt.Errorf("reconciler error")
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "status update failed",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "TestResource", rtesting.InduceFailureOpts{
				SubResource: "status",
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						if parent.Status.Fields == nil {
							parent.Status.Fields = map[string]string{}
						}
						parent.Status.Fields["Reconciler"] = "ran"
						return nil
					},
				}
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "StatusUpdateFailed",
				`Failed to update status: inducing failure for update TestResource`),
		},
		ExpectStatusUpdates: []ftesting.Factory{
			resource.AddStatusField("Reconciler", "ran"),
		},
		ShouldErr: true,
	}, {
		Name: "context is stashable",
		Key:  testKey,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						var key reconcilers.StashKey = "foo"
						// StashValue will panic if the context is not setup correctly
						reconcilers.StashValue(ctx, key, "bar")
						return nil
					},
				}
			},
		},
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ParentReconciler{
			Type:       &rtesting.TestResource{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestSyncReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)

	resource := factories.TestResource().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om factories.ObjectMeta) {
			om.Created(1)
		}).
		StatusConditions(
			factories.Condition().Type(apis.ConditionReady).Unknown(),
		)

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "sync success",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						return nil
					},
				}
			},
		},
	}, {
		Name:   "sync error",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
						return fmt.Errorf("syncreconciler error")
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name:   "missing sync method",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync:   nil,
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name:   "invalid sync signature",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent string) error {
						return nil
					},
				}
			},
		},
		ShouldPanic: true,
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

func TestChildReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	resource := factories.TestResource().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om factories.ObjectMeta) {
			om.Created(1)
		}).
		StatusConditions(
			factories.Condition().Type(apis.ConditionReady).Unknown(),
		)
	resourceReady := resource.
		StatusConditions(
			factories.Condition().Type(apis.ConditionReady).True(),
		)

	configMapCreate := factories.ConfigMap().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om factories.ObjectMeta) {
			om.ControlledBy(resource, scheme)
		}).
		AddData("foo", "bar")
	configMapGiven := configMapCreate.
		ObjectMeta(func(om factories.ObjectMeta) {
			om.Created(1)
		})

	defaultChildReconciler := func(c reconcilers.Config) *reconcilers.ChildReconciler {
		return &reconcilers.ChildReconciler{
			ChildType:     &corev1.ConfigMap{},
			ChildListType: &corev1.ConfigMapList{},

			DesiredChild: func(ctx context.Context, parent *rtesting.TestResource) (*corev1.ConfigMap, error) {
				if len(parent.Spec.Fields) == 0 {
					return nil, nil
				}

				return &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: parent.Namespace,
						Name:      parent.Name,
					},
					Data: reconcilers.MergeMaps(parent.Spec.Fields),
				}, nil
			},
			MergeBeforeUpdate: func(current, desired *corev1.ConfigMap) {
				current.Data = desired.Data
			},
			ReflectChildStatusOnParent: func(parent *rtesting.TestResource, child *corev1.ConfigMap, err error) {
				if err != nil {
					if apierrs.IsAlreadyExists(err) {
						name := err.(apierrs.APIStatus).Status().Details.Name
						parent.Status.MarkNotReady("NameConflict", "%q already exists", name)
					}
					return
				}
				if child == nil {
					parent.Status.Fields = nil
					parent.Status.MarkReady()
					return
				}
				parent.Status.Fields = reconcilers.MergeMaps(child.Data)
				parent.Status.MarkReady()
			},
			SemanticEquals: func(r1, r2 *corev1.ConfigMap) bool {
				return equality.Semantic.DeepEqual(r1.Data, r2.Data)
			},

			Config:     c,
			IndexField: ".metadata.testResourceController",
		}
	}

	rts := rtesting.SubReconcilerTestSuite{{
		Name:         "preserve no child",
		Parent:       resourceReady,
		GivenObjects: []ftesting.Factory{},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
	}, {
		Name: "child is in sync",
		Parent: resourceReady.
			AddField("foo", "bar").
			AddStatusField("foo", "bar"),
		GivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
	}, {
		Name: "create child",
		Parent: resource.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			AddField("foo", "bar").
			AddStatusField("foo", "bar"),
		ExpectCreates: []ftesting.Factory{
			configMapCreate,
		},
	}, {
		Name: "update child",
		Parent: resourceReady.
			AddField("foo", "bar").
			AddField("new", "field").
			AddStatusField("foo", "bar"),
		GivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Updated",
				`Updated ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			AddField("foo", "bar").
			AddField("new", "field").
			AddStatusField("foo", "bar").
			AddStatusField("new", "field"),
		ExpectUpdates: []ftesting.Factory{
			configMapGiven.
				AddData("new", "field"),
		},
	}, {
		Name:   "delete child",
		Parent: resourceReady,
		GivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "", Kind: "ConfigMap", Namespace: testNamespace, Name: testName},
		},
	}, {
		Name: "delete duplicate children",
		Parent: resource.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{
			configMapGiven.
				NamespaceName(testNamespace, "extra-child-1"),
			configMapGiven.
				NamespaceName(testNamespace, "extra-child-2"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectParent: resourceReady.
			AddField("foo", "bar").
			AddStatusField("foo", "bar"),
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, "extra-child-1"),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, "extra-child-2"),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "", Kind: "ConfigMap", Namespace: testNamespace, Name: "extra-child-1"},
			{Group: "", Kind: "ConfigMap", Namespace: testNamespace, Name: "extra-child-2"},
		},
		ExpectCreates: []ftesting.Factory{
			configMapCreate,
		},
	}, {
		Name: "child name collision",
		Parent: resourceReady.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
				Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectParent: resourceReady.
			AddField("foo", "bar").
			StatusConditions(
				factories.Condition().Type(apis.ConditionReady).False().
					Reason("NameConflict", `"test-resource" already exists`),
			),
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create ConfigMap %q:  %q already exists", testName, testName),
		},
		ExpectCreates: []ftesting.Factory{
			configMapCreate,
		},
	}, {
		Name: "child name collision, stale informer cache",
		Parent: resourceReady.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{},
		APIGivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
				Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create ConfigMap %q:  %q already exists", testName, testName),
		},
		ExpectCreates: []ftesting.Factory{
			configMapCreate,
		},
		ShouldErr: true,
	}, {
		Name: "preserve immutable fields",
		Parent: resourceReady.
			AddField("foo", "bar").
			AddStatusField("foo", "bar").
			AddStatusField("immutable", "field"),
		GivenObjects: []ftesting.Factory{
			configMapGiven.
				AddData("immutable", "field"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.HarmonizeImmutableFields = func(current, desired *corev1.ConfigMap) {
					desired.Data["immutable"] = current.Data["immutable"]
				}
				return r
			},
		},
	}, {
		Name:   "status only reconcile",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		ExpectParent: resourceReady.
			AddStatusField("foo", "bar"),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.DesiredChild = func(ctx context.Context, parent *rtesting.TestResource) (*corev1.ConfigMap, error) {
					return nil, reconcilers.OnlyReconcileChildStatus
				}
				return r
			},
		},
	}, {
		Name: "sanitize child before logging",
		Parent: resource.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Sanitize = func(child *corev1.ConfigMap) interface{} {
					return child.Name
				}
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			AddField("foo", "bar").
			AddStatusField("foo", "bar"),
		ExpectCreates: []ftesting.Factory{
			configMapCreate,
		},
	}, {
		Name:         "error listing children",
		Parent:       resourceReady,
		GivenObjects: []ftesting.Factory{},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "ConfigMapList"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ShouldErr: true,
	}, {
		Name: "error creating child",
		Parent: resource.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
				`Failed to create ConfigMap %q: inducing failure for create ConfigMap`, testName),
		},
		ExpectCreates: []ftesting.Factory{
			configMapCreate,
		},
		ShouldErr: true,
	}, {
		Name: "error updating child",
		Parent: resourceReady.
			AddField("foo", "bar").
			AddField("new", "field").
			AddStatusField("foo", "bar"),
		GivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "UpdateFailed",
				`Failed to update ConfigMap %q: inducing failure for update ConfigMap`, testName),
		},
		ExpectUpdates: []ftesting.Factory{
			configMapGiven.
				AddData("new", "field"),
		},
		ShouldErr: true,
	}, {
		Name:   "error deleting child",
		Parent: resourceReady,
		GivenObjects: []ftesting.Factory{
			configMapGiven,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "DeleteFailed",
				`Failed to delete ConfigMap %q: inducing failure for delete ConfigMap`, testName),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "", Kind: "ConfigMap", Namespace: testNamespace, Name: testName},
		},
		ShouldErr: true,
	}, {
		Name: "error deleting duplicate children",
		Parent: resource.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{
			configMapGiven.
				NamespaceName(testNamespace, "extra-child-1"),
			configMapGiven.
				NamespaceName(testNamespace, "extra-child-2"),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "DeleteFailed",
				`Failed to delete ConfigMap %q: inducing failure for delete ConfigMap`, "extra-child-1"),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "", Kind: "ConfigMap", Namespace: testNamespace, Name: "extra-child-1"},
		},
		ShouldErr: true,
	}, {
		Name:         "error creating desired child",
		Parent:       resource,
		GivenObjects: []ftesting.Factory{},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.DesiredChild = func(ctx context.Context, parent *rtesting.TestResource) (*corev1.ConfigMap, error) {
					return nil, fmt.Errorf("test error")
				}
				return r
			},
		},
		ShouldErr: true,
	}, {
		Name: "error empty scheme",
		Parent: resource.
			AddField("foo", "bar"),
		GivenObjects: []ftesting.Factory{},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				scheme := runtime.NewScheme()
				r.Config.Scheme = scheme
				return r
			},
		},
		ShouldErr: true,
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

func TestSequence(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)

	resource := factories.TestResource().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om factories.ObjectMeta) {
			om.Created(1)
		}).
		StatusConditions(
			factories.Condition().Type(apis.ConditionReady).Unknown(),
		)

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "sub reconciler erred",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
							return fmt.Errorf("reconciler error")
						},
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name:   "preserves result, Requeue",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
						return ctrl.Result{Requeue: true}, nil
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:   "preserves result, RequeueAfter",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:   "ignores result on err",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, fmt.Errorf("test error")
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{},
		ShouldErr:      true,
	}, {
		Name:   "Requeue + empty => Requeue",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:   "empty + Requeue => Requeue",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:   "RequeueAfter + empty => RequeueAfter",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:   "empty + RequeueAfter => RequeueAfter",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:   "RequeueAfter + Requeue => RequeueAfter",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:   "Requeue + RequeueAfter => RequeueAfter",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:   "RequeueAfter(1m) + RequeueAfter(2m) => RequeueAfter(1m)",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:   "RequeueAfter(2m) + RequeueAfter(1m) => RequeueAfter(1m)",
		Parent: resource,
		GivenObjects: []ftesting.Factory{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Config: c,
						Sync: func(ctx context.Context, parent *rtesting.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

func TestCastParent(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)

	resource := factories.TestResource().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om factories.ObjectMeta) {
			om.Created(1)
		}).
		StatusConditions(
			factories.Condition().Type(apis.ConditionReady).Unknown(),
		)

	rts := rtesting.SubReconcilerTestSuite{{
		Name: "sync success",
		Parent: resource.PodTemplateSpec(func(pts factories.PodTemplateSpec) {
			pts.ContainerNamed("test-container", func(c *corev1.Container) {})
		}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *appsv1.Deployment) error {
							c.Recorder.Event(resource.Create(), corev1.EventTypeNormal, "Test",
								parent.Spec.Template.Spec.Containers[0].Name)
							return nil
						},
						Config: c,
					},
				}
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Test", "test-container"),
		},
	}, {
		Name:   "cast mutation",
		Parent: resource,
		ExpectParent: resource.PodTemplateSpec(func(pts factories.PodTemplateSpec) {
			pts.ObjectMeta(func(om factories.ObjectMeta) {
				om.Name("mutation")
			})
		}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *appsv1.Deployment) error {
							// mutation that exists on the original parent and will be reflected
							parent.Spec.Template.Name = "mutation"
							// mutation that does not exists on the original parent and will be dropped
							parent.Spec.Paused = true
							return nil
						},
						Config: c,
					},
				}
			},
		},
	}, {
		Name:   "return subreconciler result",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *appsv1.Deployment) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
						Config: c,
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:   "return subreconciler err",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *appsv1.Deployment) error {
							return fmt.Errorf("subreconciler error")
						},
						Config: c,
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "subreconcilers must be compatible with cast value, not parent",
		Parent: resource.PodTemplateSpec(func(pts factories.PodTemplateSpec) {
			pts.ContainerNamed("test-container", func(c *corev1.Container) {})
		}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
							return nil
						},
						Config: c,
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name: "error for cast to different type than expected by sub reconciler",
		Parent: resource.PodTemplateSpec(func(pts factories.PodTemplateSpec) {
			pts.ContainerNamed("test-container", func(c *corev1.Container) {})
		}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
							return nil
						},
						Config: c,
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name:   "marshal error",
		Parent: resource.ErrorOn(true, false),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &rtesting.TestResource{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
							c.Recorder.Event(resource.Create(), corev1.EventTypeNormal, "Test", parent.Name)
							return nil
						},
						Config: c,
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name:   "unmarshal error",
		Parent: resource.ErrorOn(false, true),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &rtesting.TestResource{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *rtesting.TestResource) error {
							c.Recorder.Event(resource.Create(), corev1.EventTypeNormal, "Test", parent.Name)
							return nil
						},
						Config: c,
					},
				}
			},
		},
		ShouldErr: true,
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

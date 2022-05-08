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

	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/internal/resources/dies"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/tracker"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestParentReconcilerWithNoStatus(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource-no-status"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceNoStatusBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
			d.AddAnnotation("blah", "blah")
		})

	rts := rtesting.ReconcilerTestSuite{{
		Name: "resource exists",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResourceNoStatus) error {
						return nil
					},
				}
			},
		},
	}}
	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ParentReconciler{
			Type:       &resources.TestResourceNoStatus{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestParentReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})
	deletedAt := metav1.NewTime(time.UnixMilli(2000))

	rts := rtesting.ReconcilerTestSuite{{
		Name: "resource does not exist",
		Key:  testKey,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						t.Error("should not be called")
						return nil
					},
				}
			},
		},
	}, {
		Name: "ignore deleted resource",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&deletedAt)
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						t.Error("should not be called")
						return nil
					},
				}
			},
		},
	}, {
		Name: "error fetching resource",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "TestResource"),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
		GivenObjects: []client.Object{
			resource.StatusDie(func(d *dies.TestResourceStatusDie) {
				d.ConditionsDie()
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						expected := []metav1.Condition{
							{Type: apis.ConditionReady, Status: metav1.ConditionUnknown, Reason: "Initializing"},
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
		ExpectStatusUpdates: []client.Object{
			resource,
		},
	}, {
		Name: "reconciler mutated status",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
		ExpectStatusUpdates: []client.Object{
			resource.StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("Reconciler", "ran")
			}),
		},
	}, {
		Name: "sub reconciler erred",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						return fmt.Errorf("reconciler error")
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "status update failed",
		Key:  testKey,
		GivenObjects: []client.Object{
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
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
		ExpectStatusUpdates: []client.Object{
			resource.StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("Reconciler", "ran")
			}),
		},
		ShouldErr: true,
	}, {
		Name: "context is stashable",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						var key reconcilers.StashKey = "foo"
						// StashValue will panic if the context is not setup correctly
						reconcilers.StashValue(ctx, key, "bar")
						return nil
					},
				}
			},
		},
	}, {
		Name: "context has config",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						if config := reconcilers.RetrieveConfig(ctx); config != c {
							t.Errorf("expected config in context, found %#v", config)
						}
						if parentConfig := reconcilers.RetrieveParentConfig(ctx); parentConfig != c {
							t.Errorf("expected parent config in context, found %#v", parentConfig)
						}
						return nil
					},
				}
			},
		},
	}, {
		Name: "context has parent type",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						if parentType, ok := reconcilers.RetrieveParentType(ctx).(*resources.TestResource); !ok {
							t.Errorf("expected parent type not in context, found %#v", parentType)
						}
						if castParentType, ok := reconcilers.RetrieveCastParentType(ctx).(*resources.TestResource); !ok {
							t.Errorf("expected cast parent type not in context, found %#v", castParentType)
						}
						return nil
					},
				}
			},
		},
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ParentReconciler{
			Type:       &resources.TestResource{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestSyncReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	now := metav1.Now()

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "sync success",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
					Sync: nil,
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
					Sync: func(ctx context.Context, parent string) error {
						return nil
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name:   "should not finalize non-deleted resources",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						t.Errorf("reconciler should not call finalize for non-deleted resources")
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
	}, {
		Name: "should finalize deleted resources",
		Parent: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						t.Errorf("reconciler should not call sync for deleted resources")
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 3 * time.Hour},
	}, {
		Name: "should finalize and sync deleted resources when asked to",
		Parent: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					SyncDuringFinalization: true,
					Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
	}, {
		Name: "should finalize and sync deleted resources when asked to, shorter resync wins",
		Parent: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					SyncDuringFinalization: true,
					Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
	}, {
		Name: "finalize is optional",
		Parent: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						return nil
					},
				}
			},
		},
	}, {
		Name: "finalize error",
		Parent: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) error {
						return nil
					},
					Finalize: func(ctx context.Context, parent *resources.TestResource) error {
						return fmt.Errorf("syncreconciler finalize error")
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

func TestChildReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testFinalizer := "test.finalizer"

	now := metav1.NewTime(time.Now().Truncate(time.Second))

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})
	resourceReady := resource.
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionTrue).Reason("Ready"),
			)
		})

	configMapCreate := diecorev1.ConfigMapBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.ControlledBy(resource, scheme)
		}).
		AddData("foo", "bar")
	configMapGiven := configMapCreate.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		})

	defaultChildReconciler := func(c reconcilers.Config) *reconcilers.ChildReconciler {
		return &reconcilers.ChildReconciler{
			ChildType:     &corev1.ConfigMap{},
			ChildListType: &corev1.ConfigMapList{},

			DesiredChild: func(ctx context.Context, parent *resources.TestResource) (*corev1.ConfigMap, error) {
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
			ReflectChildStatusOnParent: func(parent *resources.TestResource, child *corev1.ConfigMap, err error) {
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
		}
	}

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "preserve no child",
		Parent: resourceReady,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
	}, {
		Name: "child is in sync",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
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
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
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
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		ExpectCreates: []client.Object{
			configMapCreate,
		},
	}, {
		Name: "create child with finalizer",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
				d.ResourceVersion("1000")
			}).
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		ExpectCreates: []client.Object{
			configMapCreate,
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":["test.finalizer"],"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name: "update child",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
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
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}),
		ExpectUpdates: []client.Object{
			configMapGiven.
				AddData("new", "field"),
		},
	}, {
		Name: "update child, preserve finalizers",
		Parent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer, "some.other.finalizer")
			}).
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Updated",
				`Updated ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer, "some.other.finalizer")
			}).
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}),
		ExpectUpdates: []client.Object{
			configMapGiven.
				AddData("new", "field"),
		},
	}, {
		Name: "update child, restoring missing finalizer",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Updated",
				`Updated ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
				d.ResourceVersion("1000")
			}).
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}),
		ExpectUpdates: []client.Object{
			configMapGiven.
				AddData("new", "field"),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":["test.finalizer"],"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name:   "delete child",
		Parent: resourceReady,
		GivenObjects: []client.Object{
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
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
	}, {
		Name: "delete child, clearing finalizer",
		Parent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
			}),
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
		},
		ExpectParent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers()
				d.ResourceVersion("1000")
			}),
		ExpectDeletes: []rtesting.DeleteRef{
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":null,"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name: "delete child, preserve other finalizer",
		Parent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers("some.other.finalizer")
			}),
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
	}, {
		Name:   "ignore extraneous children",
		Parent: resourceReady,
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool {
					return false
				}
				return r
			},
		},
	}, {
		Name: "delete duplicate children",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Name("extra-child-1")
				}),
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Name("extra-child-2")
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
		ExpectParent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
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
		ExpectCreates: []client.Object{
			configMapCreate,
		},
	}, {
		Name: "delete child durring finalization",
		Parent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
				d.Finalizers(testFinalizer)
			}),
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
		},
		ExpectParent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
				d.Finalizers()
				d.ResourceVersion("1000")
			}),
		ExpectDeletes: []rtesting.DeleteRef{
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":null,"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name: "child name collision",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
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
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.ConditionsDie(
					diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionFalse).
						Reason("NameConflict").Message(`"test-resource" already exists`),
				)
			}),
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create ConfigMap %q:  %q already exists", testName, testName),
		},
		ExpectCreates: []client.Object{
			configMapCreate,
		},
	}, {
		Name: "child name collision, stale informer cache",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		APIGivenObjects: []client.Object{
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
		ExpectCreates: []client.Object{
			configMapCreate,
		},
		ShouldErr: true,
	}, {
		Name: "preserve immutable fields",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
				d.AddField("immutable", "field")
			}),
		GivenObjects: []client.Object{
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
		GivenObjects: []client.Object{
			configMapGiven,
		},
		ExpectParent: resourceReady.
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.DesiredChild = func(ctx context.Context, parent *resources.TestResource) (*corev1.ConfigMap, error) {
					return nil, reconcilers.OnlyReconcileChildStatus
				}
				return r
			},
		},
	}, {
		Name: "sanitize child before logging",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
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
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		ExpectCreates: []client.Object{
			configMapCreate,
		},
	}, {
		Name: "sanitize is mutation safe",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Sanitize = func(child *corev1.ConfigMap) interface{} {
					child.Data["ignore"] = "me"
					return child
				}
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectParent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		ExpectCreates: []client.Object{
			configMapCreate,
		},
	}, {
		Name:   "error listing children",
		Parent: resourceReady,
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
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
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
		ExpectCreates: []client.Object{
			configMapCreate,
		},
		ShouldErr: true,
	}, {
		Name: "error adding finalizer",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("patch", "TestResource"),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "FinalizerPatchFailed",
				`Failed to patch finalizer %q: inducing failure for patch TestResource`, testFinalizer),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":["test.finalizer"],"resourceVersion":"999"}}`),
			},
		},
		ShouldErr: true,
	}, {
		Name: "error clearing finalizer",
		Parent: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
			}),
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				return r
			},
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("patch", "TestResource"),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "FinalizerPatchFailed",
				`Failed to patch finalizer %q: inducing failure for patch TestResource`, testFinalizer),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":null,"resourceVersion":"999"}}`),
			},
		},
		ShouldErr: true,
	}, {
		Name: "error updating child",
		Parent: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
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
		ExpectUpdates: []client.Object{
			configMapGiven.
				AddData("new", "field"),
		},
		ShouldErr: true,
	}, {
		Name:   "error deleting child",
		Parent: resourceReady,
		GivenObjects: []client.Object{
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
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
		ShouldErr: true,
	}, {
		Name: "error deleting duplicate children",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Name("extra-child-1")
				}),
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Name("extra-child-2")
				}),
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
		Name:   "error creating desired child",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.DesiredChild = func(ctx context.Context, parent *resources.TestResource) (*corev1.ConfigMap, error) {
					return nil, fmt.Errorf("test error")
				}
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
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "sub reconciler erred",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{Requeue: true}, nil
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:   "preserves result, RequeueAfter",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) (ctrl.Result, error) {
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
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name: "sync success",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.SpecDie(func(d *diecorev1.PodSpecDie) {
						d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *appsv1.Deployment) error {
							c.Recorder.Event(resource, corev1.EventTypeNormal, "Test",
								parent.Spec.Template.Spec.Containers[0].Name)
							return nil
						},
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
		ExpectParent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Name("mutation")
					})
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
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "subreconcilers must be compatible with cast value, not parent",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.SpecDie(func(d *diecorev1.PodSpecDie) {
						d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
							return nil
						},
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name: "error for cast to different type than expected by sub reconciler",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.SpecDie(func(d *diecorev1.PodSpecDie) {
						d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
							return nil
						},
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name: "marshal error",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.ErrOnMarshal(true)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &resources.TestResource{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
							c.Recorder.Event(resource, corev1.EventTypeNormal, "Test", parent.Name)
							return nil
						},
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "unmarshal error",
		Parent: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.ErrOnUnmarshal(true)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastParent{
					Type: &resources.TestResource{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
							c.Recorder.Event(resource, corev1.EventTypeNormal, "Test", parent.Name)
							return nil
						},
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

func TestWithConfig(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "with config",
		Parent: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, pc reconcilers.Config) reconcilers.SubReconciler {
				c := reconcilers.Config{
					Tracker: tracker.New(0),
				}

				return &reconcilers.WithConfig{
					Config: func(ctx context.Context, _ reconcilers.Config) (reconcilers.Config, error) {
						return c, nil
					},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
							ac := reconcilers.RetrieveConfig(ctx)
							apc := reconcilers.RetrieveParentConfig(ctx)

							if ac != c {
								t.Errorf("unexpected config")
							}
							if apc != pc {
								t.Errorf("unexpected parent config")
							}

							pc.Recorder.Event(resource, corev1.EventTypeNormal, "AllGood", "")

							return nil
						},
					},
				}
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "AllGood", ""),
		},
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

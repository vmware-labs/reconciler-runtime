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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestConfig_TrackAndGet(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		})

	configMap := diecorev1.ConfigMapBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("track-namespace")
			d.Name("track-name")
		}).
		AddData("greeting", "hello")

	rts := rtesting.SubReconcilerTestSuite{{
		Name:     "track and get",
		Resource: resource,
		GivenObjects: []client.Object{
			configMap,
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(configMap, resource, scheme),
		},
	}, {
		Name:      "track with not found get",
		Resource:  resource,
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(configMap, resource, scheme),
		},
	}}

	// run with typed objects
	t.Run("typed", func(t *testing.T) {
		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
			return &reconcilers.SyncReconciler{
				Sync: func(ctx context.Context, resource *resources.TestResource) error {
					c := reconcilers.RetrieveConfigOrDie(ctx)

					cm := &corev1.ConfigMap{}
					err := c.TrackAndGet(ctx, types.NamespacedName{Namespace: "track-namespace", Name: "track-name"}, cm)
					if err != nil {
						return err
					}

					if expected, actual := "hello", cm.Data["greeting"]; expected != actual {
						// should never get here
						panic(fmt.Errorf("expected configmap to have greeting %q, found %q", expected, actual))
					}
					return nil
				},
			}
		})
	})

	// run with unstructured objects
	t.Run("unstructured", func(t *testing.T) {
		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
			return &reconcilers.SyncReconciler{
				Sync: func(ctx context.Context, resource *resources.TestResource) error {
					c := reconcilers.RetrieveConfigOrDie(ctx)

					cm := &unstructured.Unstructured{}
					cm.SetAPIVersion("v1")
					cm.SetKind("ConfigMap")
					err := c.TrackAndGet(ctx, types.NamespacedName{Namespace: "track-namespace", Name: "track-name"}, cm)
					if err != nil {
						return err
					}

					if expected, actual := "hello", cm.UnstructuredContent()["data"].(map[string]interface{})["greeting"].(string); expected != actual {
						// should never get here
						panic(fmt.Errorf("expected configmap to have greeting %q, found %q", expected, actual))
					}
					return nil
				},
			}
		})
	})
}

func TestResourceReconciler_NoStatus(t *testing.T) {
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
					Sync: func(ctx context.Context, resource *resources.TestResourceNoStatus) error {
						return nil
					},
				}
			},
		},
	}}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler{
			Type:       &resources.TestResourceNoStatus{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler_EmptyStatus(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource-empty-status"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceEmptyStatusBlank.
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
					Sync: func(ctx context.Context, resource *resources.TestResourceEmptyStatus) error {
						return nil
					},
				}
			},
		},
	}}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler{
			Type:       &resources.TestResourceEmptyStatus{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler_NilableStatus(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceNilableStatusBlank.
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

	rts := rtesting.ReconcilerTestSuite{{
		Name: "nil status",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource.Status(nil),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResourceNilableStatus) error {
						if resource.Status != nil {
							t.Errorf("status expected to be nil")
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
					Sync: func(ctx context.Context, resource *resources.TestResourceNilableStatus) error {
						expected := []metav1.Condition{
							{Type: apis.ConditionReady, Status: metav1.ConditionUnknown, Reason: "Initializing"},
						}
						if diff := cmp.Diff(expected, resource.Status.Conditions, rtesting.IgnoreLastTransitionTime); diff != "" {
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
					Sync: func(ctx context.Context, resource *resources.TestResourceNilableStatus) error {
						if resource.Status.Fields == nil {
							resource.Status.Fields = map[string]string{}
						}
						resource.Status.Fields["Reconciler"] = "ran"
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
		Name: "status update failed",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "TestResourceNilableStatus", rtesting.InduceFailureOpts{
				SubResource: "status",
			}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResourceNilableStatus) error {
						if resource.Status.Fields == nil {
							resource.Status.Fields = map[string]string{}
						}
						resource.Status.Fields["Reconciler"] = "ran"
						return nil
					},
				}
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "StatusUpdateFailed",
				`Failed to update status: inducing failure for update TestResourceNilableStatus`),
		},
		ExpectStatusUpdates: []client.Object{
			resource.StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("Reconciler", "ran")
			}),
		},
		ShouldErr: true,
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler{
			Type:       &resources.TestResourceNilableStatus{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler(t *testing.T) {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						if expected, actual := "ran", resource.Spec.Fields["Defaulter"]; expected != actual {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						expected := []metav1.Condition{
							{Type: apis.ConditionReady, Status: metav1.ConditionUnknown, Reason: "Initializing"},
						}
						if diff := cmp.Diff(expected, resource.Status.Conditions, rtesting.IgnoreLastTransitionTime); diff != "" {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						if resource.Status.Fields == nil {
							resource.Status.Fields = map[string]string{}
						}
						resource.Status.Fields["Reconciler"] = "ran"
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						if resource.Status.Fields == nil {
							resource.Status.Fields = map[string]string{}
						}
						resource.Status.Fields["Reconciler"] = "ran"
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
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
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						if config := reconcilers.RetrieveConfigOrDie(ctx); config != c {
							t.Errorf("expected config in context, found %#v", config)
						}
						if resourceConfig := reconcilers.RetrieveOriginalConfigOrDie(ctx); resourceConfig != c {
							t.Errorf("expected original config in context, found %#v", resourceConfig)
						}
						return nil
					},
				}
			},
		},
	}, {
		Name: "context has resource type",
		Key:  testKey,
		GivenObjects: []client.Object{
			resource,
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						if resourceType, ok := reconcilers.RetrieveOriginalResourceType(ctx).(*resources.TestResource); !ok {
							t.Errorf("expected original resource type not in context, found %#v", resourceType)
						}
						if resourceType, ok := reconcilers.RetrieveResourceType(ctx).(*resources.TestResource); !ok {
							t.Errorf("expected resource type not in context, found %#v", resourceType)
						}
						return nil
					},
				}
			},
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler{
			Type:       &resources.TestResource{},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}
	})
}

func TestAggregateReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	key := types.NamespacedName{Namespace: testNamespace, Name: testName}

	now := metav1.NewTime(time.Now().Truncate(time.Second))

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	configMapCreate := diecorev1.ConfigMapBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		})
	configMapGiven := configMapCreate.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		})

	defaultAggregateReconciler := func(c reconcilers.Config) *reconcilers.AggregateReconciler {
		return &reconcilers.AggregateReconciler{
			Type:    &corev1.ConfigMap{},
			Request: reconcile.Request{NamespacedName: key},

			DesiredResource: func(ctx context.Context, resource *corev1.ConfigMap) (*corev1.ConfigMap, error) {
				resource.Data = map[string]string{
					"foo": "bar",
				}
				return resource, nil
			},
			SemanticEquals: func(a1, a2 *corev1.ConfigMap) bool {
				return equality.Semantic.DeepEqual(a1.Data, a2.Data)
			},
			MergeBeforeUpdate: func(current, desired *corev1.ConfigMap) {
				current.Data = desired.Data
			},

			Config: c,
		}
	}

	rts := rtesting.ReconcilerTestSuite{{
		Name: "resource is in sync",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
	}, {
		Name: "ignore other resources",
		Key:  types.NamespacedName{Namespace: testName, Name: "not-it"},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
	}, {
		Name: "ignore terminating resources",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
	}, {
		Name: "create resource",
		Key:  key,
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectCreates: []client.Object{
			configMapCreate.
				AddData("foo", "bar"),
		},
	}, {
		Name: "update resource",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeNormal, "Updated",
				`Updated ConfigMap %q`, testName),
		},
		ExpectUpdates: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
	}, {
		Name: "delete resource",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven,
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.DesiredResource = func(ctx context.Context, resource client.Object) (client.Object, error) {
					return nil, nil
				}
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
	}, {
		Name: "preserve immutable fields",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				AddData("foo", "bar").
				AddData("immutable", "field"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.HarmonizeImmutableFields = func(current, desired *corev1.ConfigMap) {
					desired.Data["immutable"] = current.Data["immutable"]
				}
				return r
			},
		},
	}, {
		Name: "sanitize resource before logging",
		Key:  key,
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Sanitize = func(child *corev1.ConfigMap) interface{} {
					return child.Name
				}
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectCreates: []client.Object{
			configMapCreate.
				AddData("foo", "bar"),
		},
	}, {
		Name: "sanitize is mutation safe",
		Key:  key,
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Sanitize = func(child *corev1.ConfigMap) interface{} {
					child.Data["ignore"] = "me"
					return child
				}
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectCreates: []client.Object{
			configMapCreate.
				AddData("foo", "bar"),
		},
	}, {
		Name: "error getting resources",
		Key:  key,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
		ShouldErr: true,
	}, {
		Name: "error creating resource",
		Key:  key,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeWarning, "CreationFailed",
				`Failed to create ConfigMap %q: inducing failure for create ConfigMap`, testName),
		},
		ExpectCreates: []client.Object{
			configMapCreate.
				AddData("foo", "bar"),
		},
		ShouldErr: true,
	}, {
		Name: "error updating resource",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				return defaultAggregateReconciler(c)
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeWarning, "UpdateFailed",
				`Failed to update ConfigMap %q: inducing failure for update ConfigMap`, testName),
		},
		ExpectUpdates: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
		ShouldErr: true,
	}, {
		Name: "error deleting resource",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "ConfigMap"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.DesiredResource = func(ctx context.Context, resource client.Object) (client.Object, error) {
					return nil, nil
				}
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(configMapGiven, scheme, corev1.EventTypeWarning, "DeleteFailed",
				`Failed to delete ConfigMap %q: inducing failure for delete ConfigMap`, testName),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			rtesting.NewDeleteRefFromObject(configMapGiven, scheme),
		},
		ShouldErr: true,
	}, {
		Name: "reconcile result",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Reconciler = &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource client.Object) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: time.Hour}, nil
					},
				}
				return r
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: time.Hour},
	}, {
		Name: "reconcile error",
		Key:  key,
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Reconciler = &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource client.Object) error {
						return fmt.Errorf("test error")
					},
				}
				return r
			},
		},
		ShouldErr: true,
	}, {
		Name: "context is stashable",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Reconciler = &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
						var key reconcilers.StashKey = "foo"
						// StashValue will panic if the context is not setup correctly
						reconcilers.StashValue(ctx, key, "bar")
						return nil
					},
				}
				return r
			},
		},
	}, {
		Name: "context has config",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Reconciler = &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
						if config := reconcilers.RetrieveConfigOrDie(ctx); config != c {
							t.Errorf("expected config in context, found %#v", config)
						}
						if resourceConfig := reconcilers.RetrieveOriginalConfigOrDie(ctx); resourceConfig != c {
							t.Errorf("expected original config in context, found %#v", resourceConfig)
						}
						return nil
					},
				}
				return r
			},
		},
	}, {
		Name: "context has resource type",
		Key:  key,
		GivenObjects: []client.Object{
			configMapGiven.
				AddData("foo", "bar"),
		},
		Metadata: map[string]interface{}{
			"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
				r := defaultAggregateReconciler(c)
				r.Reconciler = &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
						if resourceType, ok := reconcilers.RetrieveOriginalResourceType(ctx).(*corev1.ConfigMap); !ok {
							t.Errorf("expected original resource type not in context, found %#v", resourceType)
						}
						if resourceType, ok := reconcilers.RetrieveResourceType(ctx).(*corev1.ConfigMap); !ok {
							t.Errorf("expected resource type not in context, found %#v", resourceType)
						}
						return nil
					},
				}
				return r
			},
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return rtc.Metadata["Reconciler"].(func(*testing.T, reconcilers.Config) reconcile.Reconciler)(t, c)
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
		Name:     "sync success",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						return nil
					},
				}
			},
		},
	}, {
		Name:     "sync error",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						return fmt.Errorf("syncreconciler error")
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name:     "missing sync method",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: nil,
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name:     "invalid sync signature",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource string) error {
						return nil
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name:     "should not finalize non-deleted resources",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						t.Errorf("reconciler should not call finalize for non-deleted resources")
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
	}, {
		Name: "should finalize deleted resources",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						t.Errorf("reconciler should not call sync for deleted resources")
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 3 * time.Hour},
	}, {
		Name: "should finalize and sync deleted resources when asked to",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					SyncDuringFinalization: true,
					Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
	}, {
		Name: "should finalize and sync deleted resources when asked to, shorter resync wins",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					SyncDuringFinalization: true,
					Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 3 * time.Hour}, nil
					},
					Finalize: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{RequeueAfter: 2 * time.Hour}, nil
					},
				}
			},
		},
		ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
	}, {
		Name: "finalize is optional",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						return nil
					},
				}
			},
		},
	}, {
		Name: "finalize error",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) error {
						return nil
					},
					Finalize: func(ctx context.Context, resource *resources.TestResource) error {
						return fmt.Errorf("syncreconciler finalize error")
					},
				}
			},
		},
		ShouldErr: true,
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
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
		Name:     "preserve no child",
		Resource: resourceReady,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return defaultChildReconciler(c)
			},
		},
	}, {
		Name: "child is in sync",
		Resource: resourceReady.
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
		Name: "child is in sync, in a different namespace",
		Resource: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Namespace("other-ns")
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.ListOptions = func(ctx context.Context, parent *resources.TestResource) []client.ListOption {
					return []client.ListOption{
						client.InNamespace("other-ns"),
					}
				}
				return r
			},
		},
	}, {
		Name: "create child",
		Resource: resource.
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
		ExpectResource: resourceReady.
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
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
				return r
			},
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(configMapCreate, resource, scheme),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Created",
				`Created ConfigMap %q`, testName),
		},
		ExpectResource: resourceReady.
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
			configMapCreate.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
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
		Resource: resourceReady.
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
		ExpectResource: resourceReady.
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
		Resource: resourceReady.
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
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
				return r
			},
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(configMapGiven, resource, scheme),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Updated",
				`Updated ConfigMap %q`, testName),
		},
		ExpectResource: resourceReady.
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
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}).
				AddData("new", "field"),
		},
	}, {
		Name: "update child, restoring missing finalizer",
		Resource: resourceReady.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
				d.AddField("new", "field")
			}).
			StatusDie(func(d *dies.TestResourceStatusDie) {
				d.AddField("foo", "bar")
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
				return r
			},
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(configMapGiven, resource, scheme),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Updated",
				`Updated ConfigMap %q`, testName),
		},
		ExpectResource: resourceReady.
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
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}).
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
		Name:     "delete child",
		Resource: resourceReady,
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
		Resource: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
		},
		ExpectResource: resourceReady.
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
		Resource: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers("some.other.finalizer")
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
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
		Name:     "ignore extraneous children",
		Resource: resourceReady,
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
		Resource: resource.
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
		ExpectResource: resourceReady.
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
		Resource: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(&now)
				d.Finalizers(testFinalizer)
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
				return r
			},
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
				`Deleted ConfigMap %q`, testName),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
		},
		ExpectResource: resourceReady.
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
		Resource: resourceReady.
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
		ExpectResource: resourceReady.
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
		Resource: resourceReady.
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
		Resource: resourceReady.
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
		Name:     "status only reconcile",
		Resource: resource,
		GivenObjects: []client.Object{
			configMapGiven,
		},
		ExpectResource: resourceReady.
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
		Resource: resource.
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
		ExpectResource: resourceReady.
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
		Resource: resource.
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
		ExpectResource: resourceReady.
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
		Name:     "error listing children",
		Resource: resourceReady,
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
		Resource: resource.
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
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.AddField("foo", "bar")
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
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
		Resource: resourceReady.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
			}),
		GivenObjects: []client.Object{
			configMapGiven.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.OwnerReferences()
				}),
		},
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				r := defaultChildReconciler(c)
				r.Finalizer = testFinalizer
				r.SkipOwnerReference = true
				r.OurChild = func(parent, child client.Object) bool { return true }
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
		Resource: resourceReady.
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
		Name:     "error deleting child",
		Resource: resourceReady,
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
		Resource: resource.
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
		Name:     "error creating desired child",
		Resource: resource,
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

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
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
		Name:     "sub reconciler erred",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return fmt.Errorf("reconciler error")
						},
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name:     "preserves result, Requeue",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
						return ctrl.Result{Requeue: true}, nil
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:     "preserves result, RequeueAfter",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:     "ignores result on err",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, fmt.Errorf("test error")
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{},
		ShouldErr:      true,
	}, {
		Name:     "Requeue + empty => Requeue",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:     "empty + Requeue => Requeue",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:     "RequeueAfter + empty => RequeueAfter",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:     "empty + RequeueAfter => RequeueAfter",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:     "RequeueAfter + Requeue => RequeueAfter",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:     "Requeue + RequeueAfter => RequeueAfter",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:     "RequeueAfter(1m) + RequeueAfter(2m) => RequeueAfter(1m)",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}, {
		Name:     "RequeueAfter(2m) + RequeueAfter(1m) => RequeueAfter(1m)",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return reconcilers.Sequence{
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
						},
					},
					&reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) (ctrl.Result, error) {
							return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{RequeueAfter: 1 * time.Minute},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

func TestCastResource(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

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

	rts := rtesting.SubReconcilerTestSuite{{
		Name: "sync success",
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.SpecDie(func(d *diecorev1.PodSpecDie) {
						d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *appsv1.Deployment) error {
							reconcilers.RetrieveConfigOrDie(ctx).
								Recorder.Event(resource, corev1.EventTypeNormal, "Test",
								resource.Spec.Template.Spec.Containers[0].Name)
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
		Name:     "cast mutation",
		Resource: resource,
		ExpectResource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Name("mutation")
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *appsv1.Deployment) error {
							// mutation that exists on the original resource and will be reflected
							resource.Spec.Template.Name = "mutation"
							// mutation that does not exists on the original resource and will be dropped
							resource.Spec.Paused = true
							return nil
						},
					},
				}
			},
		},
	}, {
		Name:     "return subreconciler result",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *appsv1.Deployment) (ctrl.Result, error) {
							return ctrl.Result{Requeue: true}, nil
						},
					},
				}
			},
		},
		ExpectedResult: ctrl.Result{Requeue: true},
	}, {
		Name:     "return subreconciler err",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *appsv1.Deployment) error {
							return fmt.Errorf("subreconciler error")
						},
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "subreconcilers must be compatible with cast value, not reconciled resource",
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.SpecDie(func(d *diecorev1.PodSpecDie) {
						d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return nil
						},
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name: "error for cast to different type than expected by sub reconciler",
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.SpecDie(func(d *diecorev1.PodSpecDie) {
						d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
					})
				})
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &appsv1.Deployment{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return nil
						},
					},
				}
			},
		},
		ShouldPanic: true,
	}, {
		Name: "marshal error",
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.ErrOnMarshal(true)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &resources.TestResource{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							c.Recorder.Event(resource, corev1.EventTypeNormal, "Test", resource.Name)
							return nil
						},
					},
				}
			},
		},
		ShouldErr: true,
	}, {
		Name: "unmarshal error",
		Resource: resource.
			SpecDie(func(d *dies.TestResourceSpecDie) {
				d.ErrOnUnmarshal(true)
			}),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.CastResource{
					Type: &resources.TestResource{},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							c.Recorder.Event(resource, corev1.EventTypeNormal, "Test", resource.Name)
							return nil
						},
					},
				}
			},
		},
		ShouldErr: true,
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
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
		Name:     "with config",
		Resource: resource,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, oc reconcilers.Config) reconcilers.SubReconciler {
				c := reconcilers.Config{
					Tracker: tracker.New(0),
				}

				return &reconcilers.WithConfig{
					Config: func(ctx context.Context, _ reconcilers.Config) (reconcilers.Config, error) {
						return c, nil
					},
					Reconciler: &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, parent *resources.TestResource) error {
							rc := reconcilers.RetrieveConfigOrDie(ctx)
							roc := reconcilers.RetrieveOriginalConfigOrDie(ctx)

							if rc != c {
								t.Errorf("unexpected config")
							}
							if roc != oc {
								t.Errorf("unexpected original config")
							}

							oc.Recorder.Event(resource, corev1.EventTypeNormal, "AllGood", "")

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

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

func TestWithFinalizer(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testFinalizer := "test-finalizer"

	now := &metav1.Time{Time: time.Now().Truncate(time.Second)}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name: "in sync",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
			}),
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Sync", ""),
		},
	}, {
		Name:     "add finalizer",
		Resource: resource,
		ExpectResource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers(testFinalizer)
				d.ResourceVersion("1000")
			}),
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Sync", ""),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: testNamespace,
				Name:      testName,
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":["test-finalizer"],"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name:     "error adding finalizer",
		Resource: resource,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("patch", "TestResource"),
		},
		ShouldErr: true,
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
				Patch:     []byte(`{"metadata":{"finalizers":["test-finalizer"],"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name: "clear finalizer",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(now)
				d.Finalizers(testFinalizer)
			}),
		ExpectResource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(now)
				d.ResourceVersion("1000")
			}),
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Finalize", ""),
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, testFinalizer),
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
		Name: "error clearing finalizer",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(now)
				d.Finalizers(testFinalizer)
			}),
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("patch", "TestResource"),
		},
		ShouldErr: true,
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Finalize", ""),
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
				Patch:     []byte(`{"metadata":{"finalizers":null,"resourceVersion":"999"}}`),
			},
		},
	}, {
		Name: "keep finalizer on error",
		Resource: resource.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.DeletionTimestamp(now)
				d.Finalizers(testFinalizer)
			}),
		ShouldErr: true,
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Finalize", ""),
		},
		Metadata: map[string]interface{}{
			"FinalizerError": fmt.Errorf("finalize error"),
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		var syncErr, finalizeErr error
		if err, ok := rtc.Metadata["SyncError"]; ok {
			syncErr = err.(error)
		}
		if err, ok := rtc.Metadata["FinalizerError"]; ok {
			finalizeErr = err.(error)
		}

		return &reconcilers.WithFinalizer{
			Finalizer: testFinalizer,
			Reconciler: &reconcilers.SyncReconciler{
				Sync: func(ctx context.Context, resource *resources.TestResource) error {
					c.Recorder.Event(resource, corev1.EventTypeNormal, "Sync", "")
					return syncErr
				},
				Finalize: func(ctx context.Context, resource *resources.TestResource) error {
					c.Recorder.Event(resource, corev1.EventTypeNormal, "Finalize", "")
					return finalizeErr
				},
			},
		}
	})
}

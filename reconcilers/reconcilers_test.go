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
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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
		})

	configMap := diecorev1.ConfigMapBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("track-namespace")
			d.Name("track-name")
		}).
		AddData("greeting", "hello")

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"track and get": {
			Resource: resource.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMap,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(configMap, resource, scheme),
			},
		},
		"track with not found get": {
			Resource:  resource.DieReleasePtr(),
			ShouldErr: true,
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(configMap, resource, scheme),
			},
		},
	}

	// run with typed objects
	t.Run("typed", func(t *testing.T) {
		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
			return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
			return &reconcilers.SyncReconciler[*resources.TestResource]{
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

func TestConfig_TrackAndList(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testSelector, _ := labels.Parse("app=test-app")

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		})

	configMap := diecorev1.ConfigMapBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("track-namespace")
			d.Name("track-name")
			d.AddLabel("app", "test-app")
		}).
		AddData("greeting", "hello")

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"track and list": {
			Resource: resource.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMap,
			},
			Metadata: map[string]interface{}{
				"listOpts": []client.ListOption{},
			},
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{
						Namespace: testNamespace,
						Name:      testName,
					},
					TrackedReference: tracker.Reference{
						Kind:     "ConfigMap",
						Selector: labels.Everything(),
					},
				},
			},
		},
		"track and list constrained": {
			Resource: resource.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMap,
			},
			Metadata: map[string]interface{}{
				"listOpts": []client.ListOption{
					client.InNamespace("track-namespace"),
					client.MatchingLabels(map[string]string{"app": "test-app"}),
				},
			},
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{
						Namespace: testNamespace,
						Name:      testName,
					},
					TrackedReference: tracker.Reference{
						Kind:      "ConfigMap",
						Namespace: "track-namespace",
						Selector:  testSelector,
					},
				},
			},
		},
		"track with errored list": {
			Resource:  resource.DieReleasePtr(),
			ShouldErr: true,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "ConfigMapList"),
			},
			Metadata: map[string]interface{}{
				"listOpts": []client.ListOption{},
			},
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{
						Namespace: testNamespace,
						Name:      testName,
					},
					TrackedReference: tracker.Reference{
						Kind:     "ConfigMap",
						Selector: labels.Everything(),
					},
				},
			},
		},
	}

	// run with typed objects
	t.Run("typed", func(t *testing.T) {
		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
			return &reconcilers.SyncReconciler[*resources.TestResource]{
				Sync: func(ctx context.Context, resource *resources.TestResource) error {
					c := reconcilers.RetrieveConfigOrDie(ctx)

					cms := &corev1.ConfigMapList{}
					listOpts := rtc.Metadata["listOpts"].([]client.ListOption)
					err := c.TrackAndList(ctx, cms, listOpts...)
					if err != nil {
						return err
					}

					if expected, actual := "hello", cms.Items[0].Data["greeting"]; expected != actual {
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
		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
			return &reconcilers.SyncReconciler[*resources.TestResource]{
				Sync: func(ctx context.Context, resource *resources.TestResource) error {
					c := reconcilers.RetrieveConfigOrDie(ctx)

					cms := &unstructured.UnstructuredList{}
					cms.SetAPIVersion("v1")
					cms.SetKind("ConfigMapList")
					listOpts := rtc.Metadata["listOpts"].([]client.ListOption)
					err := c.TrackAndList(ctx, cms, listOpts...)
					if err != nil {
						return err
					}

					if expected, actual := "hello", cms.UnstructuredContent()["items"].([]interface{})[0].(map[string]interface{})["data"].(map[string]interface{})["greeting"].(string); expected != actual {
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
	testRequest := reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceNoStatusBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.AddAnnotation("blah", "blah")
		})

	rts := rtesting.ReconcilerTests{
		"resource exists": {
			Request: testRequest,
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNoStatus] {
					return &reconcilers.SyncReconciler[*resources.TestResourceNoStatus]{
						Sync: func(ctx context.Context, resource *resources.TestResourceNoStatus) error {
							return nil
						},
					}
				},
			},
		},
	}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler[*resources.TestResourceNoStatus]{
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNoStatus])(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler_EmptyStatus(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource-empty-status"
	testRequest := reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceEmptyStatusBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.AddAnnotation("blah", "blah")
		})

	rts := rtesting.ReconcilerTests{
		"resource exists": {
			Request: testRequest,
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceEmptyStatus] {
					return &reconcilers.SyncReconciler[*resources.TestResourceEmptyStatus]{
						Sync: func(ctx context.Context, resource *resources.TestResourceEmptyStatus) error {
							return nil
						},
					}
				},
			},
		},
	}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler[*resources.TestResourceEmptyStatus]{
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceEmptyStatus])(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler_NilableStatus(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testRequest := reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceNilableStatusBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.ReconcilerTests{
		"nil status": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResourceNilableStatus{},
			},
			GivenObjects: []client.Object{
				resource.Status(nil),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNilableStatus] {
					return &reconcilers.SyncReconciler[*resources.TestResourceNilableStatus]{
						Sync: func(ctx context.Context, resource *resources.TestResourceNilableStatus) error {
							if resource.Status != nil {
								t.Errorf("status expected to be nil")
							}
							return nil
						},
					}
				},
			},
		},
		"status conditions are initialized": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResourceNilableStatus{},
			},
			GivenObjects: []client.Object{
				resource.StatusDie(func(d *dies.TestResourceStatusDie) {
					d.ConditionsDie()
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNilableStatus] {
					return &reconcilers.SyncReconciler[*resources.TestResourceNilableStatus]{
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
		},
		"reconciler mutated status": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResourceNilableStatus{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNilableStatus] {
					return &reconcilers.SyncReconciler[*resources.TestResourceNilableStatus]{
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
		},
		"status update failed": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResourceNilableStatus{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("update", "TestResourceNilableStatus", rtesting.InduceFailureOpts{
					SubResource: "status",
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNilableStatus] {
					return &reconcilers.SyncReconciler[*resources.TestResourceNilableStatus]{
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
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler[*resources.TestResourceNilableStatus]{
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResourceNilableStatus])(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler_Unstructured(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testRequest := reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		APIVersion(resources.GroupVersion.Identifier()).
		Kind("TestResource").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.Generation(1)
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.ReconcilerTests{
		"in sync status": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return &reconcilers.SyncReconciler[*unstructured.Unstructured]{
						Sync: func(ctx context.Context, resource *unstructured.Unstructured) error {
							return nil
						},
					}
				},
			},
		},
		"status update": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return &reconcilers.SyncReconciler[*unstructured.Unstructured]{
						Sync: func(ctx context.Context, resource *unstructured.Unstructured) error {
							resource.Object["status"].(map[string]interface{})["fields"] = map[string]interface{}{
								"Reconciler": "ran",
							}
							return nil
						},
					}
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "StatusUpdated", `Updated status`),
			},
			ExpectStatusUpdates: []client.Object{
				resource.StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("Reconciler", "ran")
				}).DieReleaseUnstructured(),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &reconcilers.ResourceReconciler[*unstructured.Unstructured]{
			Type: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": resources.GroupVersion.Identifier(),
					"kind":       "TestResource",
				},
			},
			Reconciler: rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured])(t, c),
			Config:     c,
		}
	})
}

func TestResourceReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testRequest := reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})
	deletedAt := metav1.NewTime(time.UnixMilli(2000))

	rts := rtesting.ReconcilerTests{
		"resource does not exist": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							t.Error("should not be called")
							return nil
						},
					}
				},
			},
		},
		"ignore deleted resource": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&deletedAt)
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							t.Error("should not be called")
							return nil
						},
					}
				},
			},
		},
		"error fetching resource": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "TestResource"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							t.Error("should not be called")
							return nil
						},
					}
				},
			},
			ShouldErr: true,
		},
		"resource is defaulted": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							if expected, actual := "ran", resource.Spec.Fields["Defaulter"]; expected != actual {
								t.Errorf("unexpected default value, actually = %v, expected = %v", expected, actual)
							}
							return nil
						},
					}
				},
			},
		},
		"status conditions are initialized": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource.StatusDie(func(d *dies.TestResourceStatusDie) {
					d.ConditionsDie()
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"reconciler mutated status": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"skip status updates": {
			Request: testRequest,
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SkipStatusUpdate": true,
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"sub reconciler erred": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return fmt.Errorf("reconciler error")
						},
					}
				},
			},
			ShouldErr: true,
		},
		"sub reconciler halted": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							Sync: func(ctx context.Context, resource *resources.TestResource) error {
								resource.Status.Fields = map[string]string{
									"want": "this to run",
								}
								return reconcilers.HaltSubReconcilers
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							Sync: func(ctx context.Context, resource *resources.TestResource) error {
								resource.Status.Fields = map[string]string{
									"don't want": "this to run",
								}
								return fmt.Errorf("reconciler error")
							},
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
					d.AddField("want", "this to run")
				}),
			},
		},
		"status update failed": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("update", "TestResource", rtesting.InduceFailureOpts{
					SubResource: "status",
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"context is stashable": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							var key reconcilers.StashKey = "foo"
							// StashValue will panic if the context is not setup correctly
							reconcilers.StashValue(ctx, key, "bar")
							return nil
						},
					}
				},
			},
		},
		"context has config": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"context has resource type": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"context can be augmented in Prepare and accessed in Cleanup": {
			Request: testRequest,
			StatusSubResourceTypes: []client.Object{
				&resources.TestResource{},
			},
			GivenObjects: []client.Object{
				resource,
			},
			Prepare: func(t *testing.T, ctx context.Context, tc *rtesting.ReconcilerTestCase) (context.Context, error) {
				key := "test-key"
				value := "test-value"
				ctx = context.WithValue(ctx, key, value)

				tc.Metadata["SubReconciler"] = func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							if v := ctx.Value(key); v != value {
								t.Errorf("expected %s to be in context", key)
							}
							return nil
						},
					}
				}
				tc.CleanUp = func(t *testing.T, ctx context.Context, tc *rtesting.ReconcilerTestCase) error {
					if v := ctx.Value(key); v != value {
						t.Errorf("expected %s to be in context", key)
					}
					return nil
				}

				return ctx, nil
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		skipStatusUpdate := false
		if skip, ok := rtc.Metadata["SkipStatusUpdate"].(bool); ok {
			skipStatusUpdate = skip
		}
		return &reconcilers.ResourceReconciler[*resources.TestResource]{
			Reconciler:       rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c),
			SkipStatusUpdate: skipStatusUpdate,
			Config:           c,
		}
	})
}

func TestAggregateReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	request := reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}

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
		})

	defaultAggregateReconciler := func(c reconcilers.Config) *reconcilers.AggregateReconciler[*corev1.ConfigMap] {
		return &reconcilers.AggregateReconciler[*corev1.ConfigMap]{
			Request: request,

			DesiredResource: func(ctx context.Context, resource *corev1.ConfigMap) (*corev1.ConfigMap, error) {
				resource.Data = map[string]string{
					"foo": "bar",
				}
				return resource, nil
			},
			MergeBeforeUpdate: func(current, desired *corev1.ConfigMap) {
				current.Data = desired.Data
			},

			Config: c,
		}
	}

	rts := rtesting.ReconcilerTests{
		"resource is in sync": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("foo", "bar"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					return defaultAggregateReconciler(c)
				},
			},
		},
		"ignore other resources": {
			Request: reconcilers.Request{
				NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "not-it"},
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					return defaultAggregateReconciler(c)
				},
			},
		},
		"ignore terminating resources": {
			Request: request,
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
		},
		"create resource": {
			Request: request,
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
		},
		"update resource": {
			Request: request,
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
		},
		"delete resource": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.DesiredResource = func(ctx context.Context, resource *corev1.ConfigMap) (*corev1.ConfigMap, error) {
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
		},
		"preserve immutable fields": {
			Request: request,
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
		},
		"sanitize resource before logging": {
			Request: request,
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
		},
		"sanitize is mutation safe": {
			Request: request,
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
		},
		"error getting resources": {
			Request: request,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					return defaultAggregateReconciler(c)
				},
			},
			ShouldErr: true,
		},
		"error creating resource": {
			Request: request,
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
		},
		"error updating resource": {
			Request: request,
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
		},
		"error deleting resource": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("delete", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.DesiredResource = func(ctx context.Context, resource *corev1.ConfigMap) (*corev1.ConfigMap, error) {
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
		},
		"reconcile result": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("foo", "bar"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = &reconcilers.SyncReconciler[*corev1.ConfigMap]{
						SyncWithResult: func(ctx context.Context, resource *corev1.ConfigMap) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: time.Hour}, nil
						},
					}
					return r
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: time.Hour},
		},
		"reconcile error": {
			Request: request,
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = &reconcilers.SyncReconciler[*corev1.ConfigMap]{
						Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
							return fmt.Errorf("test error")
						},
					}
					return r
				},
			},
			ShouldErr: true,
		},
		"reconcile halted": {
			Request: request,
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = reconcilers.Sequence[*corev1.ConfigMap]{
						&reconcilers.SyncReconciler[*corev1.ConfigMap]{
							Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
								return reconcilers.HaltSubReconcilers
							},
						},
						&reconcilers.SyncReconciler[*corev1.ConfigMap]{
							Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
								return fmt.Errorf("test error")
							},
						},
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
		},
		"context is stashable": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("foo", "bar"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = &reconcilers.SyncReconciler[*corev1.ConfigMap]{
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
		},
		"context has config": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("foo", "bar"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = &reconcilers.SyncReconciler[*corev1.ConfigMap]{
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
		},
		"context has resource type": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("foo", "bar"),
			},
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = &reconcilers.SyncReconciler[*corev1.ConfigMap]{
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
		},
		"context can be augmented in Prepare and accessed in Cleanup": {
			Request: request,
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("foo", "bar"),
			},
			Prepare: func(t *testing.T, ctx context.Context, tc *rtesting.ReconcilerTestCase) (context.Context, error) {
				key := "test-key"
				value := "test-value"
				ctx = context.WithValue(ctx, key, value)

				tc.Metadata["Reconciler"] = func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = &reconcilers.SyncReconciler[*corev1.ConfigMap]{
						Sync: func(ctx context.Context, resource *corev1.ConfigMap) error {
							if v := ctx.Value(key); v != value {
								t.Errorf("expected %s to be in context", key)
							}
							return nil
						},
					}
					return r
				}
				tc.CleanUp = func(t *testing.T, ctx context.Context, tc *rtesting.ReconcilerTestCase) error {
					if v := ctx.Value(key); v != value {
						t.Errorf("expected %s to be in context", key)
					}
					return nil
				}

				return ctx, nil
			},
		},
	}

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
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"sync success": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return nil
						},
					}
				},
			},
		},
		"sync error": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return fmt.Errorf("syncreconciler error")
						},
					}
				},
			},
			ShouldErr: true,
		},
		"missing sync method": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: nil,
					}
				},
			},
			ShouldPanic: true,
		},
		"should not finalize non-deleted resources": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: 2 * time.Hour}, nil
						},
						FinalizeWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							t.Errorf("reconciler should not call finalize for non-deleted resources")
							return reconcilers.Result{RequeueAfter: 3 * time.Hour}, nil
						},
					}
				},
			},
			ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
		},
		"should finalize deleted resources": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							t.Errorf("reconciler should not call sync for deleted resources")
							return reconcilers.Result{RequeueAfter: 2 * time.Hour}, nil
						},
						FinalizeWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: 3 * time.Hour}, nil
						},
					}
				},
			},
			ExpectedResult: reconcile.Result{RequeueAfter: 3 * time.Hour},
		},
		"should finalize and sync deleted resources when asked to": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						SyncDuringFinalization: true,
						SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: 2 * time.Hour}, nil
						},
						FinalizeWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: 3 * time.Hour}, nil
						},
					}
				},
			},
			ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
		},
		"should finalize and sync deleted resources when asked to, shorter resync wins": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						SyncDuringFinalization: true,
						SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: 3 * time.Hour}, nil
						},
						FinalizeWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{RequeueAfter: 2 * time.Hour}, nil
						},
					}
				},
			},
			ExpectedResult: reconcile.Result{RequeueAfter: 2 * time.Hour},
		},
		"finalize is optional": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return nil
						},
					}
				},
			},
		},
		"finalize error": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
		"context can be augmented in Prepare and accessed in Cleanup": {
			Resource: resource.DieReleasePtr(),
			Prepare: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase[*resources.TestResource]) (context.Context, error) {
				key := "test-key"
				value := "test-value"
				ctx = context.WithValue(ctx, key, value)

				tc.Metadata["SubReconciler"] = func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							if v := ctx.Value(key); v != value {
								t.Errorf("expected %s to be in context", key)
							}
							return nil
						},
					}
				}
				tc.CleanUp = func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase[*resources.TestResource]) error {
					if v := ctx.Value(key); v != value {
						t.Errorf("expected %s to be in context", key)
					}
					return nil
				}

				return ctx, nil
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c)
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
		})

	defaultChildReconciler := func(c reconcilers.Config) *reconcilers.ChildReconciler[*resources.TestResource, *corev1.ConfigMap, *corev1.ConfigMapList] {
		return &reconcilers.ChildReconciler[*resources.TestResource, *corev1.ConfigMap, *corev1.ConfigMapList]{
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
			ReflectChildStatusOnParent: func(ctx context.Context, parent *resources.TestResource, child *corev1.ConfigMap, err error) {
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
		}
	}

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"preserve no child": {
			Resource: resourceReady.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
		},
		"child is in sync": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
		},
		"child is in sync, in a different namespace": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace("other-ns")
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.ListOptions = func(ctx context.Context, parent *resources.TestResource) []client.ListOption {
						return []client.ListOption{
							client.InNamespace("other-ns"),
						}
					}
					return r
				},
			},
		},
		"create child": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
				}).
				DieReleasePtr(),
			ExpectCreates: []client.Object{
				configMapCreate,
			},
		},
		"create child with finalizer": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
				}).
				DieReleasePtr(),
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
		},
		"update child": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
				}).
				DieReleasePtr(),
			ExpectUpdates: []client.Object{
				configMapGiven.
					AddData("new", "field"),
			},
		},
		"update child, preserve finalizers": {
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
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
				}).
				DieReleasePtr(),
			ExpectUpdates: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}).
					AddData("new", "field"),
			},
		},
		"update child, restoring missing finalizer": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
				}).
				DieReleasePtr(),
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
		},
		"delete child": {
			Resource: resourceReady.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
		},
		"delete child, preserve finalizers": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers(testFinalizer, "some.other.finalizer")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
		},
		"ignore extraneous children": {
			Resource: resourceReady.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool {
						return false
					}
					return r
				},
			},
		},
		"delete duplicate children": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
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
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
			ExpectResource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
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
		},
		"delete child during finalization": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
		},
		"clear finalizer after child fully deleted": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
					return r
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
					`Patched finalizer %q`, testFinalizer),
			},
			ExpectResource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers()
					d.ResourceVersion("1000")
				}).
				DieReleasePtr(),
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
		},
		"preserve finalizer for terminating child": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
						d.DeletionTimestamp(&now)
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
					return r
				},
			},
		},
		"child name collision": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
					Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
				}).
				DieReleasePtr(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
					"Failed to create ConfigMap %q:  %q already exists", testName, testName),
			},
			ExpectCreates: []client.Object{
				configMapCreate,
			},
		},
		"child name collision, stale informer cache": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			APIGivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
					Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
		},
		"preserve immutable fields": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
					d.AddField("immutable", "field")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("immutable", "field"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.HarmonizeImmutableFields = func(current, desired *corev1.ConfigMap) {
						desired.Data["immutable"] = current.Data["immutable"]
					}
					return r
				},
			},
		},
		"status only reconcile": {
			Resource: resource.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			ExpectResource: resourceReady.
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.DesiredChild = func(ctx context.Context, parent *resources.TestResource) (*corev1.ConfigMap, error) {
						return nil, reconcilers.OnlyReconcileChildStatus
					}
					return r
				},
			},
		},
		"sanitize child before logging": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
				}).
				DieReleasePtr(),
			ExpectCreates: []client.Object{
				configMapCreate,
			},
		},
		"sanitize is mutation safe": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
				}).
				DieReleasePtr(),
			ExpectCreates: []client.Object{
				configMapCreate,
			},
		},
		"error listing children": {
			Resource: resourceReady.DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "ConfigMapList"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
			ShouldErr: true,
		},
		"error creating child": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
		},
		"error adding finalizer": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
		},
		"error clearing finalizer": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *resources.TestResource, child *corev1.ConfigMap) bool { return true }
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
					Patch:     []byte(`{"metadata":{"finalizers":null,"resourceVersion":"999"}}`),
				},
			},
			ShouldErr: true,
		},
		"error updating child": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("update", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
		},
		"error deleting child": {
			Resource: resourceReady.DieReleasePtr(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("delete", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
		},
		"error deleting duplicate children": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
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
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
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
		},
		"error creating desired child": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.DesiredChild = func(ctx context.Context, parent *resources.TestResource) (*corev1.ConfigMap, error) {
						return nil, fmt.Errorf("test error")
					}
					return r
				},
			},
			ShouldErr: true,
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c)
	})
}

func TestChildReconciler_Unstructured(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testFinalizer := "test.finalizer"

	now := metav1.NewTime(time.Now().Truncate(time.Second))

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		APIVersion("testing.reconciler.runtime/v1").
		Kind("TestResource").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
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
		APIVersion("v1").
		Kind("ConfigMap").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.ControlledBy(resource, scheme)
		}).
		AddData("foo", "bar")
	configMapGiven := configMapCreate.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
		})

	defaultChildReconciler := func(c reconcilers.Config) *reconcilers.ChildReconciler[*unstructured.Unstructured, *unstructured.Unstructured, *unstructured.UnstructuredList] {
		return &reconcilers.ChildReconciler[*unstructured.Unstructured, *unstructured.Unstructured, *unstructured.UnstructuredList]{
			ChildType: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			ChildListType: &unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMapList",
				},
			},
			DesiredChild: func(ctx context.Context, parent *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				fields, ok, _ := unstructured.NestedMap(parent.Object, "spec", "fields")
				if !ok || len(fields) == 0 {
					return nil, nil
				}

				child := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"namespace": parent.GetNamespace(),
							"name":      parent.GetName(),
						},
						"data": map[string]interface{}{},
					},
				}
				for k, v := range fields {
					unstructured.SetNestedField(child.Object, v, "data", k)
				}

				return child, nil
			},
			MergeBeforeUpdate: func(current, desired *unstructured.Unstructured) {
				current.Object["data"] = desired.Object["data"]
			},
			ReflectChildStatusOnParent: func(ctx context.Context, parent *unstructured.Unstructured, child *unstructured.Unstructured, err error) {
				if err != nil {
					if apierrs.IsAlreadyExists(err) {
						name := err.(apierrs.APIStatus).Status().Details.Name
						readyCond := map[string]interface{}{
							"type":    "Ready",
							"status":  "False",
							"reason":  "NameConflict",
							"message": fmt.Sprintf("%q already exists", name),
						}
						unstructured.SetNestedSlice(parent.Object, []interface{}{readyCond}, "status", "conditions")
					}
					return
				}
				if child == nil {
					unstructured.RemoveNestedField(parent.Object, "status", "fields")
					readyCond := map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "Ready",
						"message": "",
					}
					unstructured.SetNestedSlice(parent.Object, []interface{}{readyCond}, "status", "conditions")
					return
				}
				unstructured.SetNestedMap(parent.Object, map[string]interface{}{}, "status", "fields")
				for k, v := range child.Object["data"].(map[string]interface{}) {
					unstructured.SetNestedField(parent.Object, v, "status", "fields", k)
				}
				readyCond := map[string]interface{}{
					"type":    "Ready",
					"status":  "True",
					"reason":  "Ready",
					"message": "",
				}
				unstructured.SetNestedSlice(parent.Object, []interface{}{readyCond}, "status", "conditions")
			},
		}
	}

	rts := rtesting.SubReconcilerTests[*unstructured.Unstructured]{
		"preserve no child": {
			Resource: resourceReady.DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
		},
		"child is in sync": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
		},
		"child is in sync, in a different namespace": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace("other-ns")
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.ListOptions = func(ctx context.Context, parent *unstructured.Unstructured) []client.ListOption {
						return []client.ListOption{
							client.InNamespace("other-ns"),
						}
					}
					return r
				},
			},
		},
		"create child": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
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
				}).
				DieReleaseUnstructured(),
			ExpectCreates: []client.Object{
				configMapCreate.DieReleaseUnstructured(),
			},
		},
		"create child with finalizer": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
				}).
				DieReleaseUnstructured(),
			ExpectCreates: []client.Object{
				configMapCreate.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}).
					DieReleaseUnstructured(),
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
		},
		"update child": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
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
				}).
				DieReleaseUnstructured(),
			ExpectUpdates: []client.Object{
				configMapGiven.
					AddData("new", "field").
					DieReleaseUnstructured(),
			},
		},
		"update child, preserve finalizers": {
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
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
				}).
				DieReleaseUnstructured(),
			ExpectUpdates: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}).
					AddData("new", "field").
					DieReleaseUnstructured(),
			},
		},
		"update child, restoring missing finalizer": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
				}).
				DieReleaseUnstructured(),
			ExpectUpdates: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}).
					AddData("new", "field").
					DieReleaseUnstructured(),
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
		},
		"delete child": {
			Resource: resourceReady.DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
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
		},
		"delete child, preserve finalizers": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers(testFinalizer, "some.other.finalizer")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
		},
		"ignore extraneous children": {
			Resource: resourceReady.DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool {
						return false
					}
					return r
				},
			},
		},
		"delete duplicate children": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
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
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
			ExpectResource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
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
				configMapCreate.DieReleaseUnstructured(),
			},
		},
		"delete child during finalization": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers(testFinalizer)
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
		},
		"clear finalizer after child fully deleted": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers(testFinalizer)
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
					return r
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "FinalizerPatched",
					`Patched finalizer %q`, testFinalizer),
			},
			ExpectResource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers()
					d.ResourceVersion("1000")
				}).
				DieReleaseUnstructured(),
			ExpectPatches: []rtesting.PatchRef{
				{
					Group:     "testing.reconciler.runtime",
					Kind:      "TestResource",
					Namespace: testNamespace,
					Name:      testName,
					PatchType: types.MergePatchType,
					Patch:     []byte(`{"metadata":{"finalizers":[],"resourceVersion":"999"}}`),
				},
			},
		},
		"preserve finalizer for terminating child": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
					d.Finalizers(testFinalizer)
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences()
						d.DeletionTimestamp(&now)
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
					return r
				},
			},
		},
		"child name collision": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
					Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
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
				}).
				DieReleaseUnstructured(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
					"Failed to create ConfigMap %q:  %q already exists", testName, testName),
			},
			ExpectCreates: []client.Object{
				configMapCreate.DieReleaseUnstructured(),
			},
		},
		"child name collision, stale informer cache": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			APIGivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
					Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
				}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
					"Failed to create ConfigMap %q:  %q already exists", testName, testName),
			},
			ExpectCreates: []client.Object{
				configMapCreate.DieReleaseUnstructured(),
			},
			ShouldErr: true,
		},
		"preserve immutable fields": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
					d.AddField("immutable", "field")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven.
					AddData("immutable", "field"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.HarmonizeImmutableFields = func(current, desired *unstructured.Unstructured) {
						immutable, _, _ := unstructured.NestedString(current.Object, "data", "immutable")
						unstructured.SetNestedField(desired.Object, immutable, "data", "immutable")
					}
					return r
				},
			},
		},
		"status only reconcile": {
			Resource: resource.DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			ExpectResource: resourceReady.
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.DesiredChild = func(ctx context.Context, parent *unstructured.Unstructured) (*unstructured.Unstructured, error) {
						return nil, reconcilers.OnlyReconcileChildStatus
					}
					return r
				},
			},
		},
		"sanitize child before logging": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Sanitize = func(child *unstructured.Unstructured) interface{} {
						return child.GetName()
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
				}).
				DieReleaseUnstructured(),
			ExpectCreates: []client.Object{
				configMapCreate.DieReleaseUnstructured(),
			},
		},
		"sanitize is mutation safe": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Sanitize = func(child *unstructured.Unstructured) interface{} {
						unstructured.SetNestedField(child.Object, "me", "data", "ignore")
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
				}).
				DieReleaseUnstructured(),
			ExpectCreates: []client.Object{
				configMapCreate.DieReleaseUnstructured(),
			},
		},
		"error listing children": {
			Resource: resourceReady.DieReleaseUnstructured(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "ConfigMapList"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
			ShouldErr: true,
		},
		"error creating child": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
					`Failed to create ConfigMap %q: inducing failure for create ConfigMap`, testName),
			},
			ExpectCreates: []client.Object{
				configMapCreate.DieReleaseUnstructured(),
			},
			ShouldErr: true,
		},
		"error adding finalizer": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
		},
		"error clearing finalizer": {
			Resource: resourceReady.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers(testFinalizer)
				}).
				DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.Finalizer = testFinalizer
					r.SkipOwnerReference = true
					r.OurChild = func(parent *unstructured.Unstructured, child *unstructured.Unstructured) bool { return true }
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
					Patch:     []byte(`{"metadata":{"finalizers":[],"resourceVersion":"999"}}`),
				},
			},
			ShouldErr: true,
		},
		"error updating child": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("update", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					return defaultChildReconciler(c)
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "UpdateFailed",
					`Failed to update ConfigMap %q: inducing failure for update ConfigMap`, testName),
			},
			ExpectUpdates: []client.Object{
				configMapGiven.
					AddData("new", "field").
					DieReleaseUnstructured(),
			},
			ShouldErr: true,
		},
		"error deleting child": {
			Resource: resourceReady.DieReleaseUnstructured(),
			GivenObjects: []client.Object{
				configMapGiven,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("delete", "ConfigMap"),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
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
		},
		"error deleting duplicate children": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleaseUnstructured(),
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
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
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
		},
		"error creating desired child": {
			Resource: resource.DieReleaseUnstructured(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
					r := defaultChildReconciler(c)
					r.DesiredChild = func(ctx context.Context, parent *unstructured.Unstructured) (*unstructured.Unstructured, error) {
						return nil, fmt.Errorf("test error")
					}
					return r
				},
			},
			ShouldErr: true,
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*unstructured.Unstructured], c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured])(t, c)
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
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"sub reconciler erred": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							Sync: func(ctx context.Context, resource *resources.TestResource) error {
								return fmt.Errorf("reconciler error")
							},
						},
					}
				},
			},
			ShouldErr: true,
		},
		"preserves result, Requeue": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.SyncReconciler[*resources.TestResource]{
						SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
							return reconcilers.Result{Requeue: true}, nil
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{Requeue: true},
		},
		"preserves result, RequeueAfter": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
		"ignores result on err": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, fmt.Errorf("test error")
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{},
			ShouldErr:      true,
		},
		"Requeue + empty => Requeue": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{Requeue: true},
		},
		"empty + Requeue => Requeue": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{Requeue: true},
		},
		"RequeueAfter + empty => RequeueAfter": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
		"empty + RequeueAfter => RequeueAfter": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
		"RequeueAfter + Requeue => RequeueAfter": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
		"Requeue + RequeueAfter => RequeueAfter": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
		"RequeueAfter(1m) + RequeueAfter(2m) => RequeueAfter(1m)": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 2 * time.Minute}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
		"RequeueAfter(2m) + RequeueAfter(1m) => RequeueAfter(1m)": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return reconcilers.Sequence[*resources.TestResource]{
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 2 * time.Minute}, nil
							},
						},
						&reconcilers.SyncReconciler[*resources.TestResource]{
							SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
								return reconcilers.Result{RequeueAfter: 1 * time.Minute}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{RequeueAfter: 1 * time.Minute},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c)
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
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"sync success": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("test-container", func(d *diecorev1.ContainerDie) {})
						})
					})
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *appsv1.Deployment]{
						Reconciler: &reconcilers.SyncReconciler[*appsv1.Deployment]{
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
		},
		"cast mutation": {
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.Name("mutation")
						})
					})
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *appsv1.Deployment]{
						Reconciler: &reconcilers.SyncReconciler[*appsv1.Deployment]{
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
		},
		"return subreconciler result": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *appsv1.Deployment]{
						Reconciler: &reconcilers.SyncReconciler[*appsv1.Deployment]{
							SyncWithResult: func(ctx context.Context, resource *appsv1.Deployment) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, nil
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{Requeue: true},
		},
		"return subreconciler err": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *appsv1.Deployment]{
						Reconciler: &reconcilers.SyncReconciler[*appsv1.Deployment]{
							Sync: func(ctx context.Context, resource *appsv1.Deployment) error {
								return fmt.Errorf("subreconciler error")
							},
						},
					}
				},
			},
			ShouldErr: true,
		},
		"marshal error": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.ErrOnMarshal(true)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *resources.TestResource]{
						Reconciler: &reconcilers.SyncReconciler[*resources.TestResource]{
							Sync: func(ctx context.Context, resource *resources.TestResource) error {
								c.Recorder.Event(resource, corev1.EventTypeNormal, "Test", resource.Name)
								return nil
							},
						},
					}
				},
			},
			ShouldErr: true,
		},
		"unmarshal error": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.ErrOnUnmarshal(true)
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *resources.TestResource]{
						Reconciler: &reconcilers.SyncReconciler[*resources.TestResource]{
							Sync: func(ctx context.Context, resource *resources.TestResource) error {
								c.Recorder.Event(resource, corev1.EventTypeNormal, "Test", resource.Name)
								return nil
							},
						},
					}
				},
			},
			ShouldErr: true,
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c)
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
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"with config": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, oc reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					c := reconcilers.Config{
						Tracker: tracker.New(oc.Scheme(), 0),
					}

					return &reconcilers.WithConfig[*resources.TestResource]{
						Config: func(ctx context.Context, _ reconcilers.Config) (reconcilers.Config, error) {
							return c, nil
						},
						Reconciler: &reconcilers.SyncReconciler[*resources.TestResource]{
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
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c)
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
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"in sync": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Sync", ""),
			},
		},
		"add finalizer": {
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers(testFinalizer)
					d.ResourceVersion("1000")
				}).
				DieReleasePtr(),
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
		},
		"error adding finalizer": {
			Resource: resource.DieReleasePtr(),
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
		},
		"clear finalizer": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(now)
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			ExpectResource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(now)
					d.ResourceVersion("1000")
				}).
				DieReleasePtr(),
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
		},
		"error clearing finalizer": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(now)
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
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
		},
		"keep finalizer on error": {
			Resource: resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(now)
					d.Finalizers(testFinalizer)
				}).
				DieReleasePtr(),
			ShouldErr: true,
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Finalize", ""),
			},
			Metadata: map[string]interface{}{
				"FinalizerError": fmt.Errorf("finalize error"),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		var syncErr, finalizeErr error
		if err, ok := rtc.Metadata["SyncError"]; ok {
			syncErr = err.(error)
		}
		if err, ok := rtc.Metadata["FinalizerError"]; ok {
			finalizeErr = err.(error)
		}

		return &reconcilers.WithFinalizer[*resources.TestResource]{
			Finalizer: testFinalizer,
			Reconciler: &reconcilers.SyncReconciler[*resources.TestResource]{
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

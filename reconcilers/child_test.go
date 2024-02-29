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
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/internal/resources/dies"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
					switch {
					case apierrs.IsAlreadyExists(err):
						name := err.(apierrs.APIStatus).Status().Details.Name
						parent.Status.MarkNotReady(ctx, "NameConflict", "%q already exists", name)
					case apierrs.IsInvalid(err):
						name := err.(apierrs.APIStatus).Status().Details.Name
						parent.Status.MarkNotReady(ctx, "InvalidChild", "%q was rejected by the api server", name)
					}
					return
				}
				if child == nil {
					parent.Status.Fields = nil
					parent.Status.MarkReady(ctx)
					return
				}
				parent.Status.Fields = reconcilers.MergeMaps(child.Data)
				parent.Status.MarkReady(ctx)
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
		"child is in sync, and tracked": {
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
						// clear default owner ref since this is tracked
						d.OwnerReferences()
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					r.SkipOwnerReference = true
					r.OurChild = func(resource *resources.TestResource, child *corev1.ConfigMap) bool {
						return true
					}
					return r
				},
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(configMapGiven, resourceReady, scheme),
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
		"create child with custom owner reference": {
			Resource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					r := defaultChildReconciler(c)
					desiredChild := r.DesiredChild
					r.DesiredChild = func(ctx context.Context, resource *resources.TestResource) (*corev1.ConfigMap, error) {
						child, err := desiredChild(ctx, resource)
						if child != nil {
							child.OwnerReferences = []metav1.OwnerReference{
								{
									APIVersion: resources.GroupVersion.String(),
									Kind:       "TestResource",
									Name:       resource.GetName(),
									UID:        resource.GetUID(),
									Controller: pointer.Bool(true),
									// the default controller ref is set to block
									BlockOwnerDeletion: pointer.Bool(false),
								},
							}
						}
						return child, err
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
				configMapCreate.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.OwnerReferences(
							metav1.OwnerReference{
								APIVersion:         resources.GroupVersion.String(),
								Kind:               "TestResource",
								Name:               resource.GetName(),
								UID:                resource.GetUID(),
								Controller:         pointer.Bool(true),
								BlockOwnerDeletion: pointer.Bool(false),
							},
						)
					}),
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
						d.Finalizers(testFinalizer)
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
		"invalid child": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap", rtesting.InduceFailureOpts{
					Error: apierrs.NewInvalid(schema.GroupKind{}, testName, field.ErrorList{
						field.Invalid(field.NewPath("metadata", "name"), testName, ""),
					}),
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
							Reason("InvalidChild").Message(`"test-resource" was rejected by the api server`),
					)
				}).
				DieReleasePtr(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeWarning, "CreationFailed",
					"Failed to create ConfigMap %q:  %q is invalid: metadata.name: Invalid value: %q", testName, testName, testName),
			},
			ExpectCreates: []client.Object{
				configMapCreate,
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
						d.Finalizers(testFinalizer)
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

func TestChildReconciler_UnexportedFields(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

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

	childCreate := dies.TestResourceUnexportedFieldsBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
			d.ControlledBy(resource, scheme)
		})
	childGiven := childCreate.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.CreationTimestamp(now)
		})

	defaultChildReconciler := func(c reconcilers.Config) *reconcilers.ChildReconciler[*resources.TestResource, *resources.TestResourceUnexportedFields, *resources.TestResourceUnexportedFieldsList] {
		return &reconcilers.ChildReconciler[*resources.TestResource, *resources.TestResourceUnexportedFields, *resources.TestResourceUnexportedFieldsList]{
			DesiredChild: func(ctx context.Context, parent *resources.TestResource) (*resources.TestResourceUnexportedFields, error) {
				if len(parent.Spec.Fields) == 0 {
					return nil, nil
				}

				return &resources.TestResourceUnexportedFields{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: parent.Namespace,
						Name:      parent.Name,
					},
					Spec: resources.TestResourceUnexportedFieldsSpec{
						Fields:         parent.Spec.Fields,
						Template:       parent.Spec.Template,
						ErrOnMarshal:   parent.Spec.ErrOnMarshal,
						ErrOnUnmarshal: parent.Spec.ErrOnUnmarshal,
					},
				}, nil
			},
			MergeBeforeUpdate: func(current, desired *resources.TestResourceUnexportedFields) {
				current.Spec.Fields = desired.Spec.Fields
				current.Spec.Template = desired.Spec.Template
				current.Spec.ErrOnMarshal = desired.Spec.ErrOnMarshal
				current.Spec.ErrOnUnmarshal = desired.Spec.ErrOnUnmarshal
			},
			ReflectChildStatusOnParent: func(ctx context.Context, parent *resources.TestResource, child *resources.TestResourceUnexportedFields, err error) {
				if err != nil {
					switch {
					case apierrs.IsAlreadyExists(err):
						name := err.(apierrs.APIStatus).Status().Details.Name
						parent.Status.MarkNotReady(ctx, "NameConflict", "%q already exists", name)
					case apierrs.IsInvalid(err):
						name := err.(apierrs.APIStatus).Status().Details.Name
						parent.Status.MarkNotReady(ctx, "InvalidChild", "%q was rejected by the api server", name)
					}
					return
				}
				if child == nil {
					parent.Status.Fields = nil
					parent.Status.MarkReady(ctx)
					return
				}
				parent.Status.Fields = child.Status.Fields
				parent.Status.MarkReady(ctx)
			},
		}
	}

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
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
				childGiven.
					SpecDie(func(d *dies.TestResourceUnexportedFieldsSpecDie) {
						d.AddField("foo", "bar")
					}).
					StatusDie(func(d *dies.TestResourceUnexportedFieldsStatusDie) {
						d.AddField("foo", "bar")
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
		},
		"update status": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				childGiven.
					SpecDie(func(d *dies.TestResourceUnexportedFieldsSpecDie) {
						d.AddField("foo", "bar")
					}).
					StatusDie(func(d *dies.TestResourceUnexportedFieldsStatusDie) {
						d.AddField("foo", "bar")
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
					`Created TestResourceUnexportedFields %q`, testName),
			},
			ExpectResource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
				}).
				DieReleasePtr(),
			ExpectCreates: []client.Object{
				childCreate.
					SpecDie(func(d *dies.TestResourceUnexportedFieldsSpecDie) {
						d.AddField("foo", "bar")
					}),
			},
		},
		"update child": {
			Resource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				childGiven.
					SpecDie(func(d *dies.TestResourceUnexportedFieldsSpecDie) {
						d.AddField("foo", "bar")
					}),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Updated",
					`Updated TestResourceUnexportedFields %q`, testName),
			},
			ExpectResource: resourceReady.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("foo", "bar")
					d.AddField("new", "field")
				}).
				DieReleasePtr(),
			ExpectUpdates: []client.Object{
				childGiven.
					SpecDie(func(d *dies.TestResourceUnexportedFieldsSpecDie) {
						d.AddField("foo", "bar")
						d.AddField("new", "field")
					}),
			},
		},
		"delete child": {
			Resource: resourceReady.DieReleasePtr(),
			GivenObjects: []client.Object{
				childGiven,
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return defaultChildReconciler(c)
				},
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(resource, scheme, corev1.EventTypeNormal, "Deleted",
					`Deleted TestResourceUnexportedFields %q`, testName),
			},
			ExpectDeletes: []rtesting.DeleteRef{
				rtesting.NewDeleteRefFromObject(childGiven, scheme),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource])(t, c)
	})
}

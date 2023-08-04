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
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestAggregateReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"
	testFinalizer := "test.finalizer"
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
						d.Finalizers(testFinalizer)
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
		"reconcile halted with result": {
			Request: request,
			Metadata: map[string]interface{}{
				"Reconciler": func(t *testing.T, c reconcilers.Config) reconcile.Reconciler {
					r := defaultAggregateReconciler(c)
					r.Reconciler = reconcilers.Sequence[*corev1.ConfigMap]{
						&reconcilers.SyncReconciler[*corev1.ConfigMap]{
							SyncWithResult: func(ctx context.Context, resource *corev1.ConfigMap) (reconcilers.Result, error) {
								return reconcilers.Result{Requeue: true}, reconcilers.HaltSubReconcilers
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
			ExpectedResult: reconcilers.Result{Requeue: true},
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

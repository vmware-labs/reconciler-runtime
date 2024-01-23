/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers_test

import (
	"context"
	"fmt"
	"testing"

	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/internal/resources/dies"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

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
		"return subreconciler err, preserves result and status update": {
			Resource: resource.DieReleasePtr(),
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
					return &reconcilers.CastResource[*resources.TestResource, *appsv1.Deployment]{
						Reconciler: &reconcilers.SyncReconciler[*appsv1.Deployment]{
							SyncWithResult: func(ctx context.Context, resource *appsv1.Deployment) (reconcilers.Result, error) {
								resource.Status.Conditions[0] = appsv1.DeploymentCondition{
									Type:    apis.ConditionReady,
									Status:  corev1.ConditionFalse,
									Reason:  "Failed",
									Message: "expected error",
								}
								return reconcilers.Result{Requeue: true}, fmt.Errorf("subreconciler error")
							},
						},
					}
				},
			},
			ExpectedResult: reconcilers.Result{Requeue: true},
			ExpectResource: resource.
				StatusDie(func(d *dies.TestResourceStatusDie) {
					d.ConditionsDie(
						diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionFalse).Reason("Failed").Message("expected error"),
					)
				}).
				DieReleasePtr(),
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

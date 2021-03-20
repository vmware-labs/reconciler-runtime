/*
Copyright 2020 the original author or authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/vmware-labs/reconciler-runtime/client"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/testing/factories"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDuckClient(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)

	rts := rtesting.SubReconcilerTestSuite{{
		Name:   "get duck",
		Parent: factories.ConfigMap(),
		GivenObjects: []rtesting.Factory{
			factories.TestResource().
				NamespaceName(testNamespace, testName).
				PodTemplateSpec(func(pts factories.PodTemplateSpec) {
					pts.ContainerNamed("workload", func(c *corev1.Container) {})
				}),
		},
		ExpectParent: factories.ConfigMap().
			AddData("container", "workload"),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
						duck := &rtesting.TestDuck{}
						key := client.Key{
							APIVersion: rtesting.GroupVersion.String(),
							Kind:       "TestResource",
							Namespace:  testNamespace,
							Name:       testName,
						}
						err := c.GetDuck(ctx, key, duck)
						if err != nil {
							return err
						}
						parent.Data = map[string]string{
							"container": duck.Spec.Template.Spec.Containers[0].Name,
						}
						return nil
					},
				}
			},
		},
	}, {
		Name:      "get duck error",
		Parent:    factories.ConfigMap(),
		ShouldErr: true,
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
						duck := &rtesting.TestDuck{}
						key := client.Key{
							APIVersion: rtesting.GroupVersion.String(),
							Kind:       "TestResource",
							Namespace:  testNamespace,
							Name:       testName,
						}
						err := c.GetDuck(ctx, key, duck)
						if err != nil {
							return err
						}
						parent.Data = map[string]string{
							"container": duck.Spec.Template.Spec.Containers[0].Name,
						}
						return nil
					},
				}
			},
		},
	}, {
		Name:   "list duck",
		Parent: factories.ConfigMap(),
		GivenObjects: []rtesting.Factory{
			factories.TestResource().
				NamespaceName(testNamespace, testName).
				PodTemplateSpec(func(pts factories.PodTemplateSpec) {
					pts.ContainerNamed("workload", func(c *corev1.Container) {})
				}),
		},
		ExpectParent: factories.ConfigMap().
			AddData("count", "1").
			AddData("container", "workload"),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
						ducks := &rtesting.TestDuckList{}
						key := client.Key{
							APIVersion: rtesting.GroupVersion.String(),
							Kind:       "TestResourceList",
							Namespace:  testNamespace,
						}
						err := c.ListDuck(ctx, key, ducks)
						if err != nil {
							return err
						}
						parent.Data = map[string]string{
							"count":     fmt.Sprintf("%d", len(ducks.Items)),
							"container": ducks.Items[0].Spec.Template.Spec.Containers[0].Name,
						}
						return nil
					},
				}
			},
		},
	}, {
		Name:   "list duck error",
		Parent: factories.ConfigMap(),
		GivenObjects: []rtesting.Factory{
			factories.TestResource().
				NamespaceName(testNamespace, testName).
				PodTemplateSpec(func(pts factories.PodTemplateSpec) {
					pts.ContainerNamed("workload", func(c *corev1.Container) {})
				}),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "TestResourceList"),
		},
		ShouldErr:    true,
		ExpectParent: factories.ConfigMap(),
		Metadata: map[string]interface{}{
			"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
				return &reconcilers.SyncReconciler{
					Config: c,
					Sync: func(ctx context.Context, parent *corev1.ConfigMap) error {
						ducks := &rtesting.TestDuckList{}
						key := client.Key{
							APIVersion: rtesting.GroupVersion.String(),
							Kind:       "TestResourceList",
							Namespace:  testNamespace,
						}
						err := c.ListDuck(ctx, key, ducks)
						if err != nil {
							return err
						}
						parent.Data = map[string]string{
							"count":     fmt.Sprintf("%d", len(ducks.Items)),
							"container": ducks.Items[0].Spec.Template.Spec.Containers[0].Name,
						}
						return nil
					},
				}
			},
		},
	}}

	rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return rtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c)
	})
}

/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/internal/resources/dies"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestIfThen(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		SpecDie(func(d *dies.TestResourceSpecDie) {
			d.Fields(map[string]string{})
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"then": {
			Metadata: map[string]interface{}{
				"Condition": true,
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("then", "called")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 1},
		},
		"then error": {
			Metadata: map[string]interface{}{
				"Condition": true,
				"ThenError": fmt.Errorf("then error"),
				"ElseError": fmt.Errorf("else error"),
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("then", "called")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 1}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "then error" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
		"else": {
			Metadata: map[string]interface{}{
				"Condition": false,
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("else", "called")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 2},
		},
		"else error": {
			Metadata: map[string]interface{}{
				"Condition": false,
				"ThenError": fmt.Errorf("then error"),
				"ElseError": fmt.Errorf("else error"),
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("else", "called")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 2}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "else error" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return &reconcilers.IfThen[*resources.TestResource]{
			If: func(ctx context.Context, resource *resources.TestResource) bool {
				return rtc.Metadata["Condition"].(bool)
			},
			Then: &reconcilers.SyncReconciler[*resources.TestResource]{
				SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
					resource.Spec.Fields["then"] = "called"
					var thenErr error
					if err, ok := rtc.Metadata["ThenError"]; ok {
						thenErr = err.(error)
					}
					return reconcilers.Result{RequeueAfter: 1}, thenErr
				},
			},
			Else: &reconcilers.SyncReconciler[*resources.TestResource]{
				SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
					resource.Spec.Fields["else"] = "called"
					var elseErr error
					if err, ok := rtc.Metadata["ElseError"]; ok {
						elseErr = err.(error)
					}
					return reconcilers.Result{RequeueAfter: 2}, elseErr
				},
			},
		}
	})
}

func TestWhile(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		SpecDie(func(d *dies.TestResourceSpecDie) {
			d.Fields(map[string]string{})
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"return immediately": {
			Metadata: map[string]interface{}{
				"Iterations": 0,
			},
			Resource:       resource.DieReleasePtr(),
			ExpectResource: resource.DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 0},
		},
		"return after 10 iterations": {
			Metadata: map[string]interface{}{
				"Iterations": 10,
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("iterations", "10")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 990},
		},
		"halt after default max iterations": {
			Metadata: map[string]interface{}{
				"Iterations": 1000,
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("iterations", "100")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 900}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "exceeded max iterations: 100" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
		"return before custom max iterations": {
			Metadata: map[string]interface{}{
				"MaxIterations": 10,
				"Iterations":    5,
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("iterations", "5")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 995},
		},
		"halt after custom max iterations": {
			Metadata: map[string]interface{}{
				"MaxIterations": 10,
				"Iterations":    1000,
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("iterations", "10")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 990}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "exceeded max iterations: 10" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		var desiredIterations int
		if i, ok := rtc.Metadata["Iterations"]; ok {
			desiredIterations = i.(int)
		}

		r := &reconcilers.While[*resources.TestResource]{
			Condition: func(ctx context.Context, resource *resources.TestResource) bool {
				return desiredIterations != reconcilers.RetrieveIteration(ctx)
			},
			Reconciler: &reconcilers.SyncReconciler[*resources.TestResource]{
				SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcile.Result, error) {
					i := reconcilers.RetrieveIteration(ctx) + 1
					resource.Spec.Fields["iterations"] = fmt.Sprintf("%d", i)
					return reconcile.Result{RequeueAfter: time.Duration(1000 - i)}, nil
				},
			},
		}
		if i, ok := rtc.Metadata["MaxIterations"]; ok {
			r.MaxIterations = pointer.Int(i.(int))
		}

		return r
	})
}

func TestTryCatch(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		SpecDie(func(d *dies.TestResourceSpecDie) {
			d.Fields(map[string]string{})
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"catch rethrow": {
			Metadata: map[string]interface{}{
				"TryError": fmt.Errorf("try"),
				"Catch": func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					resource.Spec.Fields["catch"] = "called"
					return result, err
				},
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("try", "called")
					d.AddField("catch", "called")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 3}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "try" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
		"catch and suppress error": {
			Metadata: map[string]interface{}{
				"TryError": fmt.Errorf("try"),
				"Catch": func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					resource.Spec.Fields["catch"] = "called"
					return result, nil
				},
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("try", "called")
					d.AddField("catch", "called")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 3},
		},
		"catch and inject error": {
			Metadata: map[string]interface{}{
				"Catch": func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					resource.Spec.Fields["catch"] = "called"
					return reconcile.Result{RequeueAfter: 2}, fmt.Errorf("catch")
				},
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("try", "called")
					d.AddField("catch", "called")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 2}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "catch" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
		"finally": {
			Metadata: map[string]interface{}{
				"Catch": func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					resource.Spec.Fields["catch"] = "called"
					return result, err
				},
				"Finally": &reconcilers.SyncReconciler[*resources.TestResource]{
					SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcile.Result, error) {
						resource.Spec.Fields["finally"] = "called"
						return reconcile.Result{RequeueAfter: 1}, nil
					},
				},
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("try", "called")
					d.AddField("catch", "called")
					d.AddField("finally", "called")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{RequeueAfter: 1},
		},
		"finally with try error": {
			Metadata: map[string]interface{}{
				"TryError": fmt.Errorf("try"),
				"Catch": func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					resource.Spec.Fields["catch"] = "called"
					return result, err
				},
				"Finally": &reconcilers.SyncReconciler[*resources.TestResource]{
					SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcile.Result, error) {
						resource.Spec.Fields["finally"] = "called"
						return reconcile.Result{RequeueAfter: 1}, nil
					},
				},
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("try", "called")
					d.AddField("catch", "called")
					d.AddField("finally", "called")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 1}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "try" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
		"finally error overrides try error": {
			Metadata: map[string]interface{}{
				"TryError": fmt.Errorf("try"),
				"Catch": func(ctx context.Context, resource *resources.TestResource, result reconcile.Result, err error) (reconcile.Result, error) {
					resource.Spec.Fields["catch"] = "called"
					return result, err
				},
				"Finally": &reconcilers.SyncReconciler[*resources.TestResource]{
					SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcile.Result, error) {
						resource.Spec.Fields["finally"] = "called"
						return reconcile.Result{RequeueAfter: 1}, fmt.Errorf("finally")
					},
				},
			},
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("try", "called")
					d.AddField("catch", "called")
					d.AddField("finally", "called")
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result reconcilers.Result, err error) {
				if result != (reconcile.Result{RequeueAfter: 1}) {
					t.Errorf("unexpected result: %v", result)
				}
				if err.Error() != "finally" {
					t.Errorf("unexpected error: %s", err)
				}
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		r := &reconcilers.TryCatch[*resources.TestResource]{
			Try: &reconcilers.SyncReconciler[*resources.TestResource]{
				SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
					resource.Spec.Fields["try"] = "called"
					var tryErr error
					if err, ok := rtc.Metadata["TryError"]; ok {
						tryErr = err.(error)
					}
					return reconcilers.Result{RequeueAfter: 3}, tryErr
				},
			},
		}
		if catch, ok := rtc.Metadata["Catch"]; ok {
			r.Catch = catch.(func(context.Context, *resources.TestResource, reconcile.Result, error) (reconcile.Result, error))
		}
		if finally, ok := rtc.Metadata["Finally"]; ok {
			r.Finally = finally.(reconcilers.SubReconciler[*resources.TestResource])
		}
		return r
	})
}

func TestOverrideSetup(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		SpecDie(func(d *dies.TestResourceSpecDie) {
			d.Fields(map[string]string{})
		})

	rts := rtesting.SubReconcilerTests[*resources.TestResource]{
		"calls reconcile": {
			Resource: resource.DieReleasePtr(),
			ExpectResource: resource.
				SpecDie(func(d *dies.TestResourceSpecDie) {
					d.AddField("reconciler", "called")
				}).
				DieReleasePtr(),
			ExpectedResult: reconcile.Result{Requeue: true},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*resources.TestResource], c reconcilers.Config) reconcilers.SubReconciler[*resources.TestResource] {
		return &reconcilers.OverrideSetup[*resources.TestResource]{
			Reconciler: &reconcilers.SyncReconciler[*resources.TestResource]{
				Setup: func(ctx context.Context, mgr manager.Manager, bldr *builder.Builder) error {
					return fmt.Errorf("setup")
				},
				SyncWithResult: func(ctx context.Context, resource *resources.TestResource) (reconcilers.Result, error) {
					resource.Spec.Fields["reconciler"] = "called"
					return reconcilers.Result{Requeue: true}, nil
				},
			},
		}
	})
}

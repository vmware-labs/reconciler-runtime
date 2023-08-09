/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vmware-labs/reconciler-runtime/internal"
	rtime "github.com/vmware-labs/reconciler-runtime/time"
	"github.com/vmware-labs/reconciler-runtime/tracker"
)

var (
	_ reconcile.Reconciler = (*AggregateReconciler[client.Object])(nil)
)

// AggregateReconciler is a controller-runtime reconciler that reconciles a specific resource. The
// Type resource is fetched for the reconciler
// request and passed in turn to each SubReconciler. Finally, the reconciled
// resource's status is compared with the original status, updating the API
// server if needed.
type AggregateReconciler[Type client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `{Type}ResourceReconciler`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr Manager, bldr *Builder) error

	// Type of resource to reconcile. Required when the generic type is not a
	// struct, or is unstructured.
	//
	// +optional
	Type Type

	// Request of resource to reconcile. Only the specific resource matching the namespace and name
	// is reconciled. The namespace may be empty for cluster scoped resources.
	Request Request

	// Reconciler is called for each reconciler request with the resource being reconciled.
	// Typically, Reconciler is a Sequence of multiple SubReconcilers.
	//
	// When HaltSubReconcilers is returned as an error, execution continues as if no error was
	// returned.
	//
	// +optional
	Reconciler SubReconciler[Type]

	// DesiredResource returns the desired resource to create/update, or nil if
	// the resource should not exist.
	//
	// +optional
	DesiredResource func(ctx context.Context, resource Type) (Type, error)

	// HarmonizeImmutableFields allows fields that are immutable on the current
	// object to be copied to the desired object in order to avoid creating
	// updates which are guaranteed to fail.
	//
	// +optional
	HarmonizeImmutableFields func(current, desired Type)

	// MergeBeforeUpdate copies desired fields on to the current object before
	// calling update. Typically fields to copy are the Spec, Labels and
	// Annotations.
	MergeBeforeUpdate func(current, desired Type)

	// Sanitize is called with an object before logging the value. Any value may
	// be returned. A meaningful subset of the resource is typically returned,
	// like the Spec.
	//
	// +optional
	Sanitize func(resource Type) interface{}

	Config Config

	// stamp manages the lifecycle of the aggregated resource.
	stamp    *ResourceManager[Type]
	lazyInit sync.Once
}

func (r *AggregateReconciler[T]) init() {
	r.lazyInit.Do(func() {
		if internal.IsNil(r.Type) {
			var nilT T
			r.Type = newEmpty(nilT).(T)
		}
		if r.Name == "" {
			r.Name = fmt.Sprintf("%sAggregateReconciler", typeName(r.Type))
		}
		if r.Reconciler == nil {
			r.Reconciler = Sequence[T]{}
		}
		if r.DesiredResource == nil {
			r.DesiredResource = func(ctx context.Context, resource T) (T, error) {
				return resource, nil
			}
		}

		r.stamp = &ResourceManager[T]{
			Name: r.Name,
			Type: r.Type,

			HarmonizeImmutableFields: r.HarmonizeImmutableFields,
			MergeBeforeUpdate:        r.MergeBeforeUpdate,
			Sanitize:                 r.Sanitize,
		}
	})
}

func (r *AggregateReconciler[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	_, err := r.SetupWithManagerYieldingController(ctx, mgr)
	return err
}

func (r *AggregateReconciler[T]) SetupWithManagerYieldingController(ctx context.Context, mgr ctrl.Manager) (controller.Controller, error) {
	r.init()

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues(
			"resourceType", gvk(r.Type, r.Config.Scheme()),
			"request", r.Request,
		)
	ctx = logr.NewContext(ctx, log)

	ctx = StashConfig(ctx, r.Config)
	ctx = StashOriginalConfig(ctx, r.Config)
	ctx = StashResourceType(ctx, r.Type)
	ctx = StashOriginalResourceType(ctx, r.Type)

	if err := r.validate(ctx); err != nil {
		return nil, err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).For(r.Type)
	if r.Setup != nil {
		if err := r.Setup(ctx, mgr, bldr); err != nil {
			return nil, err
		}
	}
	if err := r.Reconciler.SetupWithManager(ctx, mgr, bldr); err != nil {
		return nil, err
	}
	if err := r.stamp.Setup(ctx); err != nil {
		return nil, err
	}
	return bldr.Build(r)
}

func (r *AggregateReconciler[T]) validate(ctx context.Context) error {
	// validate Request value
	if r.Request.Name == "" {
		return fmt.Errorf("AggregateReconciler %q must define Request", r.Name)
	}

	// validate Reconciler value
	if r.Reconciler == nil && r.DesiredResource == nil {
		return fmt.Errorf("AggregateReconciler %q must define Reconciler and/or DesiredResource", r.Name)
	}

	return nil
}

func (r *AggregateReconciler[T]) Reconcile(ctx context.Context, req Request) (Result, error) {
	r.init()

	if req.Namespace != r.Request.Namespace || req.Name != r.Request.Name {
		// ignore other requests
		return Result{}, nil
	}

	ctx = WithStash(ctx)

	c := r.Config

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("resourceType", gvk(r.Type, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	ctx = rtime.StashNow(ctx, time.Now())
	ctx = StashRequest(ctx, req)
	ctx = StashConfig(ctx, c)
	ctx = StashOriginalConfig(ctx, r.Config)
	ctx = StashOriginalResourceType(ctx, r.Type)
	ctx = StashResourceType(ctx, r.Type)

	resource := r.Type.DeepCopyObject().(T)
	if err := c.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrs.IsNotFound(err) {
			// not found is ok
			resource.SetNamespace(r.Request.Namespace)
			resource.SetName(r.Request.Name)
		} else {
			log.Error(err, "unable to fetch resource")
			return Result{}, err
		}
	}

	if resource.GetDeletionTimestamp() != nil {
		// resource is being deleted, nothing to do
		return Result{}, nil
	}

	result, err := r.Reconciler.Reconcile(ctx, resource)
	if err != nil && !errors.Is(err, HaltSubReconcilers) {
		return result, err
	}

	// hack, ignore track requests from the child reconciler, we have it covered
	ctx = StashConfig(ctx, Config{
		Client:    c.Client,
		APIReader: c.APIReader,
		Recorder:  c.Recorder,
		Tracker:   tracker.New(c.Scheme(), 0),
	})
	desired, err := r.desiredResource(ctx, resource)
	if err != nil {
		return Result{}, err
	}
	_, err = r.stamp.Manage(ctx, resource, resource, desired)
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r *AggregateReconciler[T]) desiredResource(ctx context.Context, resource T) (T, error) {
	var nilT T

	if resource.GetDeletionTimestamp() != nil {
		// the reconciled resource is pending deletion, cleanup the child resource
		return nilT, nil
	}

	fn := reflect.ValueOf(r.DesiredResource)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(resource.DeepCopyObject()),
	})
	var obj T
	if !out[0].IsNil() {
		obj = out[0].Interface().(T)
	}
	var err error
	if !out[1].IsNil() {
		err = out[1].Interface().(error)
	}
	return obj, err
}

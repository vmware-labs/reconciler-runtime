/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-labs/reconciler-runtime/internal"
)

var (
	_ SubReconciler[client.Object] = (*ChildReconciler[client.Object, client.Object, client.ObjectList])(nil)
)

var (
	OnlyReconcileChildStatus = errors.New("skip reconciler create/update/delete behavior for the child resource, while still reflecting the existing child's status on the reconciled resource")
)

// ChildReconciler is a sub reconciler that manages a single child resource for a reconciled
// resource. The reconciler will ensure that exactly one child will match the desired state by:
//   - creating a child if none exists
//   - updating an existing child
//   - removing an unneeded child
//   - removing extra children
//
// The flow for each reconciliation request is:
//   - DesiredChild
//   - if child is desired, HarmonizeImmutableFields (optional)
//   - if child is desired, MergeBeforeUpdate
//   - ReflectChildStatusOnParent
//
// During setup, the child resource type is registered to watch for changes.
type ChildReconciler[Type, ChildType client.Object, ChildListType client.ObjectList] struct {
	// Name used to identify this reconciler.  Defaults to `{ChildType}ChildReconciler`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// ChildType is the resource being created/updated/deleted by the reconciler. For example, a
	// reconciled resource Deployment would have a ReplicaSet as a child. Required when the
	// generic type is not a struct, or is unstructured.
	//
	// +optional
	ChildType ChildType
	// ChildListType is the listing type for the child type. For example,
	// PodList is the list type for Pod. Required when the generic type is not
	// a struct, or is unstructured.
	//
	// +optional
	ChildListType ChildListType

	// Finalizer is set on the reconciled resource before a child resource is created, and cleared
	// after a child resource is deleted. The value must be unique to this specific reconciler
	// instance and not shared. Reusing a value may result in orphaned resources when the
	// reconciled resource is deleted.
	//
	// Using a finalizer is encouraged when the Kubernetes garbage collector is unable to delete
	// the child resource automatically, like when the reconciled resource and child are in different
	// namespaces, scopes or clusters.
	//
	// Use of a finalizer implies that SkipOwnerReference is true, and OurChild must be defined.
	//
	// +optional
	Finalizer string

	// SkipOwnerReference when true will not create and find child resources via an owner
	// reference. OurChild must be defined for the reconciler to distinguish the child being
	// reconciled from other resources of the same type.
	//
	// Any child resource created is tracked for changes.
	SkipOwnerReference bool

	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error

	// DesiredChild returns the desired child object for the given reconciled resource, or nil if
	// the child should not exist.
	//
	// To skip reconciliation of the child resource while still reflecting an existing child's
	// status on the reconciled resource, return OnlyReconcileChildStatus as an error.
	DesiredChild func(ctx context.Context, resource Type) (ChildType, error)

	// ReflectChildStatusOnParent updates the reconciled resource's status with values from the
	// child. Select types of errors are passed, including:
	//   - apierrs.IsAlreadyExists
	//   - apierrs.IsInvalid
	//
	// Most errors are returned directly, skipping this method. The set of handled error types
	// may grow, implementations should be defensive rather than assuming the error type.
	ReflectChildStatusOnParent func(ctx context.Context, parent Type, child ChildType, err error)

	// HarmonizeImmutableFields allows fields that are immutable on the current
	// object to be copied to the desired object in order to avoid creating
	// updates which are guaranteed to fail.
	//
	// +optional
	HarmonizeImmutableFields func(current, desired ChildType)

	// MergeBeforeUpdate copies desired fields on to the current object before
	// calling update. Typically fields to copy are the Spec, Labels and
	// Annotations.
	MergeBeforeUpdate func(current, desired ChildType)

	// ListOptions allows custom options to be use when listing potential child resources. Each
	// resource retrieved as part of the listing is confirmed via OurChild.
	//
	// Defaults to filtering by the reconciled resource's namespace:
	//     []client.ListOption{
	//         client.InNamespace(resource.GetNamespace()),
	//     }
	//
	// +optional
	ListOptions func(ctx context.Context, resource Type) []client.ListOption

	// OurChild is used when there are multiple ChildReconciler for the same ChildType controlled
	// by the same reconciled resource. The function return true for child resources managed by
	// this ChildReconciler. Objects returned from the DesiredChild function should match this
	// function, otherwise they may be orphaned. If not specified, all children match.
	//
	// OurChild is required when a Finalizer is defined or SkipOwnerReference is true.
	//
	// +optional
	OurChild func(resource Type, child ChildType) bool

	// Sanitize is called with an object before logging the value. Any value may
	// be returned. A meaningful subset of the resource is typically returned,
	// like the Spec.
	//
	// +optional
	Sanitize func(child ChildType) interface{}

	stamp    *ResourceManager[ChildType]
	lazyInit sync.Once
}

func (r *ChildReconciler[T, CT, CLT]) init() {
	r.lazyInit.Do(func() {
		if internal.IsNil(r.ChildType) {
			var nilCT CT
			r.ChildType = newEmpty(nilCT).(CT)
		}
		if internal.IsNil(r.ChildListType) {
			var nilCLT CLT
			r.ChildListType = newEmpty(nilCLT).(CLT)
		}
		if r.Name == "" {
			r.Name = fmt.Sprintf("%sChildReconciler", typeName(r.ChildType))
		}
		if r.Sanitize == nil {
			r.Sanitize = func(child CT) interface{} { return child }
		}
		if r.stamp == nil {
			r.stamp = &ResourceManager[CT]{
				Name:                     r.Name,
				Type:                     r.ChildType,
				Finalizer:                r.Finalizer,
				TrackDesired:             r.SkipOwnerReference,
				HarmonizeImmutableFields: r.HarmonizeImmutableFields,
				MergeBeforeUpdate:        r.MergeBeforeUpdate,
				Sanitize:                 r.Sanitize,
			}
		}
	})
}

func (r *ChildReconciler[T, CT, CLT]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	r.init()

	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(c, r.ChildType))
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}

	if r.SkipOwnerReference {
		bldr.Watches(r.ChildType, EnqueueTracked(ctx))
	} else {
		bldr.Owns(r.ChildType)
	}

	if r.Setup == nil {
		return nil
	}
	return r.Setup(ctx, mgr, bldr)
}

func (r *ChildReconciler[T, CT, CLT]) validate(ctx context.Context) error {
	// default implicit values
	if r.Finalizer != "" {
		r.SkipOwnerReference = true
	}

	// require DesiredChild
	if r.DesiredChild == nil {
		return fmt.Errorf("ChildReconciler %q must implement DesiredChild", r.Name)
	}

	// require ReflectChildStatusOnParent
	if r.ReflectChildStatusOnParent == nil {
		return fmt.Errorf("ChildReconciler %q must implement ReflectChildStatusOnParent", r.Name)
	}

	if r.OurChild == nil && r.SkipOwnerReference {
		// OurChild is required when SkipOwnerReference is true
		return fmt.Errorf("ChildReconciler %q must implement OurChild since owner references are not used", r.Name)
	}

	// require MergeBeforeUpdate
	if r.MergeBeforeUpdate == nil {
		return fmt.Errorf("ChildReconciler %q must implement MergeBeforeUpdate", r.Name)
	}

	return nil
}

func (r *ChildReconciler[T, CT, CLT]) SetResourceManager(rm *ResourceManager[CT]) {
	if r.stamp != nil {
		panic(fmt.Errorf("cannot call SetResourceManager after a resource manager is defined"))
	}
	r.stamp = rm
}

func (r *ChildReconciler[T, CT, CLT]) Reconcile(ctx context.Context, resource T) (Result, error) {
	r.init()

	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(c, r.ChildType))
	ctx = logr.NewContext(ctx, log)

	child, err := r.reconcile(ctx, resource)
	if resource.GetDeletionTimestamp() != nil {
		return Result{}, err
	}
	if err != nil {
		switch {
		case apierrs.IsAlreadyExists(err):
			// check if the resource blocking create is owned by the reconciled resource.
			// the created child from a previous turn may be slow to appear in the informer cache, but shouldn't appear
			// on the reconciled resource as being not ready.
			apierr := err.(apierrs.APIStatus)
			conflicted := r.ChildType.DeepCopyObject().(CT)
			_ = c.APIReader.Get(ctx, types.NamespacedName{Namespace: resource.GetNamespace(), Name: apierr.Status().Details.Name}, conflicted)
			if r.ourChild(resource, conflicted) {
				// skip updating the reconciled resource's status, fail and try again
				return Result{}, err
			}
			log.Info("unable to reconcile child, not owned", "child", namespaceName(conflicted), "ownerRefs", conflicted.GetOwnerReferences())
			r.ReflectChildStatusOnParent(ctx, resource, child, err)
			return Result{}, nil
		case apierrs.IsInvalid(err):
			r.ReflectChildStatusOnParent(ctx, resource, child, err)
			return Result{}, nil
		}
		if !errors.Is(err, ErrQuiet) {
			log.Error(err, "unable to reconcile child")
		}
		return Result{}, err
	}
	r.ReflectChildStatusOnParent(ctx, resource, child, nil)

	return Result{}, nil
}

func (r *ChildReconciler[T, CT, CLT]) reconcile(ctx context.Context, resource T) (CT, error) {
	var nilCT CT
	log := logr.FromContextOrDiscard(ctx)
	pc := RetrieveOriginalConfigOrDie(ctx)
	c := RetrieveConfigOrDie(ctx)

	actual := r.ChildType.DeepCopyObject().(CT)
	children := r.ChildListType.DeepCopyObject().(CLT)
	if err := c.List(ctx, children, r.listOptions(ctx, resource)...); err != nil {
		return nilCT, err
	}
	items := r.filterChildren(resource, children)
	if len(items) == 1 {
		actual = items[0]
	} else if len(items) > 1 {
		// this shouldn't happen, delete everything to a clean slate
		for _, extra := range items {
			log.Info("deleting extra child", "child", namespaceName(extra))
			if err := c.Delete(ctx, extra); err != nil {
				if !errors.Is(err, ErrQuiet) {
					pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "DeleteFailed",
						"Failed to delete %s %q: %v", typeName(r.ChildType), extra.GetName(), err)
				}
				return nilCT, err
			}
			pc.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted",
				"Deleted %s %q", typeName(r.ChildType), extra.GetName())
		}
	}

	desired, err := r.desiredChild(ctx, resource)
	if err != nil {
		if errors.Is(err, OnlyReconcileChildStatus) {
			return actual, nil
		}
		return nilCT, err
	}
	if !internal.IsNil(desired) {
		if !r.SkipOwnerReference && metav1.GetControllerOfNoCopy(desired) == nil {
			if err := ctrl.SetControllerReference(resource, desired, c.Scheme()); err != nil {
				return nilCT, err
			}
		}
		if !r.ourChild(resource, desired) {
			log.Info("object returned from DesiredChild does not match OurChild, this can result in orphaned children", "child", namespaceName(desired))
		}
	}

	// create/update/delete desired child
	return r.stamp.Manage(ctx, resource, actual, desired)
}

func (r *ChildReconciler[T, CT, CLT]) desiredChild(ctx context.Context, resource T) (CT, error) {
	var nilCT CT

	if resource.GetDeletionTimestamp() != nil {
		// the reconciled resource is pending deletion, cleanup the child resource
		return nilCT, nil
	}

	return r.DesiredChild(ctx, resource)
}

func (r *ChildReconciler[T, CT, CLT]) filterChildren(resource T, children CLT) []CT {
	items := []CT{}
	for _, child := range extractItems[CT](children) {
		if r.ourChild(resource, child) {
			items = append(items, child)
		}
	}
	return items
}

func (r *ChildReconciler[T, CT, CLT]) listOptions(ctx context.Context, resource T) []client.ListOption {
	if r.ListOptions == nil {
		return []client.ListOption{
			client.InNamespace(resource.GetNamespace()),
		}
	}
	return r.ListOptions(ctx, resource)
}

func (r *ChildReconciler[T, CT, CLT]) ourChild(resource T, obj CT) bool {
	if !r.SkipOwnerReference && !metav1.IsControlledBy(obj, resource) {
		return false
	}
	// TODO do we need to remove resources pending deletion?
	if r.OurChild == nil {
		return true
	}
	return r.OurChild(resource, obj)
}

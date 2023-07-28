/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/internal"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ChildSetReconciler is a sub reconciler that manages a set of child resources for a reconciled
// resource. A correlation ID is used to track the desired state of each child resource across
// reconcile requests. A ChildReconciler is created dynamically and reconciled for each desired
// and discovered child resource.
//
// During setup, the child resource type is registered to watch for changes.
type ChildSetReconciler[Type, ChildType client.Object, ChildListType client.ObjectList] struct {
	// Name used to identify this reconciler.  Defaults to `{ChildType}ChildSetReconciler`.  Ideally
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
	// Use of a finalizer implies that SkipOwnerReference is true.
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

	// DesiredChildren returns the set of desired child object for the given reconciled resource,
	// or nil if no children should exist. Each resource returned from this method must be claimed
	// by the OurChild method with a stable, unique identifier returned. The identifier is used to
	// correlate desired and actual child resources.
	//
	// To skip reconciliation of the child resources while still reflecting an existing child's
	// status on the reconciled resource, return OnlyReconcileChildStatus as an error.
	DesiredChildren func(ctx context.Context, resource Type) ([]ChildType, error)

	// ReflectChildrenStatusOnParent updates the reconciled resource's status with values from the
	// child reconciliations. Select types of errors are captured, including:
	//   - apierrs.IsAlreadyExists
	//
	// Most errors are returned directly, skipping this method. The set of handled error types
	// may grow, implementations should be defensive rather than assuming the error type.
	ReflectChildrenStatusOnParent func(ctx context.Context, parent Type, result ChildSetResult[ChildType])

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

	// OurChild is used when there are multiple sources of children of the same ChildType
	// controlled by the same reconciled resource. The function return true for child resources
	// managed by this ChildReconciler. Objects returned from the DesiredChildren function should
	// match this function, otherwise they may be orphaned. If not specified, all children match.
	// Matched child resources must also be uniquely identifiable with the IdentifyChild method.
	//
	// OurChild is required when a Finalizer is defined or SkipOwnerReference is true.
	//
	// +optional
	OurChild func(resource Type, child ChildType) bool

	// IdentifyChild returns a stable identifier for the child resource. The identifier is used to
	// correlate desired child resources with actual child resources. The same value must be returned
	// for an object both before and after it is created on the API server.
	//
	// Non-deterministic IDs will result in the rapid deletion and creation of child resources.
	IdentifyChild func(child ChildType) string

	// Sanitize is called with an object before logging the value. Any value may
	// be returned. A meaningful subset of the resource is typically returned,
	// like the Spec.
	//
	// +optional
	Sanitize func(child ChildType) interface{}

	stamp          *ResourceManager[ChildType]
	lazyInit       sync.Once
	voidReconciler *ChildReconciler[Type, ChildType, ChildListType]
}

func (r *ChildSetReconciler[T, CT, CLT]) init() {
	r.lazyInit.Do(func() {
		var nilCT CT
		if internal.IsNil(r.ChildType) {
			r.ChildType = newEmpty(nilCT).(CT)
		}
		if internal.IsNil(r.ChildListType) {
			var nilCLT CLT
			r.ChildListType = newEmpty(nilCLT).(CLT)
		}
		if r.Name == "" {
			r.Name = fmt.Sprintf("%sChildSetReconciler", typeName(r.ChildType))
		}
		r.stamp = &ResourceManager[CT]{
			Name:                     r.Name,
			Type:                     r.ChildType,
			TrackDesired:             r.SkipOwnerReference,
			HarmonizeImmutableFields: r.HarmonizeImmutableFields,
			MergeBeforeUpdate:        r.MergeBeforeUpdate,
			Sanitize:                 r.Sanitize,
		}
		r.voidReconciler = r.childReconcilerFor(nilCT, nil, "", true)
	})
}

func (r *ChildSetReconciler[T, CT, CLT]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	r.init()

	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(r.ChildType, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}

	if err := r.voidReconciler.SetupWithManager(ctx, mgr, bldr); err != nil {
		return err
	}

	if r.Setup == nil {
		return nil
	}
	return r.Setup(ctx, mgr, bldr)
}

func (r *ChildSetReconciler[T, CT, CLT]) childReconcilerFor(desired CT, desiredErr error, id string, void bool) *ChildReconciler[T, CT, CLT] {
	return &ChildReconciler[T, CT, CLT]{
		Name:               id,
		ChildType:          r.ChildType,
		ChildListType:      r.ChildListType,
		SkipOwnerReference: r.SkipOwnerReference,
		DesiredChild: func(ctx context.Context, resource T) (CT, error) {
			return desired, desiredErr
		},
		ReflectChildStatusOnParent: func(ctx context.Context, parent T, child CT, err error) {
			result := retrieveChildSetResult[CT](ctx)
			result.Children = append(result.Children, ChildSetPartialResult[CT]{
				Id:    id,
				Child: child,
				Err:   err,
			})
			stashChildSetResult(ctx, result)
		},
		HarmonizeImmutableFields: r.HarmonizeImmutableFields,
		MergeBeforeUpdate:        r.MergeBeforeUpdate,
		ListOptions:              r.ListOptions,
		OurChild: func(resource T, child CT) bool {
			if r.OurChild != nil && !r.OurChild(resource, child) {
				return false
			}
			return void || id == r.IdentifyChild(child)
		},
		Sanitize: r.Sanitize,
	}
}

func (r *ChildSetReconciler[T, CT, CLT]) validate(ctx context.Context) error {
	// default implicit values
	if r.Finalizer != "" {
		r.SkipOwnerReference = true
	}

	// require DesiredChildren
	if r.DesiredChildren == nil {
		return fmt.Errorf("ChildSetReconciler %q must implement DesiredChildren", r.Name)
	}

	// require ReflectChildrenStatusOnParent
	if r.ReflectChildrenStatusOnParent == nil {
		return fmt.Errorf("ChildSetReconciler %q must implement ReflectChildrenStatusOnParent", r.Name)
	}

	if r.OurChild == nil && r.SkipOwnerReference {
		// OurChild is required when SkipOwnerReference is true
		return fmt.Errorf("ChildSetReconciler %q must implement OurChild since owner references are not used", r.Name)
	}

	// require IdentifyChild
	if r.IdentifyChild == nil {
		return fmt.Errorf("ChildSetReconciler %q must implement IdentifyChild", r.Name)
	}

	return nil
}

func (r *ChildSetReconciler[T, CT, CLT]) Reconcile(ctx context.Context, resource T) (Result, error) {
	r.init()

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	cr, err := r.composeChildReconcilers(ctx, resource)
	if err != nil {
		return Result{}, err
	}
	result, err := cr.Reconcile(ctx, resource)
	r.reflectStatus(ctx, resource)
	return result, err
}

func (r *ChildSetReconciler[T, CT, CLT]) composeChildReconcilers(ctx context.Context, resource T) (SubReconciler[T], error) {
	c := RetrieveConfigOrDie(ctx)

	childIDs := sets.NewString()
	desiredChildren, desiredChildrenErr := r.DesiredChildren(ctx, resource)
	if desiredChildrenErr != nil && !errors.Is(desiredChildrenErr, OnlyReconcileChildStatus) {
		return nil, desiredChildrenErr
	}

	desiredChildByID := map[string]CT{}
	for _, child := range desiredChildren {
		id := r.IdentifyChild(child)
		if id == "" {
			return nil, fmt.Errorf("desired child id may not be empty")
		}
		if childIDs.Has(id) {
			return nil, fmt.Errorf("duplicate child id found: %s", id)
		}
		childIDs.Insert(id)
		desiredChildByID[id] = child
	}

	children := r.ChildListType.DeepCopyObject().(CLT)
	if err := c.List(ctx, children, r.voidReconciler.listOptions(ctx, resource)...); err != nil {
		return nil, err
	}
	for _, child := range extractItems[CT](children) {
		if !r.voidReconciler.ourChild(resource, child) {
			continue
		}
		id := r.IdentifyChild(child)
		childIDs.Insert(id)
	}

	sequence := Sequence[T]{}
	for _, id := range childIDs.List() {
		child := desiredChildByID[id]
		cr := r.childReconcilerFor(child, desiredChildrenErr, id, false)
		cr.SetResourceManager(r.stamp)
		sequence = append(sequence, cr)
	}

	if r.Finalizer != "" {
		return &WithFinalizer[T]{
			Finalizer:  r.Finalizer,
			Reconciler: sequence,
		}, nil
	}
	return sequence, nil
}

func (r *ChildSetReconciler[T, CT, CLT]) reflectStatus(ctx context.Context, parent T) {
	result := clearChildSetResult[CT](ctx)
	r.ReflectChildrenStatusOnParent(ctx, parent, result)
}

type ChildSetResult[T client.Object] struct {
	Children []ChildSetPartialResult[T]
}

type ChildSetPartialResult[T client.Object] struct {
	Id    string
	Child T
	Err   error
}

func (r *ChildSetResult[T]) AggregateError() error {
	var errs []error
	for _, childResult := range r.Children {
		errs = append(errs, childResult.Err)
	}
	return utilerrors.NewAggregate(errs)
}

const childSetResultStashKey StashKey = "reconciler-runtime:childSetResult"

func retrieveChildSetResult[T client.Object](ctx context.Context) ChildSetResult[T] {
	value := RetrieveValue(ctx, childSetResultStashKey)
	if result, ok := value.(ChildSetResult[T]); ok {
		return result
	}
	return ChildSetResult[T]{}
}

func stashChildSetResult[T client.Object](ctx context.Context, result ChildSetResult[T]) {
	StashValue(ctx, childSetResultStashKey, result)
}

func clearChildSetResult[T client.Object](ctx context.Context) ChildSetResult[T] {
	value := ClearValue(ctx, childSetResultStashKey)
	if result, ok := value.(ChildSetResult[T]); ok {
		return result
	}
	return ChildSetResult[T]{}
}

/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/vmware-labs/reconciler-runtime/client"
	"github.com/vmware-labs/reconciler-runtime/manager"
	"github.com/vmware-labs/reconciler-runtime/tracker"
)

var (
	_ reconcile.Reconciler = (*ParentReconciler)(nil)
)

// Config holds common resources for controllers. The configuration may be
// passed to sub-reconcilers.
type Config struct {
	client.DuckClient
	APIReader client.DuckReader
	Recorder  record.EventRecorder
	Log       logr.Logger
	Tracker   tracker.Tracker
}

// NewConfig creates a Config for a specific API type. Typically passed into a
// reconciler.
func NewConfig(mgr manager.SuperManager, apiType client.Object, syncPeriod time.Duration) Config {
	name := typeName(apiType)
	log := ctrl.Log.WithName("controllers").WithName(name)
	return Config{
		DuckClient: client.NewDuckClient(mgr.GetClient()),
		APIReader:  client.NewDuckReader(mgr.GetAPIReader()),
		Recorder:   mgr.GetEventRecorderFor(name),
		Log:        log,
		Tracker: tracker.NewWatcher(mgr, syncPeriod, log.WithName("tracker"), func(by client.Object, t tracker.Tracker) handler.EventHandler {
			return EnqueueTracked(by, t, mgr.GetScheme())
		}),
	}
}

// ParentReconciler is a controller-runtime reconciler that reconciles a given
// existing resource. The ParentType resource is fetched for the reconciler
// request and passed in turn to each SubReconciler. Finally, the reconciled
// resource's status is compared with the original status, updating the API
// server if needed.
type ParentReconciler struct {
	// Type of resource to reconcile
	Type client.Object

	// Reconciler is called for each reconciler request with the parent
	// resource being reconciled. Typically, Reconciler is a Sequence of
	// multiple SubReconcilers.
	Reconciler SubReconciler

	Config
}

func (r *ParentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	ctx = StashParentType(ctx, r.Type)
	ctx = StashCastParentType(ctx, r.Type)
	bldr := ctrl.NewControllerManagedBy(mgr).For(r.Type)
	if err := r.Reconciler.SetupWithManager(ctx, mgr, bldr); err != nil {
		return err
	}
	return bldr.Complete(r)
}

func (r *ParentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = WithStash(ctx)
	log := r.Log.WithValues("request", req.NamespacedName)

	ctx = StashParentType(ctx, r.Type)
	ctx = StashCastParentType(ctx, r.Type)
	originalParent := r.Type.DeepCopyObject().(client.Object)

	if err := r.Get(ctx, req.NamespacedName, originalParent); err != nil {
		if apierrs.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch resource")
		return ctrl.Result{}, err
	}
	parent := originalParent.DeepCopyObject().(client.Object)

	if defaulter, ok := parent.(webhook.Defaulter); ok {
		// parent.Default()
		defaulter.Default()
	}
	if initializeConditions := reflect.ValueOf(parent).Elem().FieldByName("Status").Addr().MethodByName("InitializeConditions"); initializeConditions.Kind() == reflect.Func {
		// parent.Status.InitializeConditions()
		initializeConditions.Call([]reflect.Value{})
	}

	result, err := r.reconcile(ctx, parent)

	// check if status has changed before updating
	if !equality.Semantic.DeepEqual(r.status(parent), r.status(originalParent)) && parent.GetDeletionTimestamp() == nil {
		// update status
		log.Info("updating status", "diff", cmp.Diff(r.status(originalParent), r.status(parent)))
		if updateErr := r.Status().Update(ctx, parent); updateErr != nil {
			log.Error(updateErr, "unable to update status", typeName(r.Type), parent)
			r.Recorder.Eventf(parent, corev1.EventTypeWarning, "StatusUpdateFailed",
				"Failed to update status: %v", updateErr)
			return ctrl.Result{}, updateErr
		}
		r.Recorder.Eventf(parent, corev1.EventTypeNormal, "StatusUpdated",
			"Updated status")
	}

	// return original reconcile result
	return result, err
}

func (r *ParentReconciler) reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	if parent.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, nil
	}

	result, err := r.Reconciler.Reconcile(ctx, parent)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.copyGeneration(parent)

	return result, nil
}

func (r *ParentReconciler) copyGeneration(obj client.Object) {
	// obj.Status.ObservedGeneration = obj.Generation
	objVal := reflect.ValueOf(obj).Elem()
	generation := objVal.FieldByName("Generation").Int()
	objVal.FieldByName("Status").FieldByName("ObservedGeneration").SetInt(generation)
}

func (r *ParentReconciler) status(obj client.Object) interface{} {
	return reflect.ValueOf(obj).Elem().FieldByName("Status").Addr().Interface()
}

const parentTypeStashKey StashKey = "reconciler-runtime:parentType"
const castParentTypeStashKey StashKey = "reconciler-runtime:castParentType"

func StashParentType(ctx context.Context, parentType client.Object) context.Context {
	return context.WithValue(ctx, parentTypeStashKey, parentType)
}

func StashCastParentType(ctx context.Context, currentType client.Object) context.Context {
	return context.WithValue(ctx, castParentTypeStashKey, currentType)
}

func RetrieveParentType(ctx context.Context) client.Object {
	value := ctx.Value(parentTypeStashKey)
	if parentType, ok := value.(client.Object); ok {
		return parentType
	}
	return nil
}

func RetrieveCastParentType(ctx context.Context) client.Object {
	value := ctx.Value(castParentTypeStashKey)
	if currentType, ok := value.(client.Object); ok {
		return currentType
	}
	return nil
}

// SubReconciler are participants in a larger reconciler request. The resource
// being reconciled is passed directly to the sub reconciler. The resource's
// status can be mutated to reflect the current state.
type SubReconciler interface {
	SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error
	Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error)
}

var (
	_ SubReconciler = (*SyncReconciler)(nil)
	_ SubReconciler = (*ChildReconciler)(nil)
	_ SubReconciler = (Sequence)(nil)
	_ SubReconciler = (*CastParent)(nil)
)

// SyncReconciler is a sub reconciler for custom reconciliation logic. No
// behavior is defined directly.
type SyncReconciler struct {
	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error

	// Sync does whatever work is necessary for the reconciler
	//
	// Expected function signature:
	//     func(ctx context.Context, parent client.Object) error
	//     func(ctx context.Context, parent client.Object) (ctrl.Result, error)
	Sync interface{}

	Config
}

func (r *SyncReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Setup == nil {
		return nil
	}
	if err := r.validate(ctx); err != nil {
		return err
	}
	return r.Setup(ctx, mgr, bldr)
}

func (r *SyncReconciler) validate(ctx context.Context) error {
	// validate Sync function signature:
	//     func(ctx context.Context, parent client.Object) error
	//     func(ctx context.Context, parent client.Object) (ctrl.Result, error)
	if r.Sync == nil {
		return fmt.Errorf("SyncReconciler must implement Sync")
	} else {
		castParentType := RetrieveCastParentType(ctx)
		fn := reflect.TypeOf(r.Sync)
		err := fmt.Errorf("SyncReconciler must implement Sync: func(context.Context, %s) error | func(context.Context, %s) (ctrl.Result, error), found: %s", reflect.TypeOf(castParentType), reflect.TypeOf(castParentType), fn)
		if fn.NumIn() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castParentType).AssignableTo(fn.In(1)) {
			return err
		}
		switch fn.NumOut() {
		case 1:
			if !reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.Out(0)) {
				return err
			}
		case 2:
			if !reflect.TypeOf(ctrl.Result{}).AssignableTo(fn.Out(0)) ||
				!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.Out(1)) {
				return err
			}
		default:
			return err
		}
	}

	return nil
}

func (r *SyncReconciler) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	result, err := r.sync(ctx, parent)
	if err != nil {
		r.Log.Error(err, "unable to sync", typeName(parent), parent)
		return ctrl.Result{}, err
	}

	return result, nil
}

func (r *SyncReconciler) sync(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	fn := reflect.ValueOf(r.Sync)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(parent),
	})
	result := ctrl.Result{}
	var err error
	switch len(out) {
	case 2:
		result = out[0].Interface().(ctrl.Result)
		if !out[1].IsNil() {
			err = out[1].Interface().(error)
		}
	case 1:
		if !out[0].IsNil() {
			err = out[0].Interface().(error)
		}
	}
	return result, err
}

var (
	OnlyReconcileChildStatus = errors.New("skip reconciler create/update/delete behavior for the child resource, while still reflecting the existing child's status on the parent")
)

// ChildReconciler is a sub reconciler that manages a single child resource for
// a parent. The reconciler will ensure that exactly one child will match the
// desired state by:
// - creating a child if none exists
// - updating an existing child
// - removing an unneeded child
// - removing extra children
//
// The flow for each reconciliation request is:
// - DesiredChild
// - if child is desired:
//    - HarmonizeImmutableFields (optional)
//    - SemanticEquals
//    - MergeBeforeUpdate
// - ReflectChildStatusOnParent
//
// During setup, the child resource type is registered to watch for changes. A
// field indexer is configured for the owner on the IndexField.
type ChildReconciler struct {
	// ChildType is the resource being created/updated/deleted by the
	// reconciler. For example, a parent Deployment would have a ReplicaSet as a
	// child.
	ChildType client.Object
	// ChildListType is the listing type for the child type. For example,
	// PodList is the list type for Pod
	ChildListType client.ObjectList

	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error

	// DesiredChild returns the desired child object for the given parent
	// object, or nil if the child should not exist.
	//
	// To skip reconciliation of the child resource while still reflecting an
	// existing child's status on the parent, return OnlyReconcileChildStatus as
	// an error.
	//
	// Expected function signature:
	//     func(ctx context.Context, parent client.Object) (client.Object, error)
	DesiredChild interface{}

	// ReflectChildStatusOnParent updates the parent object's status with values
	// from the child. Select types of error are passed, including:
	// - apierrs.IsConflict
	//
	// Expected function signature:
	//     func(parent, child client.Object, err error)
	ReflectChildStatusOnParent interface{}

	// HarmonizeImmutableFields allows fields that are immutable on the current
	// object to be copied to the desired object in order to avoid creating
	// updates which are guaranteed to fail.
	//
	// Expected function signature:
	//     func(current, desired client.Object)
	//
	// +optional
	HarmonizeImmutableFields interface{}

	// MergeBeforeUpdate copies desired fields on to the current object before
	// calling update. Typically fields to copy are the Spec, Labels and
	// Annotations.
	//
	// Expected function signature:
	//     func(current, desired client.Object)
	MergeBeforeUpdate interface{}

	// SemanticEquals compares two child resources returning true if there is a
	// meaningful difference that should trigger an update.
	//
	// Expected function signature:
	//     func(a1, a2 client.Object) bool
	SemanticEquals interface{}

	// Sanitize is called with an object before logging the value. Any value may
	// be returned. A meaningful subset of the resource is typically returned,
	// like the Spec.
	//
	// Expected function signature:
	//     func(child client.Object) interface{}
	//
	// +optional
	Sanitize interface{}

	Config

	// IndexField is used to index objects of the child's type based on their
	// controlling owner. This field needs to be unique within the manager.
	IndexField string

	// mutationCache holds patches received from updates to a resource made by
	// mutation webhooks. This cache is used to avoid unnecessary update calls
	// that would actually have no effect.
	mutationCache *cache.Expiring
}

func (r *ChildReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if err := r.validate(ctx); err != nil {
		return err
	}

	bldr.Owns(r.ChildType)

	parentType := RetrieveParentType(ctx)
	if err := IndexControllersOfType(ctx, mgr, r.IndexField, parentType, r.ChildType, r.Scheme()); err != nil {
		return err
	}

	if r.Setup == nil {
		return nil
	}
	return r.Setup(ctx, mgr, bldr)
}

func (r *ChildReconciler) validate(ctx context.Context) error {
	castParentType := RetrieveCastParentType(ctx)

	// validate IndexField value
	if r.IndexField == "" {
		return fmt.Errorf("IndexField must be defined")
	}

	// validate ChildType value
	if r.ChildType == nil {
		return fmt.Errorf("ChildType must be defined")
	}

	// validate ChildListType value
	if r.ChildListType == nil {
		return fmt.Errorf("ChildListType must be defined")
	}

	// validate DesiredChild function signature:
	//     func(ctx context.Context, parent client.Object) (client.Object, error)
	if r.DesiredChild == nil {
		return fmt.Errorf("ChildReconciler must implement DesiredChild")
	} else {
		fn := reflect.TypeOf(r.DesiredChild)
		if fn.NumIn() != 2 || fn.NumOut() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castParentType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.Out(0)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.Out(1)) {
			return fmt.Errorf("ChildReconciler must implement DesiredChild: func(context.Context, %s) (%s, error), found: %s", reflect.TypeOf(castParentType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate ReflectChildStatusOnParent function signature:
	//     func(parent, child client.Object, err error)
	if r.ReflectChildStatusOnParent == nil {
		return fmt.Errorf("ChildReconciler must implement ReflectChildStatusOnParent")
	} else {
		fn := reflect.TypeOf(r.ReflectChildStatusOnParent)
		if fn.NumIn() != 3 || fn.NumOut() != 0 ||
			!reflect.TypeOf(castParentType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.In(2)) {
			return fmt.Errorf("ChildReconciler must implement ReflectChildStatusOnParent: func(%s, %s, error), found: %s", reflect.TypeOf(castParentType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate HarmonizeImmutableFields function signature:
	//     nil
	//     func(current, desired client.Object)
	if r.HarmonizeImmutableFields != nil {
		fn := reflect.TypeOf(r.HarmonizeImmutableFields)
		if fn.NumIn() != 2 || fn.NumOut() != 0 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) {
			return fmt.Errorf("ChildReconciler must implement HarmonizeImmutableFields: nil | func(%s, %s), found: %s", reflect.TypeOf(r.ChildType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate MergeBeforeUpdate function signature:
	//     func(current, desired client.Object)
	if r.MergeBeforeUpdate == nil {
		return fmt.Errorf("ChildReconciler must implement MergeBeforeUpdate")
	} else {
		fn := reflect.TypeOf(r.MergeBeforeUpdate)
		if fn.NumIn() != 2 || fn.NumOut() != 0 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) {
			return fmt.Errorf("ChildReconciler must implement MergeBeforeUpdate: nil | func(%s, %s), found: %s", reflect.TypeOf(r.ChildType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate SemanticEquals function signature:
	//     func(a1, a2 client.Object) bool
	if r.SemanticEquals == nil {
		return fmt.Errorf("ChildReconciler must implement SemanticEquals")
	} else {
		fn := reflect.TypeOf(r.SemanticEquals)
		if fn.NumIn() != 2 || fn.NumOut() != 1 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			fn.Out(0).Kind() != reflect.Bool {
			return fmt.Errorf("ChildReconciler must implement SemanticEquals: nil | func(%s, %s) bool, found: %s", reflect.TypeOf(r.ChildType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate Sanitize function signature:
	//     nil
	//     func(child client.Object) interface{}
	if r.Sanitize != nil {
		fn := reflect.TypeOf(r.Sanitize)
		if fn.NumIn() != 1 || fn.NumOut() != 1 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) {
			return fmt.Errorf("ChildReconciler must implement Sanitize: nil | func(%s) interface{}, found: %s", reflect.TypeOf(r.ChildType), fn)
		}
	}

	return nil
}

func (r *ChildReconciler) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	if r.mutationCache == nil {
		r.mutationCache = cache.NewExpiring()
	}

	child, err := r.reconcile(ctx, parent)
	if err != nil {
		parentType := RetrieveParentType(ctx)
		if apierrs.IsAlreadyExists(err) {
			// check if the resource blocking create is owned by the parent.
			// the created child from a previous turn may be slow to appear in the informer cache, but shouldn't appear
			// on the parent as being not ready.
			apierr := err.(apierrs.APIStatus)
			conflicted := r.ChildType.DeepCopyObject().(client.Object)
			_ = r.APIReader.Get(ctx, types.NamespacedName{Namespace: parent.GetNamespace(), Name: apierr.Status().Details.Name}, conflicted)
			if metav1.IsControlledBy(conflicted, parent) {
				// skip updating the parent's status, fail and try again
				return ctrl.Result{}, err
			}
			r.Log.Info("unable to reconcile child, not owned", typeName(parentType), parent, typeName(r.ChildType), r.sanitize(child))
			r.reflectChildStatusOnParent(parent, child, err)
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "unable to reconcile child", typeName(parentType), parent)
		return ctrl.Result{}, err
	}
	r.reflectChildStatusOnParent(parent, child, err)

	return ctrl.Result{}, nil
}

func (r *ChildReconciler) reconcile(ctx context.Context, parent client.Object) (client.Object, error) {
	actual := r.ChildType.DeepCopyObject().(client.Object)
	children := r.ChildListType.DeepCopyObject().(client.ObjectList)
	if err := r.List(ctx, children, client.InNamespace(parent.GetNamespace()), client.MatchingFields{r.IndexField: parent.GetName()}); err != nil {
		return nil, err
	}
	// TODO do we need to remove resources pending deletion?
	items := r.items(children)
	if len(items) == 1 {
		actual = items[0]
	} else if len(items) > 1 {
		// this shouldn't happen, delete everything to a clean slate
		for _, extra := range items {
			r.Log.Info("deleting extra child", typeName(r.ChildType), r.sanitize(extra))
			if err := r.Delete(ctx, extra); err != nil {
				r.Recorder.Eventf(parent, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(r.ChildType), extra.GetName(), err)
				return nil, err
			}
			r.Recorder.Eventf(parent, corev1.EventTypeNormal, "Deleted",
				"Deleted %s %q", typeName(r.ChildType), extra.GetName())
		}
	}

	desired, err := r.desiredChild(ctx, parent)
	if err != nil {
		if errors.Is(err, OnlyReconcileChildStatus) {
			return actual, nil
		}
		return nil, err
	}
	if desired != nil {
		if err := ctrl.SetControllerReference(parent, desired, r.Scheme()); err != nil {
			return nil, err
		}
	}

	// delete child if no longer needed
	if desired == nil {
		if !actual.GetCreationTimestamp().Time.IsZero() {
			r.Log.Info("deleting unwanted child", typeName(r.ChildType), r.sanitize(actual))
			if err := r.Delete(ctx, actual); err != nil {
				r.Log.Error(err, "unable to delete unwanted child", typeName(r.ChildType), r.sanitize(actual))
				r.Recorder.Eventf(parent, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(r.ChildType), actual.GetName(), err)
				return nil, err
			}
			r.Recorder.Eventf(parent, corev1.EventTypeNormal, "Deleted",
				"Deleted %s %q", typeName(r.ChildType), actual.GetName())
		}
		return nil, nil
	}

	// create child if it doesn't exist
	if actual.GetName() == "" {
		r.Log.Info("creating child", typeName(r.ChildType), r.sanitize(desired))
		if err := r.Create(ctx, desired); err != nil {
			r.Log.Error(err, "unable to create child", typeName(r.ChildType), r.sanitize(desired))
			r.Recorder.Eventf(parent, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create %s %q: %v", typeName(r.ChildType), desired.GetName(), err)
			return nil, err
		}
		r.Recorder.Eventf(parent, corev1.EventTypeNormal, "Created",
			"Created %s %q", typeName(r.ChildType), desired.GetName())
		return desired, nil
	}

	// overwrite fields that should not be mutated
	r.harmonizeImmutableFields(actual, desired)

	// lookup and apply remote mutations
	desiredPatched := desired.DeepCopyObject().(client.Object)
	if patch, ok := r.mutationCache.Get(actual.GetUID()); ok {
		// the only object added to the cache is *Patch
		err := patch.(*Patch).Apply(desiredPatched)
		if err != nil {
			// there's not much we can do, but let the normal update proceed
			r.Log.Info("unable to patch desired child from mutation cache")
		}
	}

	if r.semanticEquals(desiredPatched, actual) {
		// child is unchanged
		return actual, nil
	}

	// update child with desired changes
	current := actual.DeepCopyObject().(client.Object)
	r.mergeBeforeUpdate(current, desiredPatched)
	r.Log.Info("reconciling child", "diff", cmp.Diff(r.sanitize(actual), r.sanitize(current)))
	if err := r.Update(ctx, current); err != nil {
		r.Log.Error(err, "unable to update child", typeName(r.ChildType), r.sanitize(current))
		r.Recorder.Eventf(parent, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update %s %q: %v", typeName(r.ChildType), current.GetName(), err)
		return nil, err
	}
	if r.semanticEquals(desired, current) {
		r.mutationCache.Delete(current.GetUID())
	} else {
		base := current.DeepCopyObject().(client.Object)
		r.mergeBeforeUpdate(base, desired)
		patch, err := NewPatch(base, current)
		if err != nil {
			r.Log.Error(err, "unable to generate mutation patch", "snapshot", r.sanitize(desired), "base", r.sanitize(base))
		} else {
			r.mutationCache.Set(current.GetUID(), patch, 1*time.Hour)
		}
	}
	r.Recorder.Eventf(parent, corev1.EventTypeNormal, "Updated",
		"Updated %s %q", typeName(r.ChildType), current.GetName())

	return current, nil
}

func (r *ChildReconciler) semanticEquals(a1, a2 client.Object) bool {
	fn := reflect.ValueOf(r.SemanticEquals)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(a1),
		reflect.ValueOf(a2),
	})
	return out[0].Bool()
}

func (r *ChildReconciler) desiredChild(ctx context.Context, parent client.Object) (client.Object, error) {
	fn := reflect.ValueOf(r.DesiredChild)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(parent),
	})
	var obj client.Object
	if !out[0].IsNil() {
		obj = out[0].Interface().(client.Object)
	}
	var err error
	if !out[1].IsNil() {
		err = out[1].Interface().(error)
	}
	return obj, err
}

func (r *ChildReconciler) reflectChildStatusOnParent(parent, child client.Object, err error) {
	fn := reflect.ValueOf(r.ReflectChildStatusOnParent)
	args := []reflect.Value{
		reflect.ValueOf(parent),
		reflect.ValueOf(child),
		reflect.ValueOf(err),
	}
	if parent == nil {
		args[0] = reflect.New(fn.Type().In(0)).Elem()
	}
	if child == nil {
		args[1] = reflect.New(fn.Type().In(1)).Elem()
	}
	if err == nil {
		args[2] = reflect.New(fn.Type().In(2)).Elem()
	}
	fn.Call(args)
}

func (r *ChildReconciler) harmonizeImmutableFields(current, desired client.Object) {
	if r.HarmonizeImmutableFields == nil {
		return
	}
	fn := reflect.ValueOf(r.HarmonizeImmutableFields)
	fn.Call([]reflect.Value{
		reflect.ValueOf(current),
		reflect.ValueOf(desired),
	})
}

func (r *ChildReconciler) mergeBeforeUpdate(current, desired client.Object) {
	fn := reflect.ValueOf(r.MergeBeforeUpdate)
	fn.Call([]reflect.Value{
		reflect.ValueOf(current),
		reflect.ValueOf(desired),
	})
}

func (r *ChildReconciler) sanitize(child client.Object) interface{} {
	if r.Sanitize == nil {
		return child
	}
	if child == nil {
		return nil
	}
	fn := reflect.ValueOf(r.Sanitize)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(child),
	})
	var sanitized interface{}
	if !out[0].IsNil() {
		sanitized = out[0].Interface()
	}
	return sanitized
}

func (r *ChildReconciler) items(children client.ObjectList) []client.Object {
	childrenValue := reflect.ValueOf(children).Elem()
	itemsValue := childrenValue.FieldByName("Items")
	items := make([]client.Object, itemsValue.Len())
	for i := range items {
		items[i] = itemsValue.Index(i).Addr().Interface().(client.Object)
	}
	return items
}

// Sequence is a collection of SubReconcilers called in order. If a
// reconciler errs, further reconcilers are skipped.
type Sequence []SubReconciler

func (r Sequence) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	for _, reconciler := range r {
		err := reconciler.SetupWithManager(ctx, mgr, bldr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Sequence) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	aggregateResult := ctrl.Result{}
	for _, reconciler := range r {
		result, err := reconciler.Reconcile(ctx, parent)
		if err != nil {
			return ctrl.Result{}, err
		}
		aggregateResult = r.aggregateResult(result, aggregateResult)
	}

	return aggregateResult, nil
}

func (r Sequence) aggregateResult(result, aggregate ctrl.Result) ctrl.Result {
	if result.RequeueAfter != 0 && (aggregate.RequeueAfter == 0 || result.RequeueAfter < aggregate.RequeueAfter) {
		aggregate.RequeueAfter = result.RequeueAfter
	}
	if result.Requeue {
		aggregate.Requeue = true
	}

	return aggregate
}

// CastParent casts the ParentReconciler's type by projecting the resource data
// onto a new struct. Casting the parent resource is useful to create cross
// cutting reconcilers that can operate on common portion of multiple parent
// resources, commonly referred to as a duck type.
//
// JSON encoding is used as the intermediate representation. Operations on a
// cast parent are read-only. Attempts to mutate the parent will result in the
// reconciler erring.
type CastParent struct {
	// Type of resource to reconcile
	Type client.Object

	// Reconciler is called for each reconciler request with the parent
	// resource being reconciled. Typically a Sequence is used to compose
	// multiple SubReconcilers.
	Reconciler SubReconciler
}

func (r *CastParent) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if err := r.validate(ctx); err != nil {
		return err
	}
	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *CastParent) validate(ctx context.Context) error {
	// validate Type value
	if r.Type == nil {
		return fmt.Errorf("Type must be defined")
	}

	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("Reconciler must be defined")
	}

	return nil
}

func (r *CastParent) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	ctx, castParent, err := r.cast(ctx, parent)
	if err != nil {
		return ctrl.Result{}, err
	}
	castOriginal := castParent.DeepCopyObject().(client.Object)
	result, err := r.Reconciler.Reconcile(ctx, castParent)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !equality.Semantic.DeepEqual(castParent, castOriginal) {
		// patch the parent object with the updated duck values
		patch, err := NewPatch(castOriginal, castParent)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = patch.Apply(parent)
		if err != nil {
			return ctrl.Result{}, err
		}

	}
	return result, nil
}

func (r *CastParent) cast(ctx context.Context, parent client.Object) (context.Context, client.Object, error) {
	data, err := json.Marshal(parent)
	if err != nil {
		return nil, nil, err
	}
	castParent := r.Type.DeepCopyObject().(client.Object)
	err = json.Unmarshal(data, castParent)
	if err != nil {
		return nil, nil, err
	}
	ctx = StashCastParentType(ctx, castParent)
	return ctx, castParent, nil
}

func typeName(i interface{}) string {
	t := reflect.TypeOf(i)
	// TODO do we need this?
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// MergeMaps flattens a sequence of maps into a single map. Keys in latter maps
// overwrite previous keys. None of the arguments are mutated.
func MergeMaps(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

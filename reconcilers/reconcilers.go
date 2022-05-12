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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/vmware-labs/reconciler-runtime/tracker"
)

var (
	_ reconcile.Reconciler = (*ParentReconciler)(nil)
)

// Config holds common resources for controllers. The configuration may be
// passed to sub-reconcilers.
type Config struct {
	client.Client
	APIReader client.Reader
	Recorder  record.EventRecorder
	Tracker   tracker.Tracker

	// Deprecated: use a logger from the context instead. For example:
	//  * `log, err := logr.FromContext(ctx)`
	//  * `log := logr.FromContextOrDiscard(ctx)`
	Log logr.Logger
}

func (c Config) IsEmpty() bool {
	return c == Config{}
}

// WithCluster extends the config to access a new cluster.
func (c Config) WithCluster(cluster cluster.Cluster) Config {
	return Config{
		Client:    cluster.GetClient(),
		APIReader: cluster.GetAPIReader(),
		Recorder:  cluster.GetEventRecorderFor("controller"),
		Log:       c.Log,
		Tracker:   c.Tracker,
	}
}

// TrackAndGet tracks the resources for changes and returns the current value. The track is
// registered even when the resource does not exists so that its creation can be tracked.
//
// Equivlent to calling both `c.Tracker.Track(...)` and `c.Client.Get(...)`
func (c Config) TrackAndGet(ctx context.Context, key types.NamespacedName, obj client.Object) error {
	c.Tracker.Track(
		ctx,
		tracker.NewKey(gvk(obj, c.Scheme()), key),
		RetrieveRequest(ctx).NamespacedName,
	)
	return c.Get(ctx, key, obj)
}

// NewConfig creates a Config for a specific API type. Typically passed into a
// reconciler.
func NewConfig(mgr ctrl.Manager, apiType client.Object, syncPeriod time.Duration) Config {
	name := typeName(apiType)
	log := newWarnOnceLogger(ctrl.Log.WithName("controllers").WithName(name))
	return Config{
		Log:     log,
		Tracker: tracker.New(2 * syncPeriod),
	}.WithCluster(mgr)
}

// ParentReconciler is a controller-runtime reconciler that reconciles a given
// existing resource. The ParentType resource is fetched for the reconciler
// request and passed in turn to each SubReconciler. Finally, the reconciled
// resource's status is compared with the original status, updating the API
// server if needed.
type ParentReconciler struct {
	// Name used to identify this reconciler.  Defaults to `{Type}ParentReconciler`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// Type of resource to reconcile
	Type client.Object

	// Reconciler is called for each reconciler request with the parent
	// resource being reconciled. Typically, Reconciler is a Sequence of
	// multiple SubReconcilers.
	Reconciler SubReconciler

	Config Config
}

func (r *ParentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if r.Name == "" {
		r.Name = fmt.Sprintf("%sParentReconciler", typeName(r.Type))
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("parentType", gvk(r.Type, r.Config.Scheme()))
	ctx = logr.NewContext(ctx, log)

	ctx = StashConfig(ctx, r.Config)
	ctx = StashParentConfig(ctx, r.Config)
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

	c := r.Config

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("parentType", gvk(r.Type, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	ctx = StashRequest(ctx, req)
	ctx = StashConfig(ctx, c)
	ctx = StashParentConfig(ctx, c)
	ctx = StashParentType(ctx, r.Type)
	ctx = StashCastParentType(ctx, r.Type)
	originalParent := r.Type.DeepCopyObject().(client.Object)

	if err := c.Get(ctx, req.NamespacedName, originalParent); err != nil {
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

	r.initializeConditions(parent)
	result, err := r.reconcile(ctx, parent)

	if r.hasStatus(originalParent) {
		// restore last transition time for unchanged conditions
		r.syncLastTransitionTime(r.conditions(parent), r.conditions(originalParent))

		// check if status has changed before updating
		parentStatus, originalParentStatus := r.status(parent), r.status(originalParent)
		if !equality.Semantic.DeepEqual(parentStatus, originalParentStatus) && parent.GetDeletionTimestamp() == nil {
			// update status
			log.Info("updating status", "diff", cmp.Diff(originalParentStatus, parentStatus))
			if updateErr := c.Status().Update(ctx, parent); updateErr != nil {
				log.Error(updateErr, "unable to update status")
				c.Recorder.Eventf(parent, corev1.EventTypeWarning, "StatusUpdateFailed",
					"Failed to update status: %v", updateErr)
				return ctrl.Result{}, updateErr
			}
			c.Recorder.Eventf(parent, corev1.EventTypeNormal, "StatusUpdated",
				"Updated status")
		}
	}
	// return original reconcile result
	return result, err
}

func (r *ParentReconciler) reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	if parent.GetDeletionTimestamp() != nil && len(parent.GetFinalizers()) == 0 {
		// resource is being deleted and has no pending finalizers, nothing to do
		return ctrl.Result{}, nil
	}

	result, err := r.Reconciler.Reconcile(ctx, parent)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.copyGeneration(parent)
	return result, nil
}

func (r *ParentReconciler) initializeConditions(obj client.Object) {
	status := r.status(obj)
	if status == nil {
		return
	}
	initializeConditions := reflect.ValueOf(status).MethodByName("InitializeConditions")
	if !initializeConditions.IsValid() {
		return
	}
	if t := initializeConditions.Type(); t.Kind() != reflect.Func || t.NumIn() != 0 || t.NumOut() != 0 {
		return
	}
	initializeConditions.Call([]reflect.Value{})
}

func (r *ParentReconciler) conditions(obj client.Object) []metav1.Condition {
	// return obj.Status.Conditions
	status := r.status(obj)
	if status == nil {
		return nil
	}
	statusValue := reflect.ValueOf(status).Elem()
	conditionsValue := statusValue.FieldByName("Conditions")
	if !conditionsValue.IsValid() || conditionsValue.IsZero() {
		return nil
	}
	conditions, ok := conditionsValue.Interface().([]metav1.Condition)
	if !ok {
		return nil
	}
	return conditions
}

func (r *ParentReconciler) copyGeneration(obj client.Object) {
	// obj.Status.ObservedGeneration = obj.Generation
	status := r.status(obj)
	if status == nil {
		return
	}
	statusValue := reflect.ValueOf(status).Elem()
	if !statusValue.IsValid() {
		return
	}
	observedGenerationValue := statusValue.FieldByName("ObservedGeneration")
	if observedGenerationValue.Kind() != reflect.Int64 || !observedGenerationValue.CanSet() {
		return
	}
	generation := obj.GetGeneration()
	observedGenerationValue.SetInt(generation)
}

func (r *ParentReconciler) hasStatus(obj client.Object) bool {
	status := r.status(obj)
	return status != nil
}

func (r *ParentReconciler) status(obj client.Object) interface{} {
	if obj == nil {
		return nil
	}
	statusValue := reflect.ValueOf(obj).Elem().FieldByName("Status")
	if statusValue.Kind() == reflect.Ptr {
		statusValue = statusValue.Elem()
	}
	if !statusValue.IsValid() || !statusValue.CanAddr() {
		return nil
	}
	return statusValue.Addr().Interface()
}

// syncLastTransitionTime restores a condition's LastTransitionTime value for
// each proposed condition that is otherwise equivlent to the original value.
// This method is useful to prevent updating the status for a resource that is
// otherwise unchanged.
func (r *ParentReconciler) syncLastTransitionTime(proposed, original []metav1.Condition) {
	for _, o := range original {
		for i := range proposed {
			p := &proposed[i]
			if o.Type == p.Type {
				if o.Status == p.Status &&
					o.Reason == p.Reason &&
					o.Message == p.Message &&
					o.ObservedGeneration == p.ObservedGeneration {
					p.LastTransitionTime = o.LastTransitionTime
				}
				break
			}
		}
	}
}

const requestStashKey StashKey = "reconciler-runtime:request"
const configStashKey StashKey = "reconciler-runtime:config"
const parentConfigStashKey StashKey = "reconciler-runtime:parentConfig"
const parentTypeStashKey StashKey = "reconciler-runtime:parentType"
const castParentTypeStashKey StashKey = "reconciler-runtime:castParentType"

func StashRequest(ctx context.Context, req ctrl.Request) context.Context {
	return context.WithValue(ctx, requestStashKey, req)
}

func StashConfig(ctx context.Context, config Config) context.Context {
	return context.WithValue(ctx, configStashKey, config)
}

func StashParentConfig(ctx context.Context, parentConfig Config) context.Context {
	return context.WithValue(ctx, parentConfigStashKey, parentConfig)
}

func StashParentType(ctx context.Context, parentType client.Object) context.Context {
	return context.WithValue(ctx, parentTypeStashKey, parentType)
}

func StashCastParentType(ctx context.Context, currentType client.Object) context.Context {
	return context.WithValue(ctx, castParentTypeStashKey, currentType)
}

func RetrieveRequest(ctx context.Context) ctrl.Request {
	value := ctx.Value(requestStashKey)
	if req, ok := value.(ctrl.Request); ok {
		return req
	}
	return ctrl.Request{}
}

func RetrieveConfig(ctx context.Context) Config {
	value := ctx.Value(configStashKey)
	if config, ok := value.(Config); ok {
		return config
	}
	return Config{}
}

func RetrieveConfigOrDie(ctx context.Context) Config {
	config := RetrieveConfig(ctx)
	if config.IsEmpty() {
		panic(fmt.Errorf("config must exist on the context. Check that the context is from a ParentReconciler or WithConfig"))
	}
	return config
}

func RetrieveParentConfig(ctx context.Context) Config {
	value := ctx.Value(parentConfigStashKey)
	if parentConfig, ok := value.(Config); ok {
		return parentConfig
	}
	return Config{}
}

func RetrieveParentConfigOrDie(ctx context.Context) Config {
	config := RetrieveParentConfig(ctx)
	if config.IsEmpty() {
		panic(fmt.Errorf("parent config must exist on the context. Check that the context is from a ParentReconciler"))
	}
	return config
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
	_ SubReconciler = (*WithConfig)(nil)
	_ SubReconciler = (*WithFinalizer)(nil)
)

// SyncReconciler is a sub reconciler for custom reconciliation logic. No
// behavior is defined directly.
type SyncReconciler struct {
	// Name used to identify this reconciler.  Defaults to `SyncReconciler`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error

	// SyncDuringFinalization indicates the Sync method should be called when the resource is pending deletion.
	SyncDuringFinalization bool

	// Sync does whatever work is necessary for the reconciler.
	//
	// If SyncDuringFinalization is true this method is called when the resource is pending
	// deletion. This is useful if the reconciler is managing reference data.
	//
	// Expected function signature:
	//     func(ctx context.Context, parent client.Object) error
	//     func(ctx context.Context, parent client.Object) (ctrl.Result, error)
	Sync interface{}

	// Finalize does whatever work is necessary for the reconciler when the resource is pending
	// deletion. If this reconciler sets a finalizer it should do the necessary work to clean up
	// state the finalizer represents and then clear the finalizer.
	//
	// Expected function signature:
	//     func(ctx context.Context, parent client.Object) error
	//     func(ctx context.Context, parent client.Object) (ctrl.Result, error)
	//
	// +optional
	Finalize interface{}
}

func (r *SyncReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Name == "" {
		r.Name = "SyncReconciler"
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

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
		return fmt.Errorf("SyncReconciler %q must implement Sync", r.Name)
	} else {
		castParentType := RetrieveCastParentType(ctx)
		fn := reflect.TypeOf(r.Sync)
		err := fmt.Errorf("SyncReconciler %q must implement Sync: func(context.Context, %s) error | func(context.Context, %s) (ctrl.Result, error), found: %s", r.Name, reflect.TypeOf(castParentType), reflect.TypeOf(castParentType), fn)
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

	// validate Finalize function signature:
	//     nil
	//     func(ctx context.Context, parent client.Object) error
	//     func(ctx context.Context, parent client.Object) (ctrl.Result, error)
	if r.Finalize != nil {
		castParentType := RetrieveCastParentType(ctx)
		fn := reflect.TypeOf(r.Finalize)
		err := fmt.Errorf("SyncReconciler %q must implement Finalize: nil | func(context.Context, %s) error | func(context.Context, %s) (ctrl.Result, error), found: %s", r.Name, reflect.TypeOf(castParentType), reflect.TypeOf(castParentType), fn)
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
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	result := ctrl.Result{}

	if parent.GetDeletionTimestamp() == nil || r.SyncDuringFinalization {
		syncResult, err := r.sync(ctx, parent)
		if err != nil {
			log.Error(err, "unable to sync")
			return ctrl.Result{}, err
		}
		result = AggregateResults(result, syncResult)
	}

	if parent.GetDeletionTimestamp() != nil {
		finalizeResult, err := r.finalize(ctx, parent)
		if err != nil {
			log.Error(err, "unable to finalize")
			return ctrl.Result{}, err
		}
		result = AggregateResults(result, finalizeResult)
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

func (r *SyncReconciler) finalize(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	if r.Finalize == nil {
		return ctrl.Result{}, nil
	}

	fn := reflect.ValueOf(r.Finalize)
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
// During setup, the child resource type is registered to watch for changes.
type ChildReconciler struct {
	// Name used to identify this reconciler.  Defaults to `{ChildType}ChildReconciler`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// ChildType is the resource being created/updated/deleted by the
	// reconciler. For example, a parent Deployment would have a ReplicaSet as a
	// child.
	ChildType client.Object
	// ChildListType is the listing type for the child type. For example,
	// PodList is the list type for Pod
	ChildListType client.ObjectList

	// Finalizer is set on the parent resource before a child resource is created, and cleared
	// after a child resource is deleted. The value must be unique to this specific reconciler
	// instance and not shared. Reusing a value may result in orphaned resources when the parent
	// resource is deleted.
	//
	// Using a finalizer is encouraged when the Kubernetes garbage collector is unable to delete
	// the child resource automatically, like when the parent and child are in different
	// namespaces, scopes or clusters.
	//
	// +optional
	Finalizer string

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

	// OurChild is used when there are multiple ChildReconciler for the same ChildType
	// controlled by the same parent object. The function return true for child resources
	// managed by this ChildReconciler. Objects returned from the DesiredChild function
	// should match this function, otherwise they may be orphaned. If not specified, all
	// children match.
	//
	// Expected function signature:
	//     func(parent, child client.Object) bool
	//
	// +optional
	OurChild interface{}

	// Sanitize is called with an object before logging the value. Any value may
	// be returned. A meaningful subset of the resource is typically returned,
	// like the Spec.
	//
	// Expected function signature:
	//     func(child client.Object) interface{}
	//
	// +optional
	Sanitize interface{}

	// mutationCache holds patches received from updates to a resource made by
	// mutation webhooks. This cache is used to avoid unnecessary update calls
	// that would actually have no effect.
	mutationCache *cache.Expiring
	lazyInit      sync.Once
}

func (r *ChildReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	c := RetrieveConfigOrDie(ctx)

	if r.Name == "" {
		r.Name = fmt.Sprintf("%sChildReconciler", typeName(r.ChildType))
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(r.ChildType, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}

	bldr.Owns(r.ChildType)

	if r.Setup == nil {
		return nil
	}
	return r.Setup(ctx, mgr, bldr)
}

func (r *ChildReconciler) validate(ctx context.Context) error {
	castParentType := RetrieveCastParentType(ctx)

	// validate ChildType value
	if r.ChildType == nil {
		return fmt.Errorf("ChildReconciler %q must define ChildType", r.Name)
	}

	// validate ChildListType value
	if r.ChildListType == nil {
		return fmt.Errorf("ChildReconciler %q must define ChildListType", r.Name)
	}

	// validate DesiredChild function signature:
	//     func(ctx context.Context, parent client.Object) (client.Object, error)
	if r.DesiredChild == nil {
		return fmt.Errorf("ChildReconciler %q must implement DesiredChild", r.Name)
	} else {
		fn := reflect.TypeOf(r.DesiredChild)
		if fn.NumIn() != 2 || fn.NumOut() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castParentType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.Out(0)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.Out(1)) {
			return fmt.Errorf("ChildReconciler %q must implement DesiredChild: func(context.Context, %s) (%s, error), found: %s", r.Name, reflect.TypeOf(castParentType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate ReflectChildStatusOnParent function signature:
	//     func(parent, child client.Object, err error)
	if r.ReflectChildStatusOnParent == nil {
		return fmt.Errorf("ChildReconciler %q must implement ReflectChildStatusOnParent", r.Name)
	} else {
		fn := reflect.TypeOf(r.ReflectChildStatusOnParent)
		if fn.NumIn() != 3 || fn.NumOut() != 0 ||
			!reflect.TypeOf(castParentType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.In(2)) {
			return fmt.Errorf("ChildReconciler %q must implement ReflectChildStatusOnParent: func(%s, %s, error), found: %s", r.Name, reflect.TypeOf(castParentType), reflect.TypeOf(r.ChildType), fn)
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
			return fmt.Errorf("ChildReconciler %q must implement HarmonizeImmutableFields: nil | func(%s, %s), found: %s", r.Name, reflect.TypeOf(r.ChildType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate MergeBeforeUpdate function signature:
	//     func(current, desired client.Object)
	if r.MergeBeforeUpdate == nil {
		return fmt.Errorf("ChildReconciler %q must implement MergeBeforeUpdate", r.Name)
	} else {
		fn := reflect.TypeOf(r.MergeBeforeUpdate)
		if fn.NumIn() != 2 || fn.NumOut() != 0 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) {
			return fmt.Errorf("ChildReconciler %q must implement MergeBeforeUpdate: func(%s, %s), found: %s", r.Name, reflect.TypeOf(r.ChildType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate SemanticEquals function signature:
	//     func(a1, a2 client.Object) bool
	if r.SemanticEquals == nil {
		return fmt.Errorf("ChildReconciler %q must implement SemanticEquals", r.Name)
	} else {
		fn := reflect.TypeOf(r.SemanticEquals)
		if fn.NumIn() != 2 || fn.NumOut() != 1 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			fn.Out(0).Kind() != reflect.Bool {
			return fmt.Errorf("ChildReconciler %q must implement SemanticEquals: func(%s, %s) bool, found: %s", r.Name, reflect.TypeOf(r.ChildType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate OurChild function signature:
	//     nil
	//     func(parent, child client.Object) bool
	if r.OurChild != nil {
		fn := reflect.TypeOf(r.OurChild)
		if fn.NumIn() != 2 || fn.NumOut() != 1 ||
			!reflect.TypeOf(castParentType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			fn.Out(0).Kind() != reflect.Bool {
			return fmt.Errorf("ChildReconciler %q must implement OurChild: nil | func(%s, %s) bool, found: %s", r.Name, reflect.TypeOf(castParentType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate Sanitize function signature:
	//     nil
	//     func(child client.Object) interface{}
	if r.Sanitize != nil {
		fn := reflect.TypeOf(r.Sanitize)
		if fn.NumIn() != 1 || fn.NumOut() != 1 ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(0)) {
			return fmt.Errorf("ChildReconciler %q must implement Sanitize: nil | func(%s) interface{}, found: %s", r.Name, reflect.TypeOf(r.ChildType), fn)
		}
	}

	return nil
}

func (r *ChildReconciler) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(r.ChildType, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	r.lazyInit.Do(func() {
		r.mutationCache = cache.NewExpiring()
	})

	child, err := r.reconcile(ctx, parent)
	if parent.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, err
	}
	if err != nil {
		if apierrs.IsAlreadyExists(err) {
			// check if the resource blocking create is owned by the parent.
			// the created child from a previous turn may be slow to appear in the informer cache, but shouldn't appear
			// on the parent as being not ready.
			apierr := err.(apierrs.APIStatus)
			conflicted := newEmpty(r.ChildType).(client.Object)
			_ = c.APIReader.Get(ctx, types.NamespacedName{Namespace: parent.GetNamespace(), Name: apierr.Status().Details.Name}, conflicted)
			if r.ourChild(parent, conflicted) {
				// skip updating the parent's status, fail and try again
				return ctrl.Result{}, err
			}
			log.Info("unable to reconcile child, not owned", "child", namespaceName(conflicted), "ownerRefs", conflicted.GetOwnerReferences())
			r.reflectChildStatusOnParent(parent, child, err)
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to reconcile child")
		return ctrl.Result{}, err
	}
	r.reflectChildStatusOnParent(parent, child, err)

	return ctrl.Result{}, nil
}

func (r *ChildReconciler) reconcile(ctx context.Context, parent client.Object) (client.Object, error) {
	log := logr.FromContextOrDiscard(ctx)
	pc := RetrieveParentConfigOrDie(ctx)
	c := RetrieveConfigOrDie(ctx)

	actual := newEmpty(r.ChildType).(client.Object)
	children := newEmpty(r.ChildListType).(client.ObjectList)
	if err := c.List(ctx, children, client.InNamespace(parent.GetNamespace())); err != nil {
		return nil, err
	}
	items := r.filterChildren(parent, children)
	if len(items) == 1 {
		actual = items[0]
	} else if len(items) > 1 {
		// this shouldn't happen, delete everything to a clean slate
		for _, extra := range items {
			log.Info("deleting extra child", "child", namespaceName(extra))
			if err := c.Delete(ctx, extra); err != nil {
				pc.Recorder.Eventf(parent, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(r.ChildType), extra.GetName(), err)
				return nil, err
			}
			pc.Recorder.Eventf(parent, corev1.EventTypeNormal, "Deleted",
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
		if err := ctrl.SetControllerReference(parent, desired, c.Scheme()); err != nil {
			return nil, err
		}
		if !r.ourChild(parent, desired) {
			log.Info("object returned from DesiredChild does not match OurChild, this can result in orphaned children", "child", namespaceName(desired))
		}
	}

	// delete child if no longer needed
	if desired == nil {
		if !actual.GetCreationTimestamp().Time.IsZero() {
			log.Info("deleting unwanted child", "child", namespaceName(actual))
			if err := c.Delete(ctx, actual); err != nil {
				log.Error(err, "unable to delete unwanted child", "child", namespaceName(actual))
				pc.Recorder.Eventf(parent, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(r.ChildType), actual.GetName(), err)
				return nil, err
			}
			pc.Recorder.Eventf(parent, corev1.EventTypeNormal, "Deleted",
				"Deleted %s %q", typeName(r.ChildType), actual.GetName())

			if err := ClearParentFinalizer(ctx, parent, r.Finalizer); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	if err := AddParentFinalizer(ctx, parent, r.Finalizer); err != nil {
		return nil, err
	}

	// create child if it doesn't exist
	if actual.GetName() == "" {
		log.Info("creating child", "child", r.sanitize(desired))
		if err := c.Create(ctx, desired); err != nil {
			log.Error(err, "unable to create child", "child", namespaceName(desired))
			pc.Recorder.Eventf(parent, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create %s %q: %v", typeName(r.ChildType), desired.GetName(), err)
			return nil, err
		}
		pc.Recorder.Eventf(parent, corev1.EventTypeNormal, "Created",
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
			log.Info("unable to patch desired child from mutation cache")
		}
	}

	if r.semanticEquals(desiredPatched, actual) {
		// child is unchanged
		return actual, nil
	}

	// update child with desired changes
	current := actual.DeepCopyObject().(client.Object)
	r.mergeBeforeUpdate(current, desiredPatched)
	log.Info("updating child", "diff", cmp.Diff(r.sanitize(actual), r.sanitize(current)))
	if err := c.Update(ctx, current); err != nil {
		log.Error(err, "unable to update child", "child", namespaceName(current))
		pc.Recorder.Eventf(parent, corev1.EventTypeWarning, "UpdateFailed",
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
			log.Error(err, "unable to generate mutation patch", "snapshot", r.sanitize(desired), "base", r.sanitize(base))
		} else {
			r.mutationCache.Set(current.GetUID(), patch, 1*time.Hour)
		}
	}
	log.Info("updated child")
	pc.Recorder.Eventf(parent, corev1.EventTypeNormal, "Updated",
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
	if parent.GetDeletionTimestamp() != nil {
		// the parent is pending deletion, cleanup the child resource
		return nil, nil
	}

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

	// avoid accidental mutations in Sanitize method
	child = child.DeepCopyObject().(client.Object)

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

func (r *ChildReconciler) filterChildren(parent client.Object, children client.ObjectList) []client.Object {
	childrenValue := reflect.ValueOf(children).Elem()
	itemsValue := childrenValue.FieldByName("Items")
	items := []client.Object{}
	for i := 0; i < itemsValue.Len(); i++ {
		obj := itemsValue.Index(i).Addr().Interface().(client.Object)
		if r.ourChild(parent, obj) {
			items = append(items, obj)
		}
	}
	return items
}

func (r *ChildReconciler) ourChild(parent, obj client.Object) bool {
	if !metav1.IsControlledBy(obj, parent) {
		return false
	}
	// TODO do we need to remove resources pending deletion?
	if r.OurChild == nil {
		return true
	}
	fn := reflect.ValueOf(r.OurChild)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(parent),
		reflect.ValueOf(obj),
	})
	keep := true
	if out[0].Kind() == reflect.Bool {
		keep = out[0].Bool()
	}
	return keep
}

// Sequence is a collection of SubReconcilers called in order. If a
// reconciler errs, further reconcilers are skipped.
type Sequence []SubReconciler

func (r Sequence) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	for i, reconciler := range r {
		log := logr.FromContextOrDiscard(ctx).
			WithName(fmt.Sprintf("%d", i))
		ctx = logr.NewContext(ctx, log)

		err := reconciler.SetupWithManager(ctx, mgr, bldr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Sequence) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	aggregateResult := ctrl.Result{}
	for i, reconciler := range r {
		log := logr.FromContextOrDiscard(ctx).
			WithName(fmt.Sprintf("%d", i))
		ctx = logr.NewContext(ctx, log)

		result, err := reconciler.Reconcile(ctx, parent)
		if err != nil {
			return ctrl.Result{}, err
		}
		aggregateResult = AggregateResults(result, aggregateResult)
	}

	return aggregateResult, nil
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
	// Name used to identify this reconciler.  Defaults to `{Type}CastParent`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Type of resource to reconcile
	Type client.Object

	// Reconciler is called for each reconciler request with the parent
	// resource being reconciled. Typically a Sequence is used to compose
	// multiple SubReconcilers.
	Reconciler SubReconciler
}

func (r *CastParent) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Name == "" {
		r.Name = fmt.Sprintf("%sCastParent", typeName(r.Type))
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("castParentType", typeName(r.Type))
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}
	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *CastParent) validate(ctx context.Context) error {
	// validate Type value
	if r.Type == nil {
		return fmt.Errorf("CastParent %q must define Type", r.Name)
	}

	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("CastParent %q must define Reconciler", r.Name)
	}

	return nil
}

func (r *CastParent) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("castParentType", typeName(r.Type))
	ctx = logr.NewContext(ctx, log)

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
	castParent := newEmpty(r.Type).(client.Object)
	err = json.Unmarshal(data, castParent)
	if err != nil {
		return nil, nil, err
	}
	ctx = StashCastParentType(ctx, castParent)
	return ctx, castParent, nil
}

// WithConfig injects the provided config into the reconcilers nested under it. For example, the
// client can be swapped to use a service account with different permissions, or to target an
// entirely different cluster.
//
// The specified config can be accessed with `RetrieveConfig(ctx)`, the original config used to
// load the parent resource can be accessed with `RetrieveParentConfig(ctx)`.
type WithConfig struct {
	// Name used to identify this reconciler.  Defaults to `WithConfig`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Config to use for this portion of the reconciler hierarchy. This method is called during
	// setup and during reconciliation, if context is needed, it should be available durring both
	// phases.
	Config func(context.Context, Config) (Config, error)

	// Reconciler is called for each reconciler request with the parent
	// resource being reconciled. Typically a Sequence is used to compose
	// multiple SubReconcilers.
	Reconciler SubReconciler
}

func (r *WithConfig) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Name == "" {
		r.Name = "WithConfig"
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}
	c, err := r.Config(ctx, RetrieveConfig(ctx))
	if err != nil {
		return err
	}
	ctx = StashConfig(ctx, c)
	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *WithConfig) validate(ctx context.Context) error {
	// validate Config value
	if r.Config == nil {
		return fmt.Errorf("WithConfig %q must define Config", r.Name)
	}

	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("WithConfig %q must define Reconciler", r.Name)
	}

	return nil
}

func (r *WithConfig) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	c, err := r.Config(ctx, RetrieveConfig(ctx))
	if err != nil {
		return ctrl.Result{}, err
	}
	ctx = StashConfig(ctx, c)
	return r.Reconciler.Reconcile(ctx, parent)
}

// WithFinalizer ensures the resource being reconciled has the desired finalizer set so that state
// can be cleaned up upon the resource being deleted. The finalizer is added to the resource, if not
// already set, before calling the nested reconciler. When the resource is terminating, the
// finalizer is cleared after returning from the nested reconciler without error.
type WithFinalizer struct {
	// Name used to identify this reconciler.  Defaults to `WithFinalizer`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Finalizer to set on the parent resource. The value must be unique to this specific
	// reconciler instance and not shared. Reusing a value may result in orphaned state when
	// the parent resource is deleted.
	//
	// Using a finalizer is encouraged when state needs to be manually cleaned up before a resource
	// is fully deleted. This commonly include state allocated outside of the current cluster.
	Finalizer string

	// Reconciler is called for each reconciler request with the parent
	// resource being reconciled. Typically a Sequence is used to compose
	// multiple SubReconcilers.
	Reconciler SubReconciler
}

func (r *WithFinalizer) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Name == "" {
		r.Name = "WithFinalizer"
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}
	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *WithFinalizer) validate(ctx context.Context) error {
	// validate Finalizer value
	if r.Finalizer == "" {
		return fmt.Errorf("WithFinalizer %q must define Finalizer", r.Name)
	}

	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("WithFinalizer %q must define Reconciler", r.Name)
	}

	return nil
}

func (r *WithFinalizer) Reconcile(ctx context.Context, parent client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if parent.GetDeletionTimestamp() == nil {
		if err := AddParentFinalizer(ctx, parent, r.Finalizer); err != nil {
			return ctrl.Result{}, err
		}
	}
	result, err := r.Reconciler.Reconcile(ctx, parent)
	if err != nil {
		return result, err
	}
	if parent.GetDeletionTimestamp() != nil {
		if err := ClearParentFinalizer(ctx, parent, r.Finalizer); err != nil {
			return ctrl.Result{}, err
		}
	}
	return result, err
}

func typeName(i interface{}) string {
	t := reflect.TypeOf(i)
	// TODO do we need this?
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func gvk(obj client.Object, scheme *runtime.Scheme) schema.GroupVersionKind {
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}
	}
	return gvks[0]
}

func namespaceName(obj client.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// AddParentFinalizer ensures the desired finalizer exists on the parent resource. The client that
// loaded the parent resource is used to patch it with the finalizer if not already set.
func AddParentFinalizer(ctx context.Context, parent client.Object, finalizer string) error {
	return ensureParentFinalizer(ctx, parent, finalizer, true)
}

// ClearParentFinalizer ensures the desired finalizer does not exist on the parent resource. The
// client that loaded the parent resource is used to patch it with the finalizer if set.
func ClearParentFinalizer(ctx context.Context, parent client.Object, finalizer string) error {
	return ensureParentFinalizer(ctx, parent, finalizer, false)
}

func ensureParentFinalizer(ctx context.Context, parent client.Object, finalizer string, add bool) error {
	config := RetrieveParentConfig(ctx)
	if config.IsEmpty() {
		panic(fmt.Errorf("parent config must exist on the context. Check that the context from a ParentReconciler"))
	}
	parentType := RetrieveParentType(ctx)
	if parentType == nil {
		panic(fmt.Errorf("parent type must exist on the context. Check that the context from a ParentReconciler"))
	}

	if finalizer == "" || controllerutil.ContainsFinalizer(parent, finalizer) == add {
		// nothing to do
		return nil
	}

	// cast the current object back to the parent so scheme-aware, typed client can operate on it
	cast := &CastParent{
		Type: parentType,
		Reconciler: &SyncReconciler{
			SyncDuringFinalization: true,
			Sync: func(ctx context.Context, current client.Object) error {
				log := logr.FromContextOrDiscard(ctx)

				desired := current.DeepCopyObject().(client.Object)
				if add {
					log.Info("adding parent finalizer", "finalizer", finalizer)
					controllerutil.AddFinalizer(desired, finalizer)
				} else {
					log.Info("removing parent finalizer", "finalizer", finalizer)
					controllerutil.RemoveFinalizer(desired, finalizer)
				}

				patch := client.MergeFromWithOptions(current, client.MergeFromWithOptimisticLock{})
				if err := config.Patch(ctx, desired, patch); err != nil {
					log.Error(err, "unable to patch parent finalizers", "finalizer", finalizer)
					config.Recorder.Eventf(current, corev1.EventTypeWarning, "FinalizerPatchFailed",
						"Failed to patch finalizer %q: %s", finalizer, err)
					return err
				}
				config.Recorder.Eventf(current, corev1.EventTypeNormal, "FinalizerPatched",
					"Patched finalizer %q", finalizer)

				// update current object with values from the api server after patching
				current.SetFinalizers(desired.GetFinalizers())
				current.SetResourceVersion(desired.GetResourceVersion())
				current.SetGeneration(desired.GetGeneration())

				return nil
			},
		},
	}

	_, err := cast.Reconcile(ctx, parent)
	return err
}

// AggregateResults combines multiple results into a single result. If any result requests
// requeue, the aggregate is requeued. The shortest non-zero requeue after is the aggregate value.
func AggregateResults(results ...ctrl.Result) ctrl.Result {
	aggregate := ctrl.Result{}
	for _, result := range results {
		if result.RequeueAfter != 0 && (aggregate.RequeueAfter == 0 || result.RequeueAfter < aggregate.RequeueAfter) {
			aggregate.RequeueAfter = result.RequeueAfter
		}
		if result.Requeue {
			aggregate.Requeue = true
		}
	}
	return aggregate
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

// replaceWithEmpty overwrite the underlying value with it's empty value
func replaceWithEmpty(x interface{}) {
	v := reflect.ValueOf(x).Elem()
	v.Set(reflect.Zero(v.Type()))
}

// newEmpty returns a new empty value of the same underlying type, preserving the existing value
func newEmpty(x interface{}) interface{} {
	t := reflect.TypeOf(x).Elem()
	return reflect.New(t).Interface()
}

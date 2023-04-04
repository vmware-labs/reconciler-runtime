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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/vmware-labs/reconciler-runtime/internal"
	"github.com/vmware-labs/reconciler-runtime/tracker"
)

var (
	_ reconcile.Reconciler = (*ResourceReconciler[client.Object])(nil)
	_ reconcile.Reconciler = (*AggregateReconciler[client.Object])(nil)
)

// Config holds common resources for controllers. The configuration may be
// passed to sub-reconcilers.
type Config struct {
	client.Client
	APIReader client.Reader
	Recorder  record.EventRecorder
	Tracker   tracker.Tracker
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
		Tracker:   c.Tracker,
	}
}

// TrackAndGet tracks the resources for changes and returns the current value. The track is
// registered even when the resource does not exists so that its creation can be tracked.
//
// Equivalent to calling both `c.Tracker.Track(...)` and `c.Client.Get(...)`
func (c Config) TrackAndGet(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	c.Tracker.Track(
		ctx,
		tracker.NewKey(gvk(obj, c.Scheme()), key),
		RetrieveRequest(ctx).NamespacedName,
	)
	return c.Get(ctx, key, obj, opts...)
}

// NewConfig creates a Config for a specific API type. Typically passed into a
// reconciler.
func NewConfig(mgr ctrl.Manager, apiType client.Object, syncPeriod time.Duration) Config {
	return Config{
		Tracker: tracker.New(2 * syncPeriod),
	}.WithCluster(mgr)
}

// ResourceReconciler is a controller-runtime reconciler that reconciles a given
// existing resource. The Type resource is fetched for the reconciler
// request and passed in turn to each SubReconciler. Finally, the reconciled
// resource's status is compared with the original status, updating the API
// server if needed.
type ResourceReconciler[Type client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `{Type}ResourceReconciler`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error

	// Type of resource to reconcile. Required when the generic type is not a
	// struct, or is unstructured.
	//
	// +optional
	Type Type

	// Reconciler is called for each reconciler request with the resource being reconciled.
	// Typically, Reconciler is a Sequence of multiple SubReconcilers.
	//
	// When HaltSubReconcilers is returned as an error, execution continues as if no error was
	// returned.
	Reconciler SubReconciler[Type]

	Config Config

	lazyInit sync.Once
}

func (r *ResourceReconciler[T]) init() {
	r.lazyInit.Do(func() {
		if internal.IsNil(r.Type) {
			var nilT T
			r.Type = newEmpty(nilT).(T)
		}
		if r.Name == "" {
			r.Name = fmt.Sprintf("%sResourceReconciler", typeName(r.Type))
		}
	})
}

func (r *ResourceReconciler[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	_, err := r.SetupWithManagerYieldingController(ctx, mgr)
	return err
}

func (r *ResourceReconciler[T]) SetupWithManagerYieldingController(ctx context.Context, mgr ctrl.Manager) (controller.Controller, error) {
	r.init()

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("resourceType", gvk(r.Type, r.Config.Scheme()))
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
	return bldr.Build(r)
}

func (r *ResourceReconciler[T]) validate(ctx context.Context) error {
	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("ResourceReconciler %q must define Reconciler", r.Name)
	}

	// warn users of common pitfalls. These are not blockers.

	log := logr.FromContextOrDiscard(ctx)

	resourceType := reflect.TypeOf(r.Type).Elem()
	statusField, hasStatus := resourceType.FieldByName("Status")
	if !hasStatus {
		log.Info("resource missing status field, operations related to status will be skipped")
		return nil
	}

	statusType := statusField.Type
	if statusType.Kind() == reflect.Ptr {
		log.Info("resource status is nilable, status is typically a struct")
		statusType = statusType.Elem()
	}

	observedGenerationField, hasObservedGeneration := statusType.FieldByName("ObservedGeneration")
	if !hasObservedGeneration || observedGenerationField.Type.Kind() != reflect.Int64 {
		log.Info("resource status missing ObservedGeneration field of type int64, generation will not be managed")
	}

	initializeConditionsMethod, hasInitializeConditions := reflect.PtrTo(statusType).MethodByName("InitializeConditions")
	if !hasInitializeConditions || initializeConditionsMethod.Type.NumIn() != 1 || initializeConditionsMethod.Type.NumOut() != 0 {
		log.Info("resource status missing InitializeConditions() method, conditions will not be auto-initialized")
	}

	conditionsField, hasConditions := statusType.FieldByName("Conditions")
	if !hasConditions || !conditionsField.Type.AssignableTo(reflect.TypeOf([]metav1.Condition{})) {
		log.Info("resource status is missing field Conditions of type []metav1.Condition, condition timestamps will not be managed")
	}

	return nil
}

func (r *ResourceReconciler[T]) Reconcile(ctx context.Context, req Request) (Result, error) {
	r.init()

	ctx = WithStash(ctx)

	c := r.Config

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("resourceType", gvk(r.Type, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	ctx = StashRequest(ctx, req)
	ctx = StashConfig(ctx, c)
	ctx = StashOriginalConfig(ctx, c)
	ctx = StashOriginalResourceType(ctx, r.Type)
	ctx = StashResourceType(ctx, r.Type)
	originalResource := r.Type.DeepCopyObject().(T)

	if err := c.Get(ctx, req.NamespacedName, originalResource); err != nil {
		if apierrs.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return Result{}, nil
		}
		log.Error(err, "unable to fetch resource")
		return Result{}, err
	}
	resource := originalResource.DeepCopyObject().(T)

	if defaulter, ok := client.Object(resource).(webhook.Defaulter); ok {
		// resource.Default()
		defaulter.Default()
	}

	r.initializeConditions(resource)
	result, err := r.reconcile(ctx, resource)

	// attempt to restore last transition time for unchanged conditions
	r.syncLastTransitionTime(r.conditions(resource), r.conditions(originalResource))

	// check if status has changed before updating
	resourceStatus, originalResourceStatus := r.status(resource), r.status(originalResource)
	if !equality.Semantic.DeepEqual(resourceStatus, originalResourceStatus) && resource.GetDeletionTimestamp() == nil {
		// update status
		log.Info("updating status", "diff", cmp.Diff(originalResourceStatus, resourceStatus))
		if updateErr := c.Status().Update(ctx, resource); updateErr != nil {
			log.Error(updateErr, "unable to update status")
			c.Recorder.Eventf(resource, corev1.EventTypeWarning, "StatusUpdateFailed",
				"Failed to update status: %v", updateErr)
			return Result{}, updateErr
		}
		c.Recorder.Eventf(resource, corev1.EventTypeNormal, "StatusUpdated",
			"Updated status")
	}

	// return original reconcile result
	return result, err
}

func (r *ResourceReconciler[T]) reconcile(ctx context.Context, resource T) (Result, error) {
	if resource.GetDeletionTimestamp() != nil && len(resource.GetFinalizers()) == 0 {
		// resource is being deleted and has no pending finalizers, nothing to do
		return Result{}, nil
	}

	result, err := r.Reconciler.Reconcile(ctx, resource)
	if err != nil && !errors.Is(err, HaltSubReconcilers) {
		return Result{}, err
	}

	r.copyGeneration(resource)
	return result, nil
}

func (r *ResourceReconciler[T]) initializeConditions(obj T) {
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

func (r *ResourceReconciler[T]) conditions(obj T) []metav1.Condition {
	// return obj.Status.Conditions
	status := r.status(obj)
	if status == nil {
		return nil
	}
	statusValue := reflect.ValueOf(status)
	if statusValue.Type().Kind() == reflect.Map {
		return nil
	}
	statusValue = statusValue.Elem()
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

func (r *ResourceReconciler[T]) copyGeneration(obj T) {
	// obj.Status.ObservedGeneration = obj.Generation
	status := r.status(obj)
	if status == nil {
		return
	}
	statusValue := reflect.ValueOf(status)
	if statusValue.Type().Kind() == reflect.Map {
		return
	}
	statusValue = statusValue.Elem()
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

func (r *ResourceReconciler[T]) hasStatus(obj T) bool {
	status := r.status(obj)
	return status != nil
}

func (r *ResourceReconciler[T]) status(obj T) interface{} {
	if client.Object(obj) == nil {
		return nil
	}
	if u, ok := client.Object(obj).(*unstructured.Unstructured); ok {
		return u.UnstructuredContent()["status"]
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
// each proposed condition that is otherwise equivalent to the original value.
// This method is useful to prevent updating the status for a resource that is
// otherwise unchanged.
func (r *ResourceReconciler[T]) syncLastTransitionTime(proposed, original []metav1.Condition) {
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
		Tracker:   tracker.New(0),
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

const requestStashKey StashKey = "reconciler-runtime:request"
const configStashKey StashKey = "reconciler-runtime:config"
const originalConfigStashKey StashKey = "reconciler-runtime:originalConfig"
const resourceTypeStashKey StashKey = "reconciler-runtime:resourceType"
const originalResourceTypeStashKey StashKey = "reconciler-runtime:originalResourceType"
const additionalConfigsStashKey StashKey = "reconciler-runtime:additionalConfigs"

func StashRequest(ctx context.Context, req Request) context.Context {
	return context.WithValue(ctx, requestStashKey, req)
}

// RetrieveRequest returns the reconciler Request from the context, or empty if not found.
func RetrieveRequest(ctx context.Context) Request {
	value := ctx.Value(requestStashKey)
	if req, ok := value.(Request); ok {
		return req
	}
	return Request{}
}

func StashConfig(ctx context.Context, config Config) context.Context {
	return context.WithValue(ctx, configStashKey, config)
}

// RetrieveConfig returns the Config from the context. An error is returned if not found.
func RetrieveConfig(ctx context.Context) (Config, error) {
	value := ctx.Value(configStashKey)
	if config, ok := value.(Config); ok {
		return config, nil
	}
	return Config{}, fmt.Errorf("config must exist on the context. Check that the context is from a ResourceReconciler or WithConfig")
}

// RetrieveConfigOrDie returns the Config from the context. Panics if not found.
func RetrieveConfigOrDie(ctx context.Context) Config {
	config, err := RetrieveConfig(ctx)
	if err != nil {
		panic(err)
	}
	return config
}

func StashOriginalConfig(ctx context.Context, resourceConfig Config) context.Context {
	return context.WithValue(ctx, originalConfigStashKey, resourceConfig)
}

// RetrieveOriginalConfig returns the Config from the context used to load the reconciled resource. An
// error is returned if not found.
func RetrieveOriginalConfig(ctx context.Context) (Config, error) {
	value := ctx.Value(originalConfigStashKey)
	if config, ok := value.(Config); ok {
		return config, nil
	}
	return Config{}, fmt.Errorf("resource config must exist on the context. Check that the context is from a ResourceReconciler")
}

// RetrieveOriginalConfigOrDie returns the Config from the context used to load the reconciled resource.
// Panics if not found.
func RetrieveOriginalConfigOrDie(ctx context.Context) Config {
	config, err := RetrieveOriginalConfig(ctx)
	if err != nil {
		panic(err)
	}
	return config
}

func StashResourceType(ctx context.Context, currentType client.Object) context.Context {
	return context.WithValue(ctx, resourceTypeStashKey, currentType)
}

// RetrieveResourceType returns the reconciled resource type object, or nil if not found.
func RetrieveResourceType(ctx context.Context) client.Object {
	value := ctx.Value(resourceTypeStashKey)
	if currentType, ok := value.(client.Object); ok {
		return currentType
	}
	return nil
}

func StashOriginalResourceType(ctx context.Context, resourceType client.Object) context.Context {
	return context.WithValue(ctx, originalResourceTypeStashKey, resourceType)
}

// RetrieveOriginalResourceType returns the reconciled resource type object, or nil if not found.
func RetrieveOriginalResourceType(ctx context.Context) client.Object {
	value := ctx.Value(originalResourceTypeStashKey)
	if resourceType, ok := value.(client.Object); ok {
		return resourceType
	}
	return nil
}

func StashAdditionalConfigs(ctx context.Context, additionalConfigs map[string]Config) context.Context {
	return context.WithValue(ctx, additionalConfigsStashKey, additionalConfigs)
}

// RetrieveAdditionalConfigs returns the additional configs defined for this request. Uncommon
// outside of the context of a test. An empty map is returned when no value is stashed.
func RetrieveAdditionalConfigs(ctx context.Context) map[string]Config {
	value := ctx.Value(additionalConfigsStashKey)
	if additionalConfigs, ok := value.(map[string]Config); ok {
		return additionalConfigs
	}
	return map[string]Config{}
}

// SubReconciler are participants in a larger reconciler request. The resource
// being reconciled is passed directly to the sub reconciler.
type SubReconciler[Type client.Object] interface {
	SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error
	Reconcile(ctx context.Context, resource Type) (Result, error)
}

var (
	_ SubReconciler[client.Object] = (*SyncReconciler[client.Object])(nil)
	_ SubReconciler[client.Object] = (*ChildReconciler[client.Object, client.Object, client.ObjectList])(nil)
	_ SubReconciler[client.Object] = (Sequence[client.Object])(nil)
	_ SubReconciler[client.Object] = (*CastResource[client.Object, client.Object])(nil)
	_ SubReconciler[client.Object] = (*WithConfig[client.Object])(nil)
	_ SubReconciler[client.Object] = (*WithFinalizer[client.Object])(nil)
)

var (
	// HaltSubReconcilers is an error that instructs SubReconcilers to stop processing the request,
	// while the root reconciler proceeds as if there was no error. HaltSubReconcilers may be
	// wrapped by other errors.
	//
	// See documentation for the specific SubReconciler caller to see how they handle this case.
	HaltSubReconcilers = errors.New("stop processing SubReconcilers, without returning an error")
)

// SyncReconciler is a sub reconciler for custom reconciliation logic. No
// behavior is defined directly.
type SyncReconciler[Type client.Object] struct {
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
	// Mutually exclusive with SyncWithResult
	Sync func(ctx context.Context, resource Type) error

	// SyncWithResult does whatever work is necessary for the reconciler.
	//
	// If SyncDuringFinalization is true this method is called when the resource is pending
	// deletion. This is useful if the reconciler is managing reference data.
	//
	// Mutually exclusive with Sync
	SyncWithResult func(ctx context.Context, resource Type) (Result, error)

	// Finalize does whatever work is necessary for the reconciler when the resource is pending
	// deletion. If this reconciler sets a finalizer it should do the necessary work to clean up
	// state the finalizer represents and then clear the finalizer.
	//
	// Mutually exclusive with FinalizeWithResult
	//
	// +optional
	Finalize func(ctx context.Context, resource Type) error

	// Finalize does whatever work is necessary for the reconciler when the resource is pending
	// deletion. If this reconciler sets a finalizer it should do the necessary work to clean up
	// state the finalizer represents and then clear the finalizer.
	//
	// Mutually exclusive with Finalize
	//
	// +optional
	FinalizeWithResult func(ctx context.Context, resource Type) (Result, error)
}

func (r *SyncReconciler[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
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

func (r *SyncReconciler[T]) validate(ctx context.Context) error {
	// validate Sync and SyncWithResult
	if r.Sync == nil && r.SyncWithResult == nil {
		return fmt.Errorf("SyncReconciler %q must implement Sync or SyncWithResult", r.Name)
	}
	if r.Sync != nil && r.SyncWithResult != nil {
		return fmt.Errorf("SyncReconciler %q may not implement both Sync and SyncWithResult", r.Name)
	}

	// validate Finalize and FinalizeWithResult
	if r.Finalize != nil && r.FinalizeWithResult != nil {
		return fmt.Errorf("SyncReconciler %q may not implement both Finalize and FinalizeWithResult", r.Name)
	}

	return nil
}

func (r *SyncReconciler[T]) Reconcile(ctx context.Context, resource T) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	result := Result{}

	if resource.GetDeletionTimestamp() == nil || r.SyncDuringFinalization {
		syncResult, err := r.sync(ctx, resource)
		if err != nil {
			log.Error(err, "unable to sync")
			return Result{}, err
		}
		result = AggregateResults(result, syncResult)
	}

	if resource.GetDeletionTimestamp() != nil {
		finalizeResult, err := r.finalize(ctx, resource)
		if err != nil {
			log.Error(err, "unable to finalize")
			return Result{}, err
		}
		result = AggregateResults(result, finalizeResult)
	}

	return result, nil
}

func (r *SyncReconciler[T]) sync(ctx context.Context, resource T) (Result, error) {
	if r.Sync != nil {
		err := r.Sync(ctx, resource)
		return Result{}, err
	}
	return r.SyncWithResult(ctx, resource)
}

func (r *SyncReconciler[T]) finalize(ctx context.Context, resource T) (Result, error) {
	if r.Finalize != nil {
		err := r.Finalize(ctx, resource)
		return Result{}, err
	}
	if r.FinalizeWithResult != nil {
		return r.FinalizeWithResult(ctx, resource)
	}

	return Result{}, nil
}

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
	// child. Select types of error are passed, including:
	//   - apierrs.IsConflict
	ReflectChildStatusOnParent func(parent Type, child ChildType, err error)

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
		r.stamp = &ResourceManager[CT]{
			Name:                     r.Name,
			Type:                     r.ChildType,
			Finalizer:                r.Finalizer,
			TrackDesired:             r.SkipOwnerReference,
			HarmonizeImmutableFields: r.HarmonizeImmutableFields,
			MergeBeforeUpdate:        r.MergeBeforeUpdate,
			Sanitize:                 r.Sanitize,
		}
	})
}

func (r *ChildReconciler[T, CT, CLT]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	r.init()

	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(r.ChildType, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}

	if r.SkipOwnerReference {
		bldr.Watches(&source.Kind{Type: r.ChildType}, EnqueueTracked(ctx, r.ChildType))
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

	return nil
}

func (r *ChildReconciler[T, CT, CLT]) Reconcile(ctx context.Context, resource T) (Result, error) {
	r.init()

	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(r.ChildType, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	child, err := r.reconcile(ctx, resource)
	if resource.GetDeletionTimestamp() != nil {
		return Result{}, err
	}
	if err != nil {
		if apierrs.IsAlreadyExists(err) {
			// check if the resource blocking create is owned by the reconciled resource.
			// the created child from a previous turn may be slow to appear in the informer cache, but shouldn't appear
			// on the reconciled resource as being not ready.
			apierr := err.(apierrs.APIStatus)
			conflicted := newEmpty(r.ChildType).(CT)
			_ = c.APIReader.Get(ctx, types.NamespacedName{Namespace: resource.GetNamespace(), Name: apierr.Status().Details.Name}, conflicted)
			if r.ourChild(resource, conflicted) {
				// skip updating the reconciled resource's status, fail and try again
				return Result{}, err
			}
			log.Info("unable to reconcile child, not owned", "child", namespaceName(conflicted), "ownerRefs", conflicted.GetOwnerReferences())
			r.ReflectChildStatusOnParent(resource, child, err)
			return Result{}, nil
		}
		log.Error(err, "unable to reconcile child")
		return Result{}, err
	}
	r.ReflectChildStatusOnParent(resource, child, err)

	return Result{}, nil
}

func (r *ChildReconciler[T, CT, CLT]) reconcile(ctx context.Context, resource T) (CT, error) {
	var nilCT CT
	log := logr.FromContextOrDiscard(ctx)
	pc := RetrieveOriginalConfigOrDie(ctx)
	c := RetrieveConfigOrDie(ctx)

	actual := newEmpty(r.ChildType).(CT)
	children := newEmpty(r.ChildListType).(CLT)
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
				pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(r.ChildType), extra.GetName(), err)
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
		if !r.SkipOwnerReference {
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
	childrenValue := reflect.ValueOf(children).Elem()
	itemsValue := childrenValue.FieldByName("Items")
	items := []CT{}
	for i := 0; i < itemsValue.Len(); i++ {
		obj := itemsValue.Index(i).Addr().Interface().(CT)
		if r.ourChild(resource, obj) {
			items = append(items, obj)
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

// Sequence is a collection of SubReconcilers called in order. If a
// reconciler errs, further reconcilers are skipped.
type Sequence[Type client.Object] []SubReconciler[Type]

func (r Sequence[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
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

func (r Sequence[T]) Reconcile(ctx context.Context, resource T) (Result, error) {
	aggregateResult := Result{}
	for i, reconciler := range r {
		log := logr.FromContextOrDiscard(ctx).
			WithName(fmt.Sprintf("%d", i))
		ctx = logr.NewContext(ctx, log)

		result, err := reconciler.Reconcile(ctx, resource)
		if err != nil {
			return Result{}, err
		}
		aggregateResult = AggregateResults(result, aggregateResult)
	}

	return aggregateResult, nil
}

// CastResource casts the ResourceReconciler's type by projecting the resource data
// onto a new struct. Casting the reconciled resource is useful to create cross
// cutting reconcilers that can operate on common portion of multiple  resources,
// commonly referred to as a duck type.
//
// If the CastType generic is an interface rather than a struct, the resource is
// passed directly rather than converted.
type CastResource[Type, CastType client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `{Type}CastResource`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Reconciler is called for each reconciler request with the reconciled resource. Typically a
	// Sequence is used to compose multiple SubReconcilers.
	Reconciler SubReconciler[CastType]

	noop     bool
	lazyInit sync.Once
}

func (r *CastResource[T, CT]) init() {
	r.lazyInit.Do(func() {
		var nilCT CT
		if reflect.ValueOf(nilCT).Kind() == reflect.Invalid {
			// not a real cast, just converting generic types
			r.noop = true
			return
		}
		emptyCT := newEmpty(nilCT)
		if r.Name == "" {
			r.Name = fmt.Sprintf("%sCastResource", typeName(emptyCT))
		}
	})
}

func (r *CastResource[T, CT]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	r.init()

	if !r.noop {
		var nilCT CT
		emptyCT := newEmpty(nilCT).(CT)

		log := logr.FromContextOrDiscard(ctx).
			WithName(r.Name).
			WithValues("castResourceType", typeName(emptyCT))
		ctx = logr.NewContext(ctx, log)

		if err := r.validate(ctx); err != nil {
			return err
		}
	}

	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *CastResource[T, CT]) validate(ctx context.Context) error {
	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("CastResource %q must define Reconciler", r.Name)
	}

	return nil
}

func (r *CastResource[T, CT]) Reconcile(ctx context.Context, resource T) (Result, error) {
	r.init()

	if r.noop {
		// cast the type rather than convert the object
		return r.Reconciler.Reconcile(ctx, client.Object(resource).(CT))
	}

	var nilCT CT
	emptyCT := newEmpty(nilCT).(CT)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("castResourceType", typeName(emptyCT))
	ctx = logr.NewContext(ctx, log)

	ctx, castResource, err := r.cast(ctx, resource)
	if err != nil {
		return Result{}, err
	}
	castOriginal := castResource.DeepCopyObject().(client.Object)
	result, err := r.Reconciler.Reconcile(ctx, castResource)
	if err != nil {
		return Result{}, err
	}
	if !equality.Semantic.DeepEqual(castResource, castOriginal) {
		// patch the reconciled resource with the updated duck values
		patch, err := NewPatch(castOriginal, castResource)
		if err != nil {
			return Result{}, err
		}
		err = patch.Apply(resource)
		if err != nil {
			return Result{}, err
		}

	}
	return result, nil
}

func (r *CastResource[T, CT]) cast(ctx context.Context, resource T) (context.Context, CT, error) {
	var nilCT CT

	data, err := json.Marshal(resource)
	if err != nil {
		return nil, nilCT, err
	}
	castResource := newEmpty(nilCT).(CT)
	err = json.Unmarshal(data, castResource)
	if err != nil {
		return nil, nilCT, err
	}
	if kind := castResource.GetObjectKind(); kind.GroupVersionKind().Empty() {
		// default the apiVersion/kind with the real value from the resource if not already defined
		c := RetrieveConfigOrDie(ctx)
		kind.SetGroupVersionKind(gvk(resource, c.Scheme()))
	}
	ctx = StashResourceType(ctx, castResource)
	return ctx, castResource, nil
}

// WithConfig injects the provided config into the reconcilers nested under it. For example, the
// client can be swapped to use a service account with different permissions, or to target an
// entirely different cluster.
//
// The specified config can be accessed with `RetrieveConfig(ctx)`, the original config used to
// load the reconciled resource can be accessed with `RetrieveOriginalConfig(ctx)`.
type WithConfig[Type client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `WithConfig`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Config to use for this portion of the reconciler hierarchy. This method is called during
	// setup and during reconciliation, if context is needed, it should be available durring both
	// phases.
	Config func(context.Context, Config) (Config, error)

	// Reconciler is called for each reconciler request with the reconciled
	// resource being reconciled. Typically a Sequence is used to compose
	// multiple SubReconcilers.
	Reconciler SubReconciler[Type]
}

func (r *WithConfig[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Name == "" {
		r.Name = "WithConfig"
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}
	c, err := r.Config(ctx, RetrieveConfigOrDie(ctx))
	if err != nil {
		return err
	}
	ctx = StashConfig(ctx, c)
	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *WithConfig[T]) validate(ctx context.Context) error {
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

func (r *WithConfig[T]) Reconcile(ctx context.Context, resource T) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	c, err := r.Config(ctx, RetrieveConfigOrDie(ctx))
	if err != nil {
		return Result{}, err
	}
	ctx = StashConfig(ctx, c)
	return r.Reconciler.Reconcile(ctx, resource)
}

// WithFinalizer ensures the resource being reconciled has the desired finalizer set so that state
// can be cleaned up upon the resource being deleted. The finalizer is added to the resource, if not
// already set, before calling the nested reconciler. When the resource is terminating, the
// finalizer is cleared after returning from the nested reconciler without error.
type WithFinalizer[Type client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `WithFinalizer`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Finalizer to set on the reconciled resource. The value must be unique to this specific
	// reconciler instance and not shared. Reusing a value may result in orphaned state when
	// the reconciled resource is deleted.
	//
	// Using a finalizer is encouraged when state needs to be manually cleaned up before a resource
	// is fully deleted. This commonly include state allocated outside of the current cluster.
	Finalizer string

	// Reconciler is called for each reconciler request with the reconciled
	// resource being reconciled. Typically a Sequence is used to compose
	// multiple SubReconcilers.
	Reconciler SubReconciler[Type]
}

func (r *WithFinalizer[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
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

func (r *WithFinalizer[T]) validate(ctx context.Context) error {
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

func (r *WithFinalizer[T]) Reconcile(ctx context.Context, resource T) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if resource.GetDeletionTimestamp() == nil {
		if err := AddFinalizer(ctx, resource, r.Finalizer); err != nil {
			return Result{}, err
		}
	}
	result, err := r.Reconciler.Reconcile(ctx, resource)
	if err != nil {
		return result, err
	}
	if resource.GetDeletionTimestamp() != nil {
		if err := ClearFinalizer(ctx, resource, r.Finalizer); err != nil {
			return Result{}, err
		}
	}
	return result, err
}

// ResourceManager compares the actual and desired resources to create/update/delete as desired.
type ResourceManager[Type client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `{Type}ResourceManager`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// Type is the resource being created/updated/deleted by the reconciler. Required when the
	// generic type is not a struct, or is unstructured.
	//
	// +optional
	Type Type

	// Finalizer is set on the reconciled resource before a managed resource is created, and cleared
	// after a managed resource is deleted. The value must be unique to this specific manager
	// instance and not shared. Reusing a value may result in orphaned resources when the
	// reconciled resource is deleted.
	//
	// Using a finalizer is encouraged when the Kubernetes garbage collector is unable to delete
	// the child resource automatically, like when the reconciled resource and child are in different
	// namespaces, scopes or clusters.
	//
	// +optional
	Finalizer string

	// TrackDesired when true, the desired resource is tracked after creates, before
	// updates, and on delete errors.
	TrackDesired bool

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
	Sanitize func(child Type) interface{}

	// mutationCache holds patches received from updates to a resource made by
	// mutation webhooks. This cache is used to avoid unnecessary update calls
	// that would actually have no effect.
	mutationCache *cache.Expiring
	lazyInit      sync.Once
}

func (r *ResourceManager[T]) init() {
	r.lazyInit.Do(func() {
		if internal.IsNil(r.Type) {
			var nilT T
			r.Type = newEmpty(nilT).(T)
		}
		if r.Name == "" {
			r.Name = fmt.Sprintf("%sResourceManager", typeName(r.Type))
		}
		r.mutationCache = cache.NewExpiring()
	})
}

func (r *ResourceManager[T]) Setup(ctx context.Context) error {
	r.init()
	return r.validate(ctx)
}

func (r *ResourceManager[T]) validate(ctx context.Context) error {
	// require MergeBeforeUpdate
	if r.MergeBeforeUpdate == nil {
		return fmt.Errorf("ResourceManager %q must define MergeBeforeUpdate", r.Name)
	}

	return nil
}

// Manage a specific resource to create/update/delete based on the actual and desired state. The
// resource is the reconciled resource and used to record events for mutations. The actual and
// desired objects represent the managed resource and must be compatible with the type field.
func (r *ResourceManager[T]) Manage(ctx context.Context, resource client.Object, actual, desired T) (T, error) {
	r.init()

	var nilT T

	log := logr.FromContextOrDiscard(ctx)
	pc := RetrieveOriginalConfigOrDie(ctx)
	c := RetrieveConfigOrDie(ctx)

	if (internal.IsNil(actual) || actual.GetCreationTimestamp().Time.IsZero()) && internal.IsNil(desired) {
		if err := ClearFinalizer(ctx, resource, r.Finalizer); err != nil {
			return nilT, err
		}
		return nilT, nil
	}

	// delete resource if no longer needed
	if internal.IsNil(desired) {
		if !actual.GetCreationTimestamp().Time.IsZero() && actual.GetDeletionTimestamp() == nil {
			log.Info("deleting unwanted resource", "resource", namespaceName(actual))
			if err := c.Delete(ctx, actual); err != nil {
				log.Error(err, "unable to delete unwanted resource", "resource", namespaceName(actual))
				pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(actual), actual.GetName(), err)
				return nilT, err
			}
			pc.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted",
				"Deleted %s %q", typeName(actual), actual.GetName())

		}
		return nilT, nil
	}

	if err := AddFinalizer(ctx, resource, r.Finalizer); err != nil {
		return nilT, err
	}

	// create resource if it doesn't exist
	if actual.GetCreationTimestamp().Time.IsZero() {
		log.Info("creating resource", "resource", r.sanitize(desired))
		if err := c.Create(ctx, desired); err != nil {
			log.Error(err, "unable to create resource", "resource", namespaceName(desired))
			pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create %s %q: %v", typeName(desired), desired.GetName(), err)
			return nilT, err
		}
		if r.TrackDesired {
			// normally tracks should occur before API operations, but when creating a resource with a
			// generated name, we need to know the actual resource name.
			if err := c.Tracker.TrackChild(ctx, resource, desired, c.Scheme()); err != nil {
				return nilT, err
			}
		}
		pc.Recorder.Eventf(resource, corev1.EventTypeNormal, "Created",
			"Created %s %q", typeName(desired), desired.GetName())
		return desired, nil
	}

	// overwrite fields that should not be mutated
	if r.HarmonizeImmutableFields != nil {
		r.HarmonizeImmutableFields(actual, desired)
	}

	// lookup and apply remote mutations
	desiredPatched := desired.DeepCopyObject().(T)
	if patch, ok := r.mutationCache.Get(actual.GetUID()); ok {
		// the only object added to the cache is *Patch
		err := patch.(*Patch).Apply(desiredPatched)
		if err != nil {
			// there's not much we can do, but let the normal update proceed
			log.Info("unable to patch desired child from mutation cache")
		}
	}

	// update resource with desired changes
	current := actual.DeepCopyObject().(T)
	r.MergeBeforeUpdate(current, desiredPatched)
	if equality.Semantic.DeepEqual(current, actual) {
		// resource is unchanged
		log.Info("resource is in sync, no update required")
		return actual, nil
	}
	log.Info("updating resource", "diff", cmp.Diff(r.sanitize(actual), r.sanitize(current)))
	if r.TrackDesired {
		if err := c.Tracker.TrackChild(ctx, resource, current, c.Scheme()); err != nil {
			return nilT, err
		}
	}
	if err := c.Update(ctx, current); err != nil {
		log.Error(err, "unable to update resource", "resource", namespaceName(current))
		pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update %s %q: %v", typeName(current), current.GetName(), err)
		return nilT, err
	}

	// capture admission mutation patch
	base := current.DeepCopyObject().(T)
	r.MergeBeforeUpdate(base, desired)
	patch, err := NewPatch(base, current)
	if err != nil {
		log.Error(err, "unable to generate mutation patch", "snapshot", r.sanitize(desired), "base", r.sanitize(base))
	} else {
		r.mutationCache.Set(current.GetUID(), patch, 1*time.Hour)
	}

	log.Info("updated resource")
	pc.Recorder.Eventf(resource, corev1.EventTypeNormal, "Updated",
		"Updated %s %q", typeName(current), current.GetName())

	return current, nil
}

func (r *ResourceManager[T]) sanitize(resource T) interface{} {
	if r.Sanitize == nil {
		return resource
	}
	if internal.IsNil(resource) {
		return nil
	}

	// avoid accidental mutations in Sanitize method
	resource = resource.DeepCopyObject().(T)
	return r.Sanitize(resource)
}

func typeName(i interface{}) string {
	if obj, ok := i.(client.Object); ok {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		if kind != "" {
			return kind
		}
	}

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

// AddFinalizer ensures the desired finalizer exists on the reconciled resource. The client that
// loaded the reconciled resource is used to patch it with the finalizer if not already set.
func AddFinalizer(ctx context.Context, resource client.Object, finalizer string) error {
	return ensureFinalizer(ctx, resource, finalizer, true)
}

// ClearFinalizer ensures the desired finalizer does not exist on the reconciled resource. The
// client that loaded the reconciled resource is used to patch it with the finalizer if set.
func ClearFinalizer(ctx context.Context, resource client.Object, finalizer string) error {
	return ensureFinalizer(ctx, resource, finalizer, false)
}

func ensureFinalizer(ctx context.Context, resource client.Object, finalizer string, add bool) error {
	config := RetrieveOriginalConfigOrDie(ctx)
	if config.IsEmpty() {
		panic(fmt.Errorf("resource config must exist on the context. Check that the context from a ResourceReconciler"))
	}
	resourceType := RetrieveOriginalResourceType(ctx)
	if resourceType == nil {
		panic(fmt.Errorf("resource type must exist on the context. Check that the context from a ResourceReconciler"))
	}

	if finalizer == "" || controllerutil.ContainsFinalizer(resource, finalizer) == add {
		// nothing to do
		return nil
	}

	// cast the current object back to the resource so scheme-aware, typed client can operate on it
	cast := &CastResource[client.Object, client.Object]{
		Reconciler: &SyncReconciler[client.Object]{
			SyncDuringFinalization: true,
			Sync: func(ctx context.Context, current client.Object) error {
				log := logr.FromContextOrDiscard(ctx)

				desired := current.DeepCopyObject().(client.Object)
				if add {
					log.Info("adding finalizer", "finalizer", finalizer)
					controllerutil.AddFinalizer(desired, finalizer)
				} else {
					log.Info("removing finalizer", "finalizer", finalizer)
					controllerutil.RemoveFinalizer(desired, finalizer)
				}

				patch := client.MergeFromWithOptions(current, client.MergeFromWithOptimisticLock{})
				if err := config.Patch(ctx, desired, patch); err != nil {
					log.Error(err, "unable to patch finalizers", "finalizer", finalizer)
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

	_, err := cast.Reconcile(ctx, resource)
	return err
}

// AggregateResults combines multiple results into a single result. If any result requests
// requeue, the aggregate is requeued. The shortest non-zero requeue after is the aggregate value.
func AggregateResults(results ...Result) Result {
	aggregate := Result{}
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

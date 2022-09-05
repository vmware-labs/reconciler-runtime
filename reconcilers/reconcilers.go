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

	"github.com/vmware-labs/reconciler-runtime/tracker"
)

var (
	_ reconcile.Reconciler = (*ResourceReconciler)(nil)
	_ reconcile.Reconciler = (*AggregateReconciler)(nil)
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
	name := typeName(apiType)
	log := newWarnOnceLogger(ctrl.Log.WithName("controllers").WithName(name))
	return Config{
		Log:     log,
		Tracker: tracker.New(2 * syncPeriod),
	}.WithCluster(mgr)
}

// Deprecated use ResourceReconciler
type ParentReconciler = ResourceReconciler

// ResourceReconciler is a controller-runtime reconciler that reconciles a given
// existing resource. The Type resource is fetched for the reconciler
// request and passed in turn to each SubReconciler. Finally, the reconciled
// resource's status is compared with the original status, updating the API
// server if needed.
type ResourceReconciler struct {
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

	// Type of resource to reconcile
	Type client.Object

	// Reconciler is called for each reconciler request with the resource being reconciled.
	// Typically, Reconciler is a Sequence of multiple SubReconcilers.
	//
	// When HaltSubReconcilers is returned as an error, execution continues as if no error was
	// returned.
	Reconciler SubReconciler

	Config Config
}

func (r *ResourceReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	_, err := r.SetupWithManagerYieldingController(ctx, mgr)
	return err
}

func (r *ResourceReconciler) SetupWithManagerYieldingController(ctx context.Context, mgr ctrl.Manager) (controller.Controller, error) {
	if r.Name == "" {
		r.Name = fmt.Sprintf("%sResourceReconciler", typeName(r.Type))
	}

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

func (r *ResourceReconciler) validate(ctx context.Context) error {
	// validate Type value
	if r.Type == nil {
		return fmt.Errorf("ResourceReconciler %q must define Type", r.Name)
	}

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

func (r *ResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	originalResource := r.Type.DeepCopyObject().(client.Object)

	if err := c.Get(ctx, req.NamespacedName, originalResource); err != nil {
		if apierrs.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch resource")
		return ctrl.Result{}, err
	}
	resource := originalResource.DeepCopyObject().(client.Object)

	if defaulter, ok := resource.(webhook.Defaulter); ok {
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
			return ctrl.Result{}, updateErr
		}
		c.Recorder.Eventf(resource, corev1.EventTypeNormal, "StatusUpdated",
			"Updated status")
	}

	// return original reconcile result
	return result, err
}

func (r *ResourceReconciler) reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	if resource.GetDeletionTimestamp() != nil && len(resource.GetFinalizers()) == 0 {
		// resource is being deleted and has no pending finalizers, nothing to do
		return ctrl.Result{}, nil
	}

	result, err := r.Reconciler.Reconcile(ctx, resource)
	if err != nil && !errors.Is(err, HaltSubReconcilers) {
		return ctrl.Result{}, err
	}

	r.copyGeneration(resource)
	return result, nil
}

func (r *ResourceReconciler) initializeConditions(obj client.Object) {
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

func (r *ResourceReconciler) conditions(obj client.Object) []metav1.Condition {
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

func (r *ResourceReconciler) copyGeneration(obj client.Object) {
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

func (r *ResourceReconciler) hasStatus(obj client.Object) bool {
	status := r.status(obj)
	return status != nil
}

func (r *ResourceReconciler) status(obj client.Object) interface{} {
	if obj == nil {
		return nil
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
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
// each proposed condition that is otherwise equivlent to the original value.
// This method is useful to prevent updating the status for a resource that is
// otherwise unchanged.
func (r *ResourceReconciler) syncLastTransitionTime(proposed, original []metav1.Condition) {
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
type AggregateReconciler struct {
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

	// Type of resource to reconcile
	Type client.Object
	// Request of resource to reconcile. Only the specific resource matching the namespace and name
	// is reconciled. The namespace may be empty for cluster scoped resources.
	Request ctrl.Request

	// Reconciler is called for each reconciler request with the resource being reconciled.
	// Typically, Reconciler is a Sequence of multiple SubReconcilers.
	//
	// When HaltSubReconcilers is returned as an error, execution continues as if no error was
	// returned.
	//
	// +optional
	Reconciler SubReconciler

	// DesiredResource returns the desired resource to create/update, or nil if
	// the resource should not exist.
	//
	// Expected function signature:
	//     func(ctx context.Context, resource client.Object) (client.Object, error)
	//
	// +optional
	DesiredResource interface{}

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

	// Deprecated SemanticEquals is no longer used, the field can be removed. Equality is
	// now determined based on the resource mutated by MergeBeforeUpdate
	SemanticEquals interface{}

	// Sanitize is called with an object before logging the value. Any value may
	// be returned. A meaningful subset of the resource is typically returned,
	// like the Spec.
	//
	// Expected function signature:
	//     func(resource client.Object) interface{}
	//
	// +optional
	Sanitize interface{}

	Config Config

	// stamp manages the lifecycle of the aggregated resource.
	stamp    *ResourceManager
	lazyInit sync.Once
}

func (r *AggregateReconciler) init() {
	r.lazyInit.Do(func() {
		if r.Reconciler == nil {
			r.Reconciler = Sequence{}
		}
		if r.DesiredResource == nil {
			r.DesiredResource = func(ctx context.Context, resource client.Object) (client.Object, error) {
				return resource, nil
			}
		}

		r.stamp = &ResourceManager{
			Name: r.Name,
			Type: r.Type,

			HarmonizeImmutableFields: r.HarmonizeImmutableFields,
			MergeBeforeUpdate:        r.MergeBeforeUpdate,
			Sanitize:                 r.Sanitize,
		}
	})
}

func (r *AggregateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	_, err := r.SetupWithManagerYieldingController(ctx, mgr)
	return err
}

func (r *AggregateReconciler) SetupWithManagerYieldingController(ctx context.Context, mgr ctrl.Manager) (controller.Controller, error) {
	if r.Name == "" {
		r.Name = fmt.Sprintf("%sAggregateReconciler", typeName(r.Type))
	}

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

	r.init()

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

func (r *AggregateReconciler) validate(ctx context.Context) error {
	// validate Type value
	if r.Type == nil {
		return fmt.Errorf("AggregateReconciler %q must define Type", r.Name)
	}
	// validate Request value
	if r.Request.Name == "" {
		return fmt.Errorf("AggregateReconciler %q must define Request", r.Name)
	}

	// validate Reconciler value
	if r.Reconciler == nil && r.DesiredResource == nil {
		return fmt.Errorf("AggregateReconciler %q must define Reconciler and/or DesiredResource", r.Name)
	}

	// validate DesiredResource function signature:
	//     nil
	//     func(ctx context.Context, resource client.Object) (client.Object, error)
	if r.DesiredResource != nil {
		fn := reflect.TypeOf(r.DesiredResource)
		if fn.NumIn() != 2 || fn.NumOut() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.Out(0)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.Out(1)) {
			return fmt.Errorf("AggregateReconciler %q must implement DesiredResource: nil | func(context.Context, %s) (%s, error), found: %s", r.Name, reflect.TypeOf(r.Type), reflect.TypeOf(r.Type), fn)
		}
	}

	return nil
}

func (r *AggregateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != r.Request.Namespace || req.Name != r.Request.Name {
		// ignore other requests
		return ctrl.Result{}, nil
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

	r.init()

	resource := r.Type.DeepCopyObject().(client.Object)
	if err := c.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrs.IsNotFound(err) {
			// not found is ok
			resource.SetNamespace(r.Request.Namespace)
			resource.SetName(r.Request.Name)
		} else {
			log.Error(err, "unable to fetch resource")
			return ctrl.Result{}, err
		}
	}

	if !resource.GetDeletionTimestamp().IsZero() {
		// resource is being deleted, nothing to do
		return ctrl.Result{}, nil
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
		return ctrl.Result{}, err
	}
	_, err = r.stamp.Manage(ctx, resource, resource, desired)
	if err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}

func (r *AggregateReconciler) desiredResource(ctx context.Context, resource client.Object) (client.Object, error) {
	if resource.GetDeletionTimestamp() != nil {
		// the reconciled resource is pending deletion, cleanup the child resource
		return nil, nil
	}

	fn := reflect.ValueOf(r.DesiredResource)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(resource.DeepCopyObject()),
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

const requestStashKey StashKey = "reconciler-runtime:request"
const configStashKey StashKey = "reconciler-runtime:config"
const originalConfigStashKey StashKey = "reconciler-runtime:originalConfig"
const resourceTypeStashKey StashKey = "reconciler-runtime:resourceType"
const originalResourceTypeStashKey StashKey = "reconciler-runtime:originalResourceType"
const additionalConfigsStashKey StashKey = "reconciler-runtime:additionalConfigs"

func StashRequest(ctx context.Context, req ctrl.Request) context.Context {
	return context.WithValue(ctx, requestStashKey, req)
}

// RetrieveRequest returns the reconciler Request from the context, or empty if not found.
func RetrieveRequest(ctx context.Context) ctrl.Request {
	value := ctx.Value(requestStashKey)
	if req, ok := value.(ctrl.Request); ok {
		return req
	}
	return ctrl.Request{}
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

// Deprecated use StashOriginalConfig
var StashParentConfig = StashOriginalConfig

func StashOriginalConfig(ctx context.Context, resourceConfig Config) context.Context {
	return context.WithValue(ctx, originalConfigStashKey, resourceConfig)
}

// Deprecated use RetrieveOriginalConfig
var RetrieveParentConfig = RetrieveOriginalConfig

// RetrieveOriginalConfig returns the Config from the context used to load the reconciled resource. An
// error is returned if not found.
func RetrieveOriginalConfig(ctx context.Context) (Config, error) {
	value := ctx.Value(originalConfigStashKey)
	if config, ok := value.(Config); ok {
		return config, nil
	}
	return Config{}, fmt.Errorf("resource config must exist on the context. Check that the context is from a ResourceReconciler")
}

// Deprecated use RetrieveOriginalConfigOrDie
var RetrieveParentConfigOrDie = RetrieveOriginalConfigOrDie

// RetrieveOriginalConfigOrDie returns the Config from the context used to load the reconciled resource.
// Panics if not found.
func RetrieveOriginalConfigOrDie(ctx context.Context) Config {
	config, err := RetrieveOriginalConfig(ctx)
	if err != nil {
		panic(err)
	}
	return config
}

// Deprecated use StashResourceType
var StashCastParentType = StashResourceType

func StashResourceType(ctx context.Context, currentType client.Object) context.Context {
	return context.WithValue(ctx, resourceTypeStashKey, currentType)
}

// Deprecated use RetrieveResourceType
var RetrieveCastParentType = RetrieveResourceType

// RetrieveResourceType returns the reconciled resource type object, or nil if not found.
func RetrieveResourceType(ctx context.Context) client.Object {
	value := ctx.Value(resourceTypeStashKey)
	if currentType, ok := value.(client.Object); ok {
		return currentType
	}
	return nil
}

// Deprecated use StashOriginalResourceType
var StashParentType = StashOriginalResourceType

func StashOriginalResourceType(ctx context.Context, resourceType client.Object) context.Context {
	return context.WithValue(ctx, originalResourceTypeStashKey, resourceType)
}

// Deprecated use RetrieveOriginalResourceType
var RetrieveParentType = RetrieveOriginalResourceType

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
type SubReconciler interface {
	SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error
	Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error)
}

var (
	_ SubReconciler = (*SyncReconciler)(nil)
	_ SubReconciler = (*ChildReconciler)(nil)
	_ SubReconciler = (Sequence)(nil)
	_ SubReconciler = (*CastResource)(nil)
	_ SubReconciler = (*WithConfig)(nil)
	_ SubReconciler = (*WithFinalizer)(nil)
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
	//     func(ctx context.Context, resource client.Object) error
	//     func(ctx context.Context, resource client.Object) (ctrl.Result, error)
	Sync interface{}

	// Finalize does whatever work is necessary for the reconciler when the resource is pending
	// deletion. If this reconciler sets a finalizer it should do the necessary work to clean up
	// state the finalizer represents and then clear the finalizer.
	//
	// Expected function signature:
	//     func(ctx context.Context, resource client.Object) error
	//     func(ctx context.Context, resource client.Object) (ctrl.Result, error)
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
	//     func(ctx context.Context, resource client.Object) error
	//     func(ctx context.Context, resource client.Object) (ctrl.Result, error)
	if r.Sync == nil {
		return fmt.Errorf("SyncReconciler %q must implement Sync", r.Name)
	} else {
		castResourceType := RetrieveResourceType(ctx)
		fn := reflect.TypeOf(r.Sync)
		err := fmt.Errorf("SyncReconciler %q must implement Sync: func(context.Context, %s) error | func(context.Context, %s) (ctrl.Result, error), found: %s", r.Name, reflect.TypeOf(castResourceType), reflect.TypeOf(castResourceType), fn)
		if fn.NumIn() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castResourceType).AssignableTo(fn.In(1)) {
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
	//     func(ctx context.Context, resource client.Object) error
	//     func(ctx context.Context, resource client.Object) (ctrl.Result, error)
	if r.Finalize != nil {
		castResourceType := RetrieveResourceType(ctx)
		fn := reflect.TypeOf(r.Finalize)
		err := fmt.Errorf("SyncReconciler %q must implement Finalize: nil | func(context.Context, %s) error | func(context.Context, %s) (ctrl.Result, error), found: %s", r.Name, reflect.TypeOf(castResourceType), reflect.TypeOf(castResourceType), fn)
		if fn.NumIn() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castResourceType).AssignableTo(fn.In(1)) {
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

func (r *SyncReconciler) Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	result := ctrl.Result{}

	if resource.GetDeletionTimestamp() == nil || r.SyncDuringFinalization {
		syncResult, err := r.sync(ctx, resource)
		if err != nil {
			log.Error(err, "unable to sync")
			return ctrl.Result{}, err
		}
		result = AggregateResults(result, syncResult)
	}

	if resource.GetDeletionTimestamp() != nil {
		finalizeResult, err := r.finalize(ctx, resource)
		if err != nil {
			log.Error(err, "unable to finalize")
			return ctrl.Result{}, err
		}
		result = AggregateResults(result, finalizeResult)
	}

	return result, nil
}

func (r *SyncReconciler) sync(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	fn := reflect.ValueOf(r.Sync)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(resource),
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

func (r *SyncReconciler) finalize(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	if r.Finalize == nil {
		return ctrl.Result{}, nil
	}

	fn := reflect.ValueOf(r.Finalize)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(resource),
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
type ChildReconciler struct {
	// Name used to identify this reconciler.  Defaults to `{ChildType}ChildReconciler`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// ChildType is the resource being created/updated/deleted by the reconciler. For example, a
	// reconciled resource Deployment would have a ReplicaSet as a child.
	ChildType client.Object
	// ChildListType is the listing type for the child type. For example,
	// PodList is the list type for Pod
	ChildListType client.ObjectList

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
	//
	// Expected function signature:
	//     func(ctx context.Context, resource client.Object) (client.Object, error)
	DesiredChild interface{}

	// ReflectChildStatusOnParent updates the reconciled resource's status with values from the
	// child. Select types of error are passed, including:
	//   - apierrs.IsConflict
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

	// Deprecated SemanticEquals is no longer used, the field can be removed. Equality is
	// now determined based on the resource mutated by MergeBeforeUpdate
	SemanticEquals interface{}

	// ListOptions allows custom options to be use when listing potential child resources. Each
	// resource retrieved as part of the listing is confirmed via OurChild.
	//
	// Defaults to filtering by the reconciled resource's namespace:
	//     []client.ListOption{
	//         client.InNamespace(resource.GetNamespace()),
	//     }
	//
	// Expected function signature:
	//     func(ctx context.Context, resource client.Object) []client.ListOption
	//
	// +optional
	ListOptions interface{}

	// OurChild is used when there are multiple ChildReconciler for the same ChildType controlled
	// by the same reconciled resource. The function return true for child resources managed by
	// this ChildReconciler. Objects returned from the DesiredChild function should match this
	// function, otherwise they may be orphaned. If not specified, all children match.
	//
	// OurChild is required when a Finalizer is defined or SkipOwnerReference is true.
	//
	// Expected function signature:
	//     func(resource, child client.Object) bool
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

	stamp    *ResourceManager
	lazyInit sync.Once
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

func (r *ChildReconciler) validate(ctx context.Context) error {
	castResourceType := RetrieveResourceType(ctx)

	// default implicit values
	if r.Finalizer != "" {
		r.SkipOwnerReference = true
	}

	// validate ChildType value
	if r.ChildType == nil {
		return fmt.Errorf("ChildReconciler %q must define ChildType", r.Name)
	}

	// validate ChildListType value
	if r.ChildListType == nil {
		return fmt.Errorf("ChildReconciler %q must define ChildListType", r.Name)
	}

	// validate DesiredChild function signature:
	//     func(ctx context.Context, resource client.Object) (client.Object, error)
	if r.DesiredChild == nil {
		return fmt.Errorf("ChildReconciler %q must implement DesiredChild", r.Name)
	} else {
		fn := reflect.TypeOf(r.DesiredChild)
		if fn.NumIn() != 2 || fn.NumOut() != 2 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castResourceType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.Out(0)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.Out(1)) {
			return fmt.Errorf("ChildReconciler %q must implement DesiredChild: func(context.Context, %s) (%s, error), found: %s", r.Name, reflect.TypeOf(castResourceType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate ReflectChildStatusOnParent function signature:
	//     func(resource, child client.Object, err error)
	if r.ReflectChildStatusOnParent == nil {
		return fmt.Errorf("ChildReconciler %q must implement ReflectChildStatusOnParent", r.Name)
	} else {
		fn := reflect.TypeOf(r.ReflectChildStatusOnParent)
		if fn.NumIn() != 3 || fn.NumOut() != 0 ||
			!reflect.TypeOf(castResourceType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf((*error)(nil)).Elem().AssignableTo(fn.In(2)) {
			return fmt.Errorf("ChildReconciler %q must implement ReflectChildStatusOnParent: func(%s, %s, error), found: %s", r.Name, reflect.TypeOf(castResourceType), reflect.TypeOf(r.ChildType), fn)
		}
	}

	// validate ListOptions function signature:
	//     nil
	//     func(ctx context.Context, resource client.Object) []client.ListOption
	if r.ListOptions != nil {
		fn := reflect.TypeOf(r.ListOptions)
		if fn.NumIn() != 2 || fn.NumOut() != 1 ||
			!reflect.TypeOf((*context.Context)(nil)).Elem().AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(castResourceType).AssignableTo(fn.In(1)) ||
			!reflect.TypeOf([]client.ListOption{}).AssignableTo(fn.Out(0)) {
			return fmt.Errorf("ChildReconciler %q must implement ListOptions: nil | func(context.Context, %s) []client.ListOption, found: %s", r.Name, reflect.TypeOf(castResourceType), fn)
		}
	}

	// validate OurChild function signature:
	//     nil
	//     func(resource, child client.Object) bool
	if r.OurChild != nil {
		fn := reflect.TypeOf(r.OurChild)
		if fn.NumIn() != 2 || fn.NumOut() != 1 ||
			!reflect.TypeOf(castResourceType).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.ChildType).AssignableTo(fn.In(1)) ||
			fn.Out(0).Kind() != reflect.Bool {
			return fmt.Errorf("ChildReconciler %q must implement OurChild: nil | func(%s, %s) bool, found: %s", r.Name, reflect.TypeOf(castResourceType), reflect.TypeOf(r.ChildType), fn)
		}
	}
	if r.OurChild == nil && r.SkipOwnerReference {
		// OurChild is required when SkipOwnerReference is true
		return fmt.Errorf("ChildReconciler %q must implement OurChild since owner references are not used", r.Name)
	}

	return nil
}

func (r *ChildReconciler) Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	c := RetrieveConfigOrDie(ctx)

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("childType", gvk(r.ChildType, c.Scheme()))
	ctx = logr.NewContext(ctx, log)

	r.lazyInit.Do(func() {
		r.stamp = &ResourceManager{
			Name:                     r.Name,
			Type:                     r.ChildType,
			Finalizer:                r.Finalizer,
			TrackDesired:             r.SkipOwnerReference,
			HarmonizeImmutableFields: r.HarmonizeImmutableFields,
			MergeBeforeUpdate:        r.MergeBeforeUpdate,
			Sanitize:                 r.Sanitize,
		}
	})

	child, err := r.reconcile(ctx, resource)
	if resource.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, err
	}
	if err != nil {
		if apierrs.IsAlreadyExists(err) {
			// check if the resource blocking create is owned by the reconciled resource.
			// the created child from a previous turn may be slow to appear in the informer cache, but shouldn't appear
			// on the reconciled resource as being not ready.
			apierr := err.(apierrs.APIStatus)
			conflicted := newEmpty(r.ChildType).(client.Object)
			_ = c.APIReader.Get(ctx, types.NamespacedName{Namespace: resource.GetNamespace(), Name: apierr.Status().Details.Name}, conflicted)
			if r.ourChild(resource, conflicted) {
				// skip updating the reconciled resource's status, fail and try again
				return ctrl.Result{}, err
			}
			log.Info("unable to reconcile child, not owned", "child", namespaceName(conflicted), "ownerRefs", conflicted.GetOwnerReferences())
			r.reflectChildStatusOnParent(resource, child, err)
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to reconcile child")
		return ctrl.Result{}, err
	}
	r.reflectChildStatusOnParent(resource, child, err)

	return ctrl.Result{}, nil
}

func (r *ChildReconciler) reconcile(ctx context.Context, resource client.Object) (client.Object, error) {
	log := logr.FromContextOrDiscard(ctx)
	pc := RetrieveOriginalConfigOrDie(ctx)
	c := RetrieveConfigOrDie(ctx)

	actual := newEmpty(r.ChildType).(client.Object)
	children := newEmpty(r.ChildListType).(client.ObjectList)
	if err := c.List(ctx, children, r.listOptions(ctx, resource)...); err != nil {
		return nil, err
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
				return nil, err
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
		return nil, err
	}
	if desired != nil {
		if !r.SkipOwnerReference {
			if err := ctrl.SetControllerReference(resource, desired, c.Scheme()); err != nil {
				return nil, err
			}
		}
		if !r.ourChild(resource, desired) {
			log.Info("object returned from DesiredChild does not match OurChild, this can result in orphaned children", "child", namespaceName(desired))
		}
	}

	// create/update/delete desired child
	return r.stamp.Manage(ctx, resource, actual, desired)
}

func (r *ChildReconciler) desiredChild(ctx context.Context, resource client.Object) (client.Object, error) {
	if resource.GetDeletionTimestamp() != nil {
		// the reconciled resource is pending deletion, cleanup the child resource
		return nil, nil
	}

	fn := reflect.ValueOf(r.DesiredChild)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(resource),
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

func (r *ChildReconciler) reflectChildStatusOnParent(resource, child client.Object, err error) {
	fn := reflect.ValueOf(r.ReflectChildStatusOnParent)
	args := []reflect.Value{
		reflect.ValueOf(resource),
		reflect.ValueOf(child),
		reflect.ValueOf(err),
	}
	if resource == nil {
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

func (r *ChildReconciler) filterChildren(resource client.Object, children client.ObjectList) []client.Object {
	childrenValue := reflect.ValueOf(children).Elem()
	itemsValue := childrenValue.FieldByName("Items")
	items := []client.Object{}
	for i := 0; i < itemsValue.Len(); i++ {
		obj := itemsValue.Index(i).Addr().Interface().(client.Object)
		if r.ourChild(resource, obj) {
			items = append(items, obj)
		}
	}
	return items
}

func (r *ChildReconciler) listOptions(ctx context.Context, resource client.Object) []client.ListOption {
	if r.ListOptions == nil {
		return []client.ListOption{
			client.InNamespace(resource.GetNamespace()),
		}
	}
	fn := reflect.ValueOf(r.ListOptions)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(resource),
	})
	return out[0].Interface().([]client.ListOption)
}

func (r *ChildReconciler) ourChild(resource, obj client.Object) bool {
	if !r.SkipOwnerReference && !metav1.IsControlledBy(obj, resource) {
		return false
	}
	// TODO do we need to remove resources pending deletion?
	if r.OurChild == nil {
		return true
	}
	fn := reflect.ValueOf(r.OurChild)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(resource),
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

func (r Sequence) Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	aggregateResult := ctrl.Result{}
	for i, reconciler := range r {
		log := logr.FromContextOrDiscard(ctx).
			WithName(fmt.Sprintf("%d", i))
		ctx = logr.NewContext(ctx, log)

		result, err := reconciler.Reconcile(ctx, resource)
		if err != nil {
			return ctrl.Result{}, err
		}
		aggregateResult = AggregateResults(result, aggregateResult)
	}

	return aggregateResult, nil
}

// Deprecated use CastResource
type CastParent = CastResource

// CastResource casts the ResourceReconciler's type by projecting the resource data
// onto a new struct. Casting the reconciled resource is useful to create cross
// cutting reconcilers that can operate on common portion of multiple  resources,
// commonly referred to as a duck type.
type CastResource struct {
	// Name used to identify this reconciler.  Defaults to `{Type}CastResource`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Type of resource to reconcile
	Type client.Object

	// Reconciler is called for each reconciler request with the reconciled resource. Typically a
	// Sequence is used to compose multiple SubReconcilers.
	Reconciler SubReconciler
}

func (r *CastResource) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.Name == "" {
		r.Name = fmt.Sprintf("%sCastResource", typeName(r.Type))
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("castResourceType", typeName(r.Type))
	ctx = logr.NewContext(ctx, log)

	if err := r.validate(ctx); err != nil {
		return err
	}
	return r.Reconciler.SetupWithManager(ctx, mgr, bldr)
}

func (r *CastResource) validate(ctx context.Context) error {
	// validate Type value
	if r.Type == nil {
		return fmt.Errorf("CastResource %q must define Type", r.Name)
	}

	// validate Reconciler value
	if r.Reconciler == nil {
		return fmt.Errorf("CastResource %q must define Reconciler", r.Name)
	}

	return nil
}

func (r *CastResource) Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name).
		WithValues("castResourceType", typeName(r.Type))
	ctx = logr.NewContext(ctx, log)

	ctx, castResource, err := r.cast(ctx, resource)
	if err != nil {
		return ctrl.Result{}, err
	}
	castOriginal := castResource.DeepCopyObject().(client.Object)
	result, err := r.Reconciler.Reconcile(ctx, castResource)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !equality.Semantic.DeepEqual(castResource, castOriginal) {
		// patch the reconciled resource with the updated duck values
		patch, err := NewPatch(castOriginal, castResource)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = patch.Apply(resource)
		if err != nil {
			return ctrl.Result{}, err
		}

	}
	return result, nil
}

func (r *CastResource) cast(ctx context.Context, resource client.Object) (context.Context, client.Object, error) {
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, nil, err
	}
	castResource := newEmpty(r.Type).(client.Object)
	err = json.Unmarshal(data, castResource)
	if err != nil {
		return nil, nil, err
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

	// Reconciler is called for each reconciler request with the reconciled
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
	c, err := r.Config(ctx, RetrieveConfigOrDie(ctx))
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

func (r *WithConfig) Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	c, err := r.Config(ctx, RetrieveConfigOrDie(ctx))
	if err != nil {
		return ctrl.Result{}, err
	}
	ctx = StashConfig(ctx, c)
	return r.Reconciler.Reconcile(ctx, resource)
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

func (r *WithFinalizer) Reconcile(ctx context.Context, resource client.Object) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if resource.GetDeletionTimestamp() == nil {
		if err := AddFinalizer(ctx, resource, r.Finalizer); err != nil {
			return ctrl.Result{}, err
		}
	}
	result, err := r.Reconciler.Reconcile(ctx, resource)
	if err != nil {
		return result, err
	}
	if resource.GetDeletionTimestamp() != nil {
		if err := ClearFinalizer(ctx, resource, r.Finalizer); err != nil {
			return ctrl.Result{}, err
		}
	}
	return result, err
}

// ResourceManager compares the actual and desired resources to create/update/delete as desired.
type ResourceManager struct {
	// Name used to identify this reconciler.  Defaults to `{Type}ResourceManager`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// Type is the resource being created/updated/deleted by the reconciler.
	Type client.Object

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

	// Deprecated SemanticEquals is no longer used, the field can be removed. Equality is
	// now determined based on the resource mutated by MergeBeforeUpdate
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

	// mutationCache holds patches received from updates to a resource made by
	// mutation webhooks. This cache is used to avoid unnecessary update calls
	// that would actually have no effect.
	mutationCache *cache.Expiring
	lazyInit      sync.Once
}

func (r *ResourceManager) Setup(ctx context.Context) error {
	if r.Name == "" {
		r.Name = fmt.Sprintf("%sResourceManager", typeName(r.Type))
	}

	return r.validate(ctx)
}

func (r *ResourceManager) validate(ctx context.Context) error {
	// validate Type value
	if r.Type == nil {
		return fmt.Errorf("ResourceManager %q must define Type", r.Name)
	}

	// validate HarmonizeImmutableFields function signature:
	//     nil
	//     func(current, desired client.Object)
	if r.HarmonizeImmutableFields != nil {
		fn := reflect.TypeOf(r.HarmonizeImmutableFields)
		if fn.NumIn() != 2 || fn.NumOut() != 0 ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.In(1)) {
			return fmt.Errorf("ResourceManager %q must implement HarmonizeImmutableFields: nil | func(%s, %s), found: %s", r.Name, reflect.TypeOf(r.Type), reflect.TypeOf(r.Type), fn)
		}
	}

	// validate MergeBeforeUpdate function signature:
	//     func(current, desired client.Object)
	if r.MergeBeforeUpdate == nil {
		return fmt.Errorf("ResourceManager %q must define MergeBeforeUpdate", r.Name)
	} else {
		fn := reflect.TypeOf(r.MergeBeforeUpdate)
		if fn.NumIn() != 2 || fn.NumOut() != 0 ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.In(0)) ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.In(1)) {
			return fmt.Errorf("ResourceManager %q must implement MergeBeforeUpdate: func(%s, %s), found: %s", r.Name, reflect.TypeOf(r.Type), reflect.TypeOf(r.Type), fn)
		}
	}

	// validate Sanitize function signature:
	//     nil
	//     func(child client.Object) interface{}
	if r.Sanitize != nil {
		fn := reflect.TypeOf(r.Sanitize)
		if fn.NumIn() != 1 || fn.NumOut() != 1 ||
			!reflect.TypeOf(r.Type).AssignableTo(fn.In(0)) {
			return fmt.Errorf("ResourceManager %q must implement Sanitize: nil | func(%s) interface{}, found: %s", r.Name, reflect.TypeOf(r.Type), fn)
		}
	}

	return nil
}

// Manage a specific resource to create/update/delete based on the actual and desired state. The
// resource is the reconciled resource and used to record events for mutations. The actual and
// desired objects represent the managed resource and must be compatible with the type field.
func (r *ResourceManager) Manage(ctx context.Context, resource, actual, desired client.Object) (client.Object, error) {
	log := logr.FromContextOrDiscard(ctx)
	pc := RetrieveOriginalConfigOrDie(ctx)
	c := RetrieveConfigOrDie(ctx)

	r.lazyInit.Do(func() {
		r.mutationCache = cache.NewExpiring()
	})

	if actual == nil && desired == nil {
		return nil, nil
	}

	// delete resource if no longer needed
	if desired == nil {
		if !actual.GetCreationTimestamp().Time.IsZero() {
			log.Info("deleting unwanted resource", "resource", namespaceName(actual))
			if err := c.Delete(ctx, actual); err != nil {
				log.Error(err, "unable to delete unwanted resource", "resource", namespaceName(actual))
				pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "DeleteFailed",
					"Failed to delete %s %q: %v", typeName(actual), actual.GetName(), err)
				return nil, err
			}
			pc.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted",
				"Deleted %s %q", typeName(actual), actual.GetName())

			if err := ClearFinalizer(ctx, resource, r.Finalizer); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	if err := AddFinalizer(ctx, resource, r.Finalizer); err != nil {
		return nil, err
	}

	// create resource if it doesn't exist
	if actual.GetCreationTimestamp().Time.IsZero() {
		log.Info("creating resource", "resource", r.sanitize(desired))
		if err := c.Create(ctx, desired); err != nil {
			log.Error(err, "unable to create resource", "resource", namespaceName(desired))
			pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create %s %q: %v", typeName(desired), desired.GetName(), err)
			return nil, err
		}
		if r.TrackDesired {
			// normally tracks should occur before API operations, but when creating a resource with a
			// generated name, we need to know the actual resource name.
			if err := c.Tracker.TrackChild(ctx, resource, desired, c.Scheme()); err != nil {
				return nil, err
			}
		}
		pc.Recorder.Eventf(resource, corev1.EventTypeNormal, "Created",
			"Created %s %q", typeName(desired), desired.GetName())
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

	// update resource with desired changes
	current := actual.DeepCopyObject().(client.Object)
	r.mergeBeforeUpdate(current, desiredPatched)
	if equality.Semantic.DeepEqual(current, actual) {
		// resource is unchanged
		log.Info("resource is in sync, no update required")
		return actual, nil
	}
	log.Info("updating resource", "diff", cmp.Diff(r.sanitize(actual), r.sanitize(current)))
	if r.TrackDesired {
		if err := c.Tracker.TrackChild(ctx, resource, current, c.Scheme()); err != nil {
			return nil, err
		}
	}
	if err := c.Update(ctx, current); err != nil {
		log.Error(err, "unable to update resource", "resource", namespaceName(current))
		pc.Recorder.Eventf(resource, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update %s %q: %v", typeName(current), current.GetName(), err)
		return nil, err
	}

	// capture admission mutation patch
	base := current.DeepCopyObject().(client.Object)
	r.mergeBeforeUpdate(base, desired)
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

func (r *ResourceManager) harmonizeImmutableFields(current, desired client.Object) {
	if r.HarmonizeImmutableFields == nil {
		return
	}
	fn := reflect.ValueOf(r.HarmonizeImmutableFields)
	fn.Call([]reflect.Value{
		reflect.ValueOf(current),
		reflect.ValueOf(desired),
	})
}

func (r *ResourceManager) mergeBeforeUpdate(current, desired client.Object) {
	fn := reflect.ValueOf(r.MergeBeforeUpdate)
	fn.Call([]reflect.Value{
		reflect.ValueOf(current),
		reflect.ValueOf(desired),
	})
}

func (r *ResourceManager) sanitize(resource client.Object) interface{} {
	if r.Sanitize == nil {
		return resource
	}
	if resource == nil {
		return nil
	}

	// avoid accidental mutations in Sanitize method
	resource = resource.DeepCopyObject().(client.Object)

	fn := reflect.ValueOf(r.Sanitize)
	out := fn.Call([]reflect.Value{
		reflect.ValueOf(resource),
	})
	var sanitized interface{}
	if !out[0].IsNil() {
		sanitized = out[0].Interface()
	}
	return sanitized
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
	cast := &CastResource{
		Type: resourceType,
		Reconciler: &SyncReconciler{
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

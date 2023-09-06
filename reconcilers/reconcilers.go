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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// SubReconciler are participants in a larger reconciler request. The resource
// being reconciled is passed directly to the sub reconciler.
type SubReconciler[Type client.Object] interface {
	SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error
	Reconcile(ctx context.Context, resource Type) (Result, error)
}

var (
	// ErrQuiet are not logged or recorded as events within reconciler-runtime.
	// They are propagated as errors unless otherwise defined.
	//
	// Test for ErrQuiet with errors.Is(err, ErrQuiet)
	ErrQuiet = fmt.Errorf("quiet errors are returned as errors, but not logged or recorded")

	// ErrHaltSubReconcilers is an error that instructs SubReconcilers to stop processing the request,
	// while the root reconciler proceeds as if there was no error. ErrHaltSubReconcilers may be
	// wrapped by other errors.
	//
	// ErrHaltSubReconcilers wraps ErrQuiet to suppress spurious logs.
	//
	// See documentation for the specific SubReconciler caller to see how they handle this case.
	ErrHaltSubReconcilers = fmt.Errorf("stop processing SubReconcilers, without returning an error: %w", ErrQuiet)

	// Deprecated HaltSubReconcilers use ErrHaltSubReconcilers instead
	HaltSubReconcilers = ErrHaltSubReconcilers
)

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

func EnqueueTracked(ctx context.Context) handler.EventHandler {
	c := RetrieveConfigOrDie(ctx)
	log := logr.FromContextOrDiscard(ctx)

	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []Request {
			var requests []Request

			items, err := c.Tracker.GetObservers(obj)
			if err != nil {
				if !errors.Is(err, ErrQuiet) {
					log.Error(err, "unable to get tracked requests")
				}
				return nil
			}

			for _, item := range items {
				requests = append(requests, Request{NamespacedName: item})
			}

			return requests
		},
	)
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

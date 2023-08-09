/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/duck"
	"github.com/vmware-labs/reconciler-runtime/tracker"
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
		Client:    duck.NewDuckAwareClientWrapper(cluster.GetClient().(client.WithWatch)),
		APIReader: duck.NewDuckAwareAPIReaderWrapper(cluster.GetAPIReader(), cluster.GetClient()),
		Recorder:  cluster.GetEventRecorderFor("controller"),
		Tracker:   c.Tracker,
	}
}

// TrackAndGet tracks the resources for changes and returns the current value. The track is
// registered even when the resource does not exists so that its creation can be tracked.
//
// Equivalent to calling both `c.Tracker.TrackObject(...)` and `c.Client.Get(...)`
func (c Config) TrackAndGet(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	// create synthetic resource to track from known type and request
	req := RetrieveRequest(ctx)
	resource := RetrieveResourceType(ctx).DeepCopyObject().(client.Object)
	resource.SetNamespace(req.Namespace)
	resource.SetName(req.Name)
	ref := obj.DeepCopyObject().(client.Object)
	ref.SetNamespace(key.Namespace)
	ref.SetName(key.Name)
	c.Tracker.TrackObject(ref, resource)

	return c.Get(ctx, key, obj, opts...)
}

// TrackAndList tracks the resources for changes and returns the current value.
//
// Equivalent to calling both `c.Tracker.TrackReference(...)` and `c.Client.List(...)`
func (c Config) TrackAndList(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	// create synthetic resource to track from known type and request
	req := RetrieveRequest(ctx)
	resource := RetrieveResourceType(ctx).DeepCopyObject().(client.Object)
	resource.SetNamespace(req.Namespace)
	resource.SetName(req.Name)

	or, err := reference.GetReference(c.Scheme(), list)
	if err != nil {
		return err
	}
	gvk := schema.FromAPIVersionAndKind(or.APIVersion, or.Kind)
	listOpts := (&client.ListOptions{}).ApplyOptions(opts)
	if listOpts.LabelSelector == nil {
		listOpts.LabelSelector = labels.Everything()
	}
	ref := tracker.Reference{
		APIGroup:  gvk.Group,
		Kind:      strings.TrimSuffix(gvk.Kind, "List"),
		Namespace: listOpts.Namespace,
		Selector:  listOpts.LabelSelector,
	}
	c.Tracker.TrackReference(ref, resource)

	return c.List(ctx, list, opts...)
}

// NewConfig creates a Config for a specific API type. Typically passed into a
// reconciler.
func NewConfig(mgr ctrl.Manager, apiType client.Object, syncPeriod time.Duration) Config {
	return Config{
		Tracker: tracker.New(mgr.GetScheme(), 2*syncPeriod),
	}.WithCluster(mgr)
}

var (
	_ SubReconciler[client.Object] = (*WithConfig[client.Object])(nil)
)

// Experimental: WithConfig injects the provided config into the reconcilers nested under it. For
// example, the client can be swapped to use a service account with different permissions, or to
// target an entirely different cluster.
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

/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package informers

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/manager"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Informers knows how to fetch informers for different group-version-kinds.
type Informers interface {
	// GetInformer creates if necessary and then fetches an informer for the
	// given group-version-kind. The informer, along with any watches on behalf
	// of event handlers added to the informer, will expire a certain period of
	// time after the last time it was fetched. The caller should therefore call
	// this method periodically. The cancellation function must called when the
	// informer is no longer needed so that expired informers are cleaned up.
	GetInformer(gvk schema.GroupVersionKind) (Informer, context.CancelFunc, error)

	// Start runs all the informers known to this informers instance until the
	// given context is closed. This method blocks until the context is closed
	// or an error occurs.
	Start(ctx context.Context) error
}

// Informer knows how to add watches to the underlying informer.
type Informer interface {
	// AddEventHandler adds an event handler to the shared informer and
	// dispatches reconciliations to the given reconciler.
	AddEventHandler(handler handler.EventHandler, reconciler reconcile.Reconciler) error
}

var (
	_ Informers = &informers{}
	_ Informer  = &informer{}
)

// New creates an Informers instance.
func New(lease time.Duration, sm manager.SuperManager, log logr.Logger) Informers {
	return &informers{
		log:   log,
		sm:    sm,
		lease: lease,
		index: make(map[schema.GroupKind]*expiringInformer),
	}
}

type expiringInformer struct {
	inf    *informer
	expiry time.Time
}

type informers struct {
	log   logr.Logger
	sm    manager.SuperManager
	lease time.Duration

	m     sync.Mutex // protects index
	index map[schema.GroupKind]*expiringInformer
}

func (is *informers) expiryTime() time.Time {
	return time.Now().Add(is.lease)
}

// GetInformer implements Informers.
func (is *informers) GetInformer(gvk schema.GroupVersionKind) (Informer, context.CancelFunc, error) {
	if i := is.maybeGetInformer(gvk); i != nil {
		i.expiry = is.expiryTime() // give the informer a new lease
		is.log.Info("GetInformer: returning existing informer", "GVK", gvk, "informer", i.inf)
		return i.inf, i.inf.cancelFunc, nil
	}

	// Create a new manager with its own cache which can be stopped when watches need to be stopped.
	mgr, err := is.sm.NewManager()
	if err != nil {
		return nil, nil, err
	}

	// Start the manager.
	cancel, err := is.sm.AddCancelable(mgr)
	if err != nil {
		return nil, nil, err
	}

	inf := &informer{
		log: is.log.WithName("informer").WithValues("GVK", gvk),
		mgr: mgr,
		gvk: gvk,
	}

	i := &expiringInformer{
		inf:    inf,
		expiry: is.expiryTime(),
	}

	inf.cancelFunc = func() {
		is.checkInformerExpiry(gvk)
		cancel()
	}

	inf.ctlrs = make(map[reconcile.Reconciler]controller.Controller)

	if old := is.getAndSetInformer(gvk, i); old != nil {
		cancel()                     // tidy up the new informer since we no longer need it
		old.expiry = is.expiryTime() // give the informer a new lease
		is.log.Info("GetInformer: returning existing informer after race", "GVK", gvk, "informer", old.inf)
		return old.inf, old.inf.cancelFunc, nil
	}

	is.log.Info("GetInformer: returning new informer", "GVK", gvk, "informer", i.inf)
	return i.inf, i.inf.cancelFunc, nil
}

func (is *informers) maybeGetInformer(gvk schema.GroupVersionKind) *expiringInformer {
	is.m.Lock()
	defer is.m.Unlock()
	return is.index[gvk.GroupKind()]
}

func (is *informers) getAndSetInformer(gvk schema.GroupVersionKind, new *expiringInformer) *expiringInformer {
	is.m.Lock()
	defer is.m.Unlock()
	old := is.index[gvk.GroupKind()]
	if old == nil {
		is.index[gvk.GroupKind()] = new
	}
	return old
}

func (is *informers) checkInformerExpiry(gvk schema.GroupVersionKind) {
	is.m.Lock()
	defer is.m.Unlock()
	current := is.index[gvk.GroupKind()]
	if current.expiry.After(time.Now()) {
		delete(is.index, gvk.GroupKind())
		is.log.Info("checkInformerExpiry: deleted expired informer", "GroupKind", gvk.GroupKind(), "informer", current)
	}
}

// Start implements Informers.
func (is *informers) Start(ctx context.Context) error {
	is.log.Info("Start: starting informers")
	return is.sm.Start(ctx)
}

type informer struct {
	log        logr.Logger
	mgr        ctrl.Manager
	gvk        schema.GroupVersionKind
	cancelFunc context.CancelFunc

	m     sync.Mutex // protects ctlrs
	ctlrs map[reconcile.Reconciler]controller.Controller
}

// AddEventHandler implements Informer.
func (i *informer) AddEventHandler(handler handler.EventHandler, reconciler reconcile.Reconciler) error {
	// Get a controller for the given reconciler.
	ctlr, err := i.getController(reconciler)
	if err != nil {
		return err
	}

	// Add a watch to the controller.
	// TODO: make the following a metadata only watch. Note that builder.OnlyMetadata
	// relies on a manager's scheme to get the GVK from an incoming object - so
	// a different approach may be necessary here.
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(i.gvk)
	i.log.Info("AddEventHandler: adding controller watch")
	return ctlr.Watch(&source.Kind{Type: obj}, handler)
}

func (i *informer) getController(reconciler reconcile.Reconciler) (controller.Controller, error) {
	if ctlr := i.maybeGetController(reconciler); ctlr != nil {
		return ctlr, nil
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(i.gvk)

	// Create a controller for the given reconciler. This will start the controller.
	ctlr, err := builder.ControllerManagedBy(i.mgr).
		For(obj).
		Build(reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
			return reconciler.Reconcile(ctx, req)
		}))
	if err != nil {
		i.log.Error(err, "getController: failed to create controller", "reconciler", reconciler)
		return nil, err
	}

	if old := i.getAndSetController(reconciler, ctlr); old != nil {
		// TODO: tidy up ctlr.
		i.log.Info("getController: returning existing controller after race", "controller", old)
		return old, nil
	}

	i.log.Info("getController: returning new controller", "controller", ctlr)
	return ctlr, nil
}

func (i *informer) maybeGetController(reconciler reconcile.Reconciler) controller.Controller {
	i.m.Lock()
	defer i.m.Unlock()
	return i.ctlrs[reconciler]
}

func (i *informer) getAndSetController(reconciler reconcile.Reconciler, new controller.Controller) controller.Controller {
	i.m.Lock()
	defer i.m.Unlock()
	old := i.ctlrs[reconciler]
	if old == nil {
		i.ctlrs[reconciler] = new
	}
	return old
}

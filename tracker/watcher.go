/*
Copyright 2019-2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tracker

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/client"
	"github.com/vmware-labs/reconciler-runtime/manager"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type watchFunc func(gvk schema.GroupVersionKind) (context.CancelFunc, error)

type watcher struct {
	tracker GKAwareTracker
	watch   watchFunc

	m       sync.Mutex // protects watches
	watches map[schema.GroupKind]context.CancelFunc
}

var (
	_ Tracker = (*watcher)(nil)
)

var nopReconciler = reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
})

// NewWatcher returns an implementation of Tracker that lets a Reconciler register a
// particular resource as watching a resource for a particular lease duration.
// This watch must be refreshed periodically (e.g. by a controller resync) or
// it will expire.
func NewWatcher(sm manager.SuperManager, lease time.Duration, log logr.Logger, scheme *runtime.Scheme, enqueueTracked func(by client.Object, t Tracker) handler.EventHandler) Tracker {
	tracker := New(lease, log).(GKAwareTracker)
	return NewWatchingTracker(tracker, func(gvk schema.GroupVersionKind) (context.CancelFunc, error) {
		rObj, err := scheme.New(gvk)
		obj := rObj.(crclient.Object)
		if err != nil {
			return nil, err
		}

		// Create a new manager with its own cache which can be stopped when watches need to be stopped.
		mgr, err := sm.NewManager()
		if err != nil {
			return nil, err
		}

		ctrl, err := builder.ControllerManagedBy(mgr).Build(nopReconciler)
		if err != nil {
			return nil, err
		}

		// Start the manager. This will start the above controller.
		cancel, err := sm.AddCancelable(mgr)
		if err != nil {
			return nil, err
		}

		if err := ctrl.Watch(&source.Kind{Type: obj}, enqueueTracked(obj, tracker)); err != nil {
			cancel()
			return nil, err
		}

		return cancel, nil
	})
}

// GKAwareTracker extends Tracker with a function for checking whether a given group and kind is being tracked.
type GKAwareTracker interface {
	Tracker

	// Tracking returns true if and only if any references with the given group
	// and kind are being tracked.
	Tracking(groupKind schema.GroupKind) bool
}

// Deprecated: use NewWatcher
func NewWatchingTracker(tracker GKAwareTracker, watch watchFunc) Tracker {
	return &watcher{
		tracker: tracker,
		watch:   watch,
		watches: map[schema.GroupKind]context.CancelFunc{},
	}
}

// Track tells us that "obj" is tracking changes to the referenced object
// and establishes an informer watch for the group kind, and version of the
// referenced object. Any existing informer for the same group and kind, but
// potentially a distinct version can be reused since we are only using the
// informer to watch for metadata changes and these are version independent.
func (i *watcher) Track(ref Key, obj types.NamespacedName) error {
	if err := i.startWatch(ref); err != nil {
		return err
	}

	return i.tracker.Track(ref, obj)
}

func (i *watcher) startWatch(ref Key) error {
	i.m.Lock() // TODO: this is held across alien calls, so use finer grain mutexes to avoid deadlocks
	defer i.m.Unlock()
	_, watching := i.watches[ref.GroupKind()]

	if watching {
		return nil
	}

	cancel, err := i.watch(ref.GroupVersionKind)
	if err != nil {
		return err
	}

	i.watches[ref.GroupKind()] = cancel
	return nil
}

// Lookup implements Tracker.
func (i *watcher) Lookup(ref Key) []types.NamespacedName {
	trackedObjects := i.tracker.Lookup(ref)

	if stopWatch := i.gcWatches(ref); stopWatch != nil {
		stopWatch()
	}

	return trackedObjects
}

func (i *watcher) gcWatches(ref Key) context.CancelFunc {
	groupKind := ref.GroupKind()
	i.m.Lock()
	defer i.m.Unlock()

	// Avoid garbage collection if not watching.
	if _, watching := i.watches[groupKind]; !watching {
		return nil
	}

	if !i.tracker.Tracking(groupKind) {
		stopWatch := i.watches[groupKind]
		delete(i.watches, groupKind)
		return stopWatch
	}
	return nil
}

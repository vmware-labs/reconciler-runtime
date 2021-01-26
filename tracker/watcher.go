/*
Copyright 2019-2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tracker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/informers"
	"github.com/vmware-labs/reconciler-runtime/reconcilers/stash/parent"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type watchFunc func(ctx context.Context, gvk schema.GroupVersionKind) (context.CancelFunc, error)

type watcher struct {
	log     logr.Logger
	tracker GKAwareTracker
	watch   watchFunc

	m       sync.Mutex // protects watches
	watches map[schema.GroupKind]context.CancelFunc
}

var (
	_ Tracker = (*watcher)(nil)
)

func reconcilerToReconcileFunc(reconciler reconcile.Reconciler) reconcile.Func {
	return func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		return reconciler.Reconcile(ctx, req)
	}
}

// NewWatcher returns an implementation of Tracker that lets a Reconciler register a
// particular resource as watching a resource for a particular lease duration.
// This watch must be refreshed periodically (e.g. by a controller resync) or
// it will expire.
func NewWatcher(is informers.Informers, lease time.Duration, log logr.Logger, enqueueTracked func(trackedGVK schema.GroupVersionKind, t Tracker) handler.EventHandler) Tracker {
	tracker := New(lease, log.WithName("tracker")).(GKAwareTracker)
	return NewWatchingTracker(tracker, func(ctx context.Context, gvk schema.GroupVersionKind) (context.CancelFunc, error) {
		parentReconciler := parent.RetrieveParentReconciler(ctx)
		if parentReconciler == nil {
			err := errors.New("parent reconciler not retrieved from context")
			log.Error(err, "watchFunc: failed")
			return nil, err
		}

		informer, cancel, err := is.GetInformer(gvk)
		if err != nil {
			return nil, err
		}

		if err := informer.AddEventHandler(enqueueTracked(gvk, tracker), parentReconciler); err != nil {
			cancel()
			return nil, err
		}

		return cancel, nil
	}, log)
}

// GKAwareTracker extends Tracker with a function for checking whether a given group and kind is being tracked.
type GKAwareTracker interface {
	Tracker

	// Tracking returns true if and only if any references with the given group
	// and kind are being tracked.
	Tracking(groupKind schema.GroupKind) bool
}

// Deprecated: use NewWatcher
func NewWatchingTracker(tracker GKAwareTracker, watch watchFunc, log logr.Logger) Tracker {
	return &watcher{
		log:     log,
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
func (i *watcher) Track(ctx context.Context, ref Key, obj types.NamespacedName) error {
	if err := i.startWatch(ctx, ref); err != nil {
		return err
	}

	return i.tracker.Track(ctx, ref, obj)
}

func (i *watcher) startWatch(ctx context.Context, ref Key) error {
	i.m.Lock() // TODO: this is held across alien calls, so use finer grain mutexes to avoid deadlocks
	defer i.m.Unlock()
	_, watching := i.watches[ref.GroupKind()]

	if watching {
		i.log.Info("startWatch: already watching", "GroupKind", ref.GroupKind())
		return nil
	}

	cancel, err := i.watch(ctx, ref.GroupVersionKind)
	if err != nil {
		return err
	}

	i.watches[ref.GroupKind()] = cancel
	i.log.Info("startWatch: successful", "GroupKind", ref.GroupKind())
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
		i.log.Info("gcWatches: not watching", "GroupKind", groupKind)
		return nil
	}

	if !i.tracker.Tracking(groupKind) {
		stopWatch := i.watches[groupKind]
		delete(i.watches, groupKind)
		i.log.Info("gcWatches: stopped watching", "GroupKind", groupKind)
		return stopWatch
	}
	return nil
}

/*
Copyright 2019-2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package watchtracker

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/client"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type watchFunc func(gvk schema.GroupVersionKind, controller controller.Controller) error

type impl struct {
	tracker tracker.Tracker
	watch   watchFunc

	m          sync.Mutex // protects watches and controller
	watches    map[schema.GroupKind]struct{}
	controller controller.Controller
}

var _ tracker.Tracker = &impl{}

// New returns an implementation of Tracker that lets a Reconciler register a
// particular resource as watching a resource for a particular lease duration.
// This watch must be refreshed periodically (e.g. by a controller resync) or
// it will expire.
func New(lease time.Duration, log logr.Logger, scheme *runtime.Scheme, enqueueTracked func(by client.Object, t tracker.Tracker) handler.EventHandler) tracker.Tracker {
	tracker := tracker.New(lease, log)
	return newWatchingTracker(tracker, func(gvk schema.GroupVersionKind, controller controller.Controller) error {
		rObj, err := scheme.New(gvk)
		obj := rObj.(crclient.Object)
		if err != nil {
			return err
		}

		return controller.Watch(&source.Kind{Type: obj}, enqueueTracked(obj, tracker))
	})
}

func newWatchingTracker(tracker tracker.Tracker, watch watchFunc) tracker.Tracker {
	return &impl{
		tracker: tracker,
		watch:   watch,
		watches: map[schema.GroupKind]struct{}{},
	}
}

// Controller injects a controller into this tracker which will be used to
// start watches.
func (i *impl) Controller(controller controller.Controller) {
	i.m.Lock()
	defer i.m.Unlock()
	i.controller = controller
}

// Track tells us that "obj" is tracking changes to the referenced object
// and establishes an informer watch for the group kind, and version of the
// referenced object. Any existing informer for the same group and kind, but
// potentially a distinct version can be reused since we are only using the
// informer to watch for metadata changes and these are version independent.
func (i *impl) Track(ref tracker.Key, obj types.NamespacedName) error {
	if err := i.startWatch(ref); err != nil {
		return err
	}

	return i.tracker.Track(ref, obj)
}

func (i *impl) startWatch(ref tracker.Key) error {
	gvk := schema.GroupVersionKind{
		Group:   ref.GroupKind.Group,
		Version: ref.Version,
		Kind:    ref.GroupKind.Kind,
	}
	i.m.Lock() // TODO: this is held across alien calls, so use finer grain mutexes to avoid deadlocks
	defer i.m.Unlock()
	_, watching := i.watches[ref.GroupKind]

	if watching {
		return nil
	}

	if err := i.watch(gvk, i.controller); err != nil {
		return err
	}

	i.watches[ref.GroupKind] = struct{}{}
	return nil
}

// Lookup implements Tracker.
func (i *impl) Lookup(ref tracker.Key) []types.NamespacedName {
	// TODO: garbage collect unnecessary watches after the call below
	return i.tracker.Lookup(ref)
}

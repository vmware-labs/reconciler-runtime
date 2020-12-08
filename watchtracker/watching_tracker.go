/*
Copyright 2019-2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package watchtracker

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// WatchingTracker defines an interface through which an object can register
// that it is tracking another object by reference and which will automatically
// establish an informer watch for the group, version, and kind of the tracked
// object.
type WatchingTracker interface {
	// Track tells us that "obj" is tracking changes to the referenced object
	// with the given version and establishes and informer watch for the group
	// and kind of the referenced object and the given version.
	Track(ref tracker.Key, version string, obj types.NamespacedName) error

	// Lookup returns actively tracked objects for the reference.
	Lookup(ref tracker.Key) []types.NamespacedName
}

type impl struct {
	tracker    tracker.Tracker
	controller controller.Controller
	scheme     *runtime.Scheme

	m       sync.Mutex // protects watches
	watches map[schema.GroupVersionKind]struct{}
}

// New returns an implementation of WatchingTracker that lets a Reconciler
// register a particular resource as watching a resource for
// a particular lease duration.  This watch must be refreshed
// periodically (e.g. by a controller resync) or it will expire.
func New(lease time.Duration, log logr.Logger, controller controller.Controller, scheme *runtime.Scheme) WatchingTracker {
	return newWatchingTracker(tracker.New(lease, log), controller, scheme)
}

func newWatchingTracker(tracker tracker.Tracker, controller controller.Controller, scheme *runtime.Scheme) WatchingTracker {
	return &impl{
		tracker:    tracker,
		controller: controller,
		scheme:     scheme,
		watches:    map[schema.GroupVersionKind]struct{}{},
	}
}

// Track implements Tracker.
func (i *impl) Track(ref tracker.Key, version string, obj types.NamespacedName) error {
	gvk := schema.GroupVersionKind{
		Group:   ref.GroupKind.Group,
		Version: version,
		Kind:    ref.GroupKind.Kind,
	}
	if err := i.watch(gvk); err != nil {
		return err
	}

	i.tracker.Track(ref, obj)
	return nil
}

func (i *impl) watch(gvk schema.GroupVersionKind) error {
	i.m.Lock()
	_, watching := i.watches[gvk]
	i.m.Unlock()

	if watching {
		return nil
	}

	obj, err := i.scheme.New(gvk)
	if err != nil {
		return err
	}

	err = i.controller.Watch(&source.Kind{Type: obj}, reconcilers.EnqueueTracked(obj, i.tracker, i.scheme))
	if err != nil {
		return err
	}

	i.m.Lock()
	i.watches[gvk] = struct{}{}
	i.m.Unlock()
	return nil
}

// Lookup implements Tracker.
func (i *impl) Lookup(ref tracker.Key) []types.NamespacedName {
	// TODO: garbage collect unnecessary watches after the call below
	return i.tracker.Lookup(ref)
}

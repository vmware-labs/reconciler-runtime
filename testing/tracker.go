/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware-labs/reconciler-runtime/tracker"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TrackRequest records that one object is tracking another object.
type TrackRequest struct {
	// Tracker is the object doing the tracking
	Tracker types.NamespacedName
	// Tracked is the object being tracked
	Tracked tracker.Key
}

type trackBy func(trackingObjNamespace, trackingObjName string) TrackRequest

func (t trackBy) By(trackingObjNamespace, trackingObjName string) TrackRequest {
	return t(trackingObjNamespace, trackingObjName)
}

func CreateTrackRequest(trackedObjGroup, trackedObjKind, trackedObjNamespace, trackedObjName string) trackBy {
	return func(trackingObjNamespace, trackingObjName string) TrackRequest {
		return TrackRequest{
			Tracked: tracker.Key{GroupKind: schema.GroupKind{Group: trackedObjGroup, Kind: trackedObjKind}, NamespacedName: types.NamespacedName{Namespace: trackedObjNamespace, Name: trackedObjName}},
			Tracker: types.NamespacedName{Namespace: trackingObjNamespace, Name: trackingObjName},
		}
	}
}

func NewTrackRequest(t, b client.Object, scheme *runtime.Scheme) TrackRequest {
	tracked, by := t.DeepCopyObject().(client.Object), b.DeepCopyObject().(client.Object)
	gvks, _, err := scheme.ObjectKinds(tracked)
	if err != nil {
		panic(err)
	}
	return TrackRequest{
		Tracked: tracker.Key{GroupKind: schema.GroupKind{Group: gvks[0].Group, Kind: gvks[0].Kind}, NamespacedName: types.NamespacedName{Namespace: tracked.GetNamespace(), Name: tracked.GetName()}},
		Tracker: types.NamespacedName{Namespace: by.GetNamespace(), Name: by.GetName()},
	}
}

const maxDuration = time.Duration(1<<63 - 1)

func createTracker(given []TrackRequest) *mockTracker {
	t := &mockTracker{Tracker: tracker.New(maxDuration)}
	for _, g := range given {
		t.Track(context.TODO(), g.Tracked, g.Tracker)
	}
	// reset tracked requests
	t.reqs = []TrackRequest{}
	return t
}

type mockTracker struct {
	tracker.Tracker
	reqs []TrackRequest
}

var _ tracker.Tracker = &mockTracker{}

func (t *mockTracker) Track(ctx context.Context, ref tracker.Key, obj types.NamespacedName) {
	t.Tracker.Track(ctx, ref, obj)
	t.reqs = append(t.reqs, TrackRequest{Tracked: ref, Tracker: obj})
}

func (t *mockTracker) TrackChild(ctx context.Context, parent, child client.Object, s *runtime.Scheme) error {
	gvks, _, err := s.ObjectKinds(child)
	if err != nil {
		return err
	}
	if len(gvks) != 1 {
		return fmt.Errorf("expected exactly one GVK, found: %s", gvks)
	}
	t.Track(
		ctx,
		tracker.NewKey(gvks[0], types.NamespacedName{Namespace: child.GetNamespace(), Name: child.GetName()}),
		types.NamespacedName{Namespace: parent.GetNamespace(), Name: parent.GetName()},
	)
	return nil
}

func (t *mockTracker) getTrackRequests() []TrackRequest {
	return append([]TrackRequest{}, t.reqs...)
}

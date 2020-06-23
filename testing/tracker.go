/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"time"

	"github.com/go-logr/logr/testing"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

func NewTrackRequest(t, b Factory, scheme *runtime.Scheme) TrackRequest {
	tracked, by := t.CreateObject(), b.CreateObject()
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

func createTracker() *mockTracker {
	return &mockTracker{Tracker: tracker.New(maxDuration, testing.NullLogger{}), reqs: []TrackRequest{}}
}

type mockTracker struct {
	tracker.Tracker
	reqs []TrackRequest
}

var _ tracker.Tracker = &mockTracker{}

func (t *mockTracker) Track(ref tracker.Key, obj types.NamespacedName) {
	t.Tracker.Track(ref, obj)
	t.reqs = append(t.reqs, TrackRequest{Tracked: ref, Tracker: obj})
}

func (t *mockTracker) getTrackRequests() []TrackRequest {
	result := []TrackRequest{}
	for _, req := range t.reqs {
		result = append(result, req)
	}
	return result
}

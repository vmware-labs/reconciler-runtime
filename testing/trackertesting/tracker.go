/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package trackertesting

import (
	"time"

	"github.com/go-logr/logr/testing"
	ftesting "github.com/vmware-labs/reconciler-runtime/testing/factorytesting"
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

func CreateTrackRequest(trackedObjGroup, trackedObjVersion, trackedObjKind, trackedObjNamespace, trackedObjName string) trackBy {
	return func(trackingObjNamespace, trackingObjName string) TrackRequest {
		return TrackRequest{
			Tracked: tracker.Key{GroupKind: schema.GroupKind{Group: trackedObjGroup, Kind: trackedObjKind}, Version: trackedObjVersion, NamespacedName: types.NamespacedName{Namespace: trackedObjNamespace, Name: trackedObjName}},
			Tracker: types.NamespacedName{Namespace: trackingObjNamespace, Name: trackingObjName},
		}
	}
}

func NewTrackRequest(t, b ftesting.Factory, scheme *runtime.Scheme) TrackRequest {
	tracked, by := t.CreateObject(), b.CreateObject()
	gvks, _, err := scheme.ObjectKinds(tracked)
	if err != nil {
		panic(err)
	}
	return TrackRequest{
		Tracked: tracker.Key{GroupKind: schema.GroupKind{Group: gvks[0].Group, Kind: gvks[0].Kind}, Version: gvks[0].Version, NamespacedName: types.NamespacedName{Namespace: tracked.GetNamespace(), Name: tracked.GetName()}},
		Tracker: types.NamespacedName{Namespace: by.GetNamespace(), Name: by.GetName()},
	}
}

const maxDuration = time.Duration(1<<63 - 1)

func CreateTracker() *MockTracker {
	return &MockTracker{Tracker: tracker.New(maxDuration, testing.NullLogger{}), reqs: []TrackRequest{}}
}

type MockTracker struct {
	tracker.Tracker
	reqs []TrackRequest
}

var _ tracker.Tracker = &MockTracker{}

func (t *MockTracker) Track(ref tracker.Key, obj types.NamespacedName) error {
	t.Tracker.Track(ref, obj)
	t.reqs = append(t.reqs, TrackRequest{Tracked: ref, Tracker: obj})
	return nil
}

func (t *MockTracker) GetTrackRequests() []TrackRequest {
	result := []TrackRequest{}
	for _, req := range t.reqs {
		result = append(result, req)
	}
	return result
}

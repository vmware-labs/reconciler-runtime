/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"time"

	"github.com/vmware-labs/reconciler-runtime/tracker"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TrackRequest records that one object is tracking another object.
type TrackRequest struct {
	// Tracker is the object doing the tracking
	Tracker types.NamespacedName

	// Deprecated use TrackedReference
	// Tracked is the object being tracked
	Tracked tracker.Key

	// TrackedReference is a ref to the object being tracked
	TrackedReference tracker.Reference
}

func (tr *TrackRequest) normalize() {
	if tr.TrackedReference != (tracker.Reference{}) {
		return
	}
	tr.TrackedReference = tracker.Reference{
		APIGroup:  tr.Tracked.GroupKind.Group,
		Kind:      tr.Tracked.GroupKind.Kind,
		Namespace: tr.Tracked.NamespacedName.Namespace,
		Name:      tr.Tracked.NamespacedName.Name,
	}
	tr.Tracked = tracker.Key{}
}

type trackBy func(trackingObjNamespace, trackingObjName string) TrackRequest

func (t trackBy) By(trackingObjNamespace, trackingObjName string) TrackRequest {
	return t(trackingObjNamespace, trackingObjName)
}

func CreateTrackRequest(trackedObjGroup, trackedObjKind, trackedObjNamespace, trackedObjName string) trackBy {
	return func(trackingObjNamespace, trackingObjName string) TrackRequest {
		return TrackRequest{
			TrackedReference: tracker.Reference{
				APIGroup:  trackedObjGroup,
				Kind:      trackedObjKind,
				Namespace: trackedObjNamespace,
				Name:      trackedObjName,
			},
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
		TrackedReference: tracker.Reference{
			APIGroup:  gvks[0].Group,
			Kind:      gvks[0].Kind,
			Namespace: tracked.GetNamespace(),
			Name:      tracked.GetName(),
		},
		Tracker: types.NamespacedName{Namespace: by.GetNamespace(), Name: by.GetName()},
	}
}

func createTracker(given []TrackRequest, scheme *runtime.Scheme) *mockTracker {
	t := &mockTracker{
		Tracker: tracker.New(scheme, 24*time.Hour),
		scheme:  scheme,
	}
	for _, g := range given {
		g.normalize()
		obj := &unstructured.Unstructured{}
		obj.SetNamespace(g.Tracker.Namespace)
		obj.SetName(g.Tracker.Name)
		t.TrackReference(g.TrackedReference, obj)
	}
	// reset tracked requests
	t.reqs = []TrackRequest{}
	return t
}

type mockTracker struct {
	tracker.Tracker
	reqs   []TrackRequest
	scheme *runtime.Scheme
}

var _ tracker.Tracker = &mockTracker{}

// TrackObject tells us that "obj" is tracking changes to the
// referenced object.
func (t *mockTracker) TrackObject(ref client.Object, obj client.Object) error {
	or, err := reference.GetReference(t.scheme, ref)
	if err != nil {
		return err
	}
	gv := schema.FromAPIVersionAndKind(or.APIVersion, or.Kind)
	return t.TrackReference(tracker.Reference{
		APIGroup:  gv.Group,
		Kind:      gv.Kind,
		Namespace: ref.GetNamespace(),
		Name:      ref.GetName(),
	}, obj)
}

// TrackReference tells us that "obj" is tracking changes to the
// referenced object.
func (t *mockTracker) TrackReference(ref tracker.Reference, obj client.Object) error {
	t.reqs = append(t.reqs, TrackRequest{
		Tracker: types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
		TrackedReference: ref,
	})
	return t.Tracker.TrackReference(ref, obj)
}

func (t *mockTracker) getTrackRequests() []TrackRequest {
	return append([]TrackRequest{}, t.reqs...)
}

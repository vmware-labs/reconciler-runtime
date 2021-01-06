/*
Copyright 2019-2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tracker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestWatchingTracker_Ok(t *testing.T) {
	mockTracker := rtesting.CreateTracker()
	watches := []schema.GroupVersionKind{}

	wt := tracker.NewWatchingTracker(mockTracker, func(gvk schema.GroupVersionKind) (context.CancelFunc, error) {
		watches = append(watches, gvk)
		return nil, nil
	})

	gvk1 := schema.GroupVersionKind{
		Group:   "trackedGroup",
		Version: "trackedVersion1",
		Kind:    "trackedKind",
	}
	n1 := types.NamespacedName{
		Namespace: "trackedNamespace",
		Name:      "trackedName",
	}

	tracked1 := tracker.NewKey(gvk1, n1)

	tracking1 := types.NamespacedName{
		Namespace: "trackingnamespace1",
		Name:      "trackingname1",
	}

	// Tracking should delegate to the underlying tracker and start watching
	if err := wt.Track(tracked1, tracking1); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	tr := rtesting.CreateTrackRequest(gvk1.Group, gvk1.Version, gvk1.Kind, n1.Namespace, n1.Name).By(tracking1.Namespace, tracking1.Name)
	if diff := cmp.Diff([]rtesting.TrackRequest{tr}, mockTracker.GetTrackRequests()); diff != "" {
		t.Errorf("incorrect track requests (-expected, +actual): %s", diff)
	}

	if diff := cmp.Diff([]schema.GroupVersionKind{gvk1}, watches); diff != "" {
		t.Errorf("incorrect watches (-expected, +actual): %s", diff)
	}

	// Lookup should delegate to the underlying tracker
	if diff := cmp.Diff([]types.NamespacedName{tracking1}, wt.Lookup(tracked1)); diff != "" {
		t.Errorf("incorrect Lookup result (-expected, +actual): %s", diff)
	}

	// Repeating the track request should delegate to the underlying tracker, but produce no new watches
	if err := wt.Track(tracked1, tracking1); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if diff := cmp.Diff([]rtesting.TrackRequest{tr, tr}, mockTracker.GetTrackRequests()); diff != "" {
		t.Errorf("incorrect track requests (-expected, +actual): %s", diff)
	}
	if diff := cmp.Diff([]schema.GroupVersionKind{gvk1}, watches); diff != "" {
		t.Errorf("incorrect watches (-expected, +actual): %s", diff)
	}

	// A new track request with GVK differing only in the tracked version should delegate to the underlying tracker, but produce no new watches
	gvk2 := gvk1
	gvk2.Version = "trackedVersion2"
	n2 := types.NamespacedName{
		Namespace: "trackedNamespace2",
		Name:      "trackedName2",
	}
	tracked2 := tracker.NewKey(gvk2, n2)
	tracking2 := types.NamespacedName{
		Namespace: "trackingnamespace2",
		Name:      "trackingname2",
	}
	if err := wt.Track(tracked2, tracking2); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	tr2 := rtesting.CreateTrackRequest(gvk2.Group, gvk2.Version, gvk2.Kind, n2.Namespace, n2.Name).By(tracking2.Namespace, tracking2.Name)

	if diff := cmp.Diff([]rtesting.TrackRequest{tr, tr, tr2}, mockTracker.GetTrackRequests()); diff != "" {
		t.Errorf("incorrect track requests (-expected, +actual): %s", diff)
	}
	if diff := cmp.Diff([]schema.GroupVersionKind{gvk1}, watches); diff != "" {
		t.Errorf("incorrect watches (-expected, +actual): %s", diff)
	}
}

func TestWatchingTracker_WatchError(t *testing.T) {
	mockTracker := rtesting.CreateTracker()
	watches := []schema.GroupVersionKind{}

	wt := tracker.NewWatchingTracker(mockTracker, func(gvk schema.GroupVersionKind) (context.CancelFunc, error) {
		watches = append(watches, gvk)
		return nil, errors.New("failed")
	})

	gvk1 := schema.GroupVersionKind{
		Group:   "trackedGroup",
		Version: "trackedVersion1",
		Kind:    "trackedKind",
	}
	n1 := types.NamespacedName{
		Namespace: "trackedNamespace",
		Name:      "trackedName",
	}

	tracked1 := tracker.NewKey(gvk1, n1)

	tracking1 := types.NamespacedName{
		Namespace: "trackingnamespace1",
		Name:      "trackingname1",
	}

	if err := wt.Track(tracked1, tracking1); err.Error() != "failed" {
		t.Errorf("unexpected error %v", err)
	}

	if diff := cmp.Diff([]rtesting.TrackRequest{}, mockTracker.GetTrackRequests()); diff != "" {
		t.Errorf("incorrect track requests (-expected, +actual): %s", diff)
	}

	if diff := cmp.Diff([]schema.GroupVersionKind{gvk1}, watches); diff != "" {
		t.Errorf("incorrect watches (-expected, +actual): %s", diff)
	}
}

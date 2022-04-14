/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"strings"
	"testing"

	logrtesting "github.com/go-logr/logr/testing"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcilerTestCase holds a single test case of a reconciler test suite.
type ReconcilerTestCase struct {
	// Name is a descriptive name for this test suitable as a first argument to t.Run()
	Name string
	// Focus is true if and only if only this and any other focused tests are to be executed.
	// If one or more tests are focused, the overall test suite will fail.
	Focus bool
	// Skip is true if and only if this test should be skipped.
	Skip bool
	// Metadata contains arbitrary value that are stored with the test case
	Metadata map[string]interface{}

	// inputs

	// Key identifies the object to be reconciled
	Key types.NamespacedName
	// WithReactors installs each ReactionFunc into each fake clientset. ReactionFuncs intercept
	// each call to the clientset providing the ability to mutate the resource or inject an error.
	WithReactors []ReactionFunc
	// GivenObjects build the kubernetes objects which are present at the onset of reconciliation
	GivenObjects []client.Object
	// APIGivenObjects contains objects that are only available via an API reader instead of the normal cache
	APIGivenObjects []client.Object

	// side effects

	// ExpectTracks holds the ordered list of Track calls expected during reconciliation
	ExpectTracks []TrackRequest
	// ExpectEvents holds the ordered list of events recorded during the reconciliation
	ExpectEvents []Event
	// ExpectCreates builds the ordered list of objects expected to be created during reconciliation
	ExpectCreates []client.Object
	// ExpectUpdates builds the ordered list of objects expected to be updated during reconciliation
	ExpectUpdates []client.Object
	// ExpectPatches builds the ordered list of objects expected to be patched during reconciliation
	ExpectPatches []PatchRef
	// ExpectDeletes holds the ordered list of objects expected to be deleted during reconciliation
	ExpectDeletes []DeleteRef
	// ExpectDeleteCollections holds the ordered list of collections expected to be deleted during reconciliation
	ExpectDeleteCollections []DeleteCollectionRef
	// ExpectStatusUpdates builds the ordered list of objects whose status is updated during reconciliation
	ExpectStatusUpdates []client.Object
	// ExpectStatusPatches builds the ordered list of objects whose status is patched during reconciliation
	ExpectStatusPatches []PatchRef

	// outputs

	// ShouldErr is true if and only if reconciliation is expected to return an error
	ShouldErr bool
	// ExpectedResult is compared to the result returned from the reconciler if there was no error
	ExpectedResult controllerruntime.Result
	// Verify provides the reconciliation Result and error for custom assertions
	Verify VerifyFunc

	// lifecycle

	// Prepare is called before the reconciler is executed. It is intended to prepare the broader
	// environment before the specific test case is executed. For example, setting mock expectations.
	Prepare func(t *testing.T) error
	// CleanUp is called after the test case is finished and all defined assertions complete.
	// It is indended to clean up any state created in the Prepare step or during the test
	// execution, or to make assertions for mocks.
	CleanUp func(t *testing.T) error
}

// VerifyFunc is a verification function
type VerifyFunc func(t *testing.T, result controllerruntime.Result, err error)

// ReconcilerTestSuite represents a list of reconciler test cases.
type ReconcilerTestSuite []ReconcilerTestCase

// Deprecated: Use Run instead
// Test executes the test case.
func (tc *ReconcilerTestCase) Test(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	tc.Run(t, scheme, factory)
}

// Run executes the test case.
func (tc *ReconcilerTestCase) Run(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	if tc.Skip {
		t.SkipNow()
	}

	ctx := context.TODO()

	// Record the given objects
	givenObjects := make([]client.Object, 0, len(tc.GivenObjects))
	originalGivenObjects := make([]client.Object, 0, len(tc.GivenObjects))
	for _, f := range tc.GivenObjects {
		object := f.DeepCopyObject().(client.Object)
		givenObjects = append(givenObjects, object.DeepCopyObject().(client.Object))
		originalGivenObjects = append(originalGivenObjects, object.DeepCopyObject().(client.Object))
	}
	apiGivenObjects := make([]client.Object, 0, len(tc.APIGivenObjects))
	for _, f := range tc.APIGivenObjects {
		apiGivenObjects = append(apiGivenObjects, f.DeepCopyObject().(client.Object))
	}

	clientWrapper := NewFakeClient(scheme, givenObjects...)
	for i := range tc.WithReactors {
		// in reverse order since we prepend
		reactor := tc.WithReactors[len(tc.WithReactors)-1-i]
		clientWrapper.PrependReactor("*", "*", reactor)
	}
	apiReader := NewFakeClient(scheme, apiGivenObjects...)
	tracker := createTracker()
	recorder := &eventRecorder{
		events: []Event{},
		scheme: scheme,
	}
	log := logrtesting.NewTestLogger(t)
	c := factory(t, tc, reconcilers.Config{
		Client:    clientWrapper,
		APIReader: apiReader,
		Tracker:   tracker,
		Recorder:  recorder,
		Log:       log,
	})

	if tc.CleanUp != nil {
		defer func() {
			if err := tc.CleanUp(t); err != nil {
				t.Errorf("error during clean up: %s", err)
			}
		}()
	}
	if tc.Prepare != nil {
		if err := tc.Prepare(t); err != nil {
			t.Errorf("error during prepare: %s", err)
		}
	}

	// Run the Reconcile we're testing.
	result, err := c.Reconcile(ctx, reconcile.Request{
		NamespacedName: tc.Key,
	})

	if (err != nil) != tc.ShouldErr {
		t.Errorf("Reconcile() error = %v, ExpectErr %v", err, tc.ShouldErr)
	}
	if err == nil {
		// result is only significant if there wasn't an error
		if diff := cmp.Diff(normalizeResult(tc.ExpectedResult), normalizeResult(result)); diff != "" {
			t.Errorf("Unexpected result (-expected, +actual): %s", diff)
		}
	}

	if tc.Verify != nil {
		tc.Verify(t, result, err)
	}

	// compare tracks
	actualTracks := tracker.getTrackRequests()
	for i, exp := range tc.ExpectTracks {
		if i >= len(actualTracks) {
			t.Errorf("Missing tracking request: %s", exp)
			continue
		}

		if diff := cmp.Diff(exp, actualTracks[i]); diff != "" {
			t.Errorf("Unexpected tracking request(-expected, +actual): %s", diff)
		}
	}
	if actual, exp := len(actualTracks), len(tc.ExpectTracks); actual > exp {
		for _, extra := range actualTracks[exp:] {
			t.Errorf("Extra tracking request: %s", extra)
		}
	}

	// compare events
	actualEvents := recorder.events
	for i, exp := range tc.ExpectEvents {
		if i >= len(actualEvents) {
			t.Errorf("Missing recorded event: %s", exp)
			continue
		}

		if diff := cmp.Diff(exp, actualEvents[i]); diff != "" {
			t.Errorf("Unexpected recorded event(-expected, +actual): %s", diff)
		}
	}
	if actual, exp := len(actualEvents), len(tc.ExpectEvents); actual > exp {
		for _, extra := range actualEvents[exp:] {
			t.Errorf("Extra recorded event: %s", extra)
		}
	}

	// compare create
	CompareActions(t, "create", tc.ExpectCreates, clientWrapper.CreateActions, IgnoreLastTransitionTime, SafeDeployDiff, IgnoreTypeMeta, IgnoreResourceVersion, cmpopts.EquateEmpty())

	// compare update
	CompareActions(t, "update", tc.ExpectUpdates, clientWrapper.UpdateActions, IgnoreLastTransitionTime, SafeDeployDiff, IgnoreTypeMeta, IgnoreResourceVersion, cmpopts.EquateEmpty())

	// compare patches
	for i, exp := range tc.ExpectPatches {
		if i >= len(clientWrapper.PatchActions) {
			t.Errorf("Missing patch: %#v", exp)
			continue
		}
		actual := NewPatchRef(clientWrapper.PatchActions[i])

		if diff := cmp.Diff(exp, actual); diff != "" {
			t.Errorf("Unexpected patch (-expected, +actual): %s", diff)
		}
	}
	if actual, expected := len(clientWrapper.PatchActions), len(tc.ExpectPatches); actual > expected {
		for _, extra := range clientWrapper.PatchActions[expected:] {
			t.Errorf("Extra patch: %#v", extra)
		}
	}

	// compare deletes
	for i, exp := range tc.ExpectDeletes {
		if i >= len(clientWrapper.DeleteActions) {
			t.Errorf("Missing delete: %#v", exp)
			continue
		}
		actual := NewDeleteRef(clientWrapper.DeleteActions[i])

		if diff := cmp.Diff(exp, actual); diff != "" {
			t.Errorf("Unexpected delete (-expected, +actual): %s", diff)
		}
	}
	if actual, expected := len(clientWrapper.DeleteActions), len(tc.ExpectDeletes); actual > expected {
		for _, extra := range clientWrapper.DeleteActions[expected:] {
			t.Errorf("Extra delete: %#v", extra)
		}
	}

	// compare delete collections
	for i, exp := range tc.ExpectDeleteCollections {
		if i >= len(clientWrapper.DeleteCollectionActions) {
			t.Errorf("Missing delete collection: %#v", exp)
			continue
		}
		actual := NewDeleteCollectionRef(clientWrapper.DeleteCollectionActions[i])

		if diff := cmp.Diff(exp, actual); diff != "" {
			t.Errorf("Unexpected delete collection (-expected, +actual): %s", diff)
		}
	}
	if actual, expected := len(clientWrapper.DeleteCollectionActions), len(tc.ExpectDeleteCollections); actual > expected {
		for _, extra := range clientWrapper.DeleteCollectionActions[expected:] {
			t.Errorf("Extra delete collection: %#v", extra)
		}
	}

	// compare status update
	CompareActions(t, "status update", tc.ExpectStatusUpdates, clientWrapper.StatusUpdateActions, statusSubresourceOnly, IgnoreLastTransitionTime, SafeDeployDiff, cmpopts.EquateEmpty())

	// status patch
	for i, exp := range tc.ExpectStatusPatches {
		if i >= len(clientWrapper.StatusPatchActions) {
			t.Errorf("Missing status patch: %#v", exp)
			continue
		}
		actual := NewPatchRef(clientWrapper.StatusPatchActions[i])

		if diff := cmp.Diff(exp, actual); diff != "" {
			t.Errorf("Unexpected status patch (-expected, +actual): %s", diff)
		}
	}
	if actual, expected := len(clientWrapper.StatusPatchActions), len(tc.ExpectStatusPatches); actual > expected {
		for _, extra := range clientWrapper.StatusPatchActions[expected:] {
			t.Errorf("Extra status patch: %#v", extra)
		}
	}

	// Validate the given objects are not mutated by reconciliation
	if diff := cmp.Diff(originalGivenObjects, givenObjects, SafeDeployDiff, IgnoreResourceVersion, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Given objects mutated by test %q (-expected, +actual): %v", tc.Name, diff)
	}
}

func normalizeResult(result controllerruntime.Result) controllerruntime.Result {
	// RequeueAfter implies Requeue, no need to set both
	if result.RequeueAfter != 0 {
		result.Requeue = false
	}
	return result
}

func CompareActions(t *testing.T, actionName string, expectedActionFactories []client.Object, actualActions []objectAction, diffOptions ...cmp.Option) {
	// TODO(scothis) this could be a really good place to play with generics
	t.Helper()
	for i, exp := range expectedActionFactories {
		if i >= len(actualActions) {
			t.Errorf("Missing %s: %#v", actionName, exp.DeepCopyObject())
			continue
		}
		actual := actualActions[i].GetObject()

		if diff := cmp.Diff(exp.DeepCopyObject(), actual, diffOptions...); diff != "" {
			t.Errorf("Unexpected %s (-expected, +actual): %s", actionName, diff)
		}
	}
	if actual, expected := len(actualActions), len(expectedActionFactories); actual > expected {
		for _, extra := range actualActions[expected:] {
			t.Errorf("Extra %s: %#v", actionName, extra)
		}
	}
}

var (
	IgnoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime")
	}, cmp.Ignore())
	IgnoreTypeMeta = cmp.FilterPath(func(p cmp.Path) bool {
		path := p.String()
		return strings.HasSuffix(path, "TypeMeta.APIVersion") || strings.HasSuffix(path, "TypeMeta.Kind")
	}, cmp.Ignore())
	IgnoreResourceVersion = cmp.FilterPath(func(p cmp.Path) bool {
		path := p.String()
		return strings.HasSuffix(path, "ObjectMeta.ResourceVersion")
	}, cmp.Ignore())

	statusSubresourceOnly = cmp.FilterPath(func(p cmp.Path) bool {
		q := p.String()
		return q != "" && !strings.HasPrefix(q, "Status")
	}, cmp.Ignore())

	SafeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)

// Deprecated: Use Run instead
// Test executes the reconciler test suite.
func (ts ReconcilerTestSuite) Test(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	ts.Run(t, scheme, factory)
}

// Run executes the reconciler test suite.
func (ts ReconcilerTestSuite) Run(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	focused := ReconcilerTestSuite{}
	for _, test := range ts {
		if test.Focus {
			focused = append(focused, test)
			break
		}
	}
	testsToExecute := ts
	if len(focused) > 0 {
		testsToExecute = focused
	}
	for _, test := range testsToExecute {
		t.Run(test.Name, func(t *testing.T) {
			t.Helper()
			test.Test(t, scheme, factory)
		})
	}
	if len(focused) > 0 {
		t.Errorf("%d tests out of %d are still focused, so the test suite fails", len(focused), len(ts))
	}
}

// ReconcilerFactory returns a Reconciler.Interface to perform reconciliation of a test case,
// ActionRecorderList/EventList to capture k8s actions/events produced during reconciliation
// and FakeStatsReporter to capture stats.
type ReconcilerFactory func(t *testing.T, rtc *ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler

type PatchRef struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
	PatchType types.PatchType
	Patch     []byte
}

func NewPatchRef(action PatchAction) PatchRef {
	return PatchRef{
		Group:     action.GetResource().Group,
		Kind:      action.GetResource().Resource,
		Namespace: action.GetNamespace(),
		Name:      action.GetName(),
		PatchType: action.GetPatchType(),
		Patch:     action.GetPatch(),
	}
}

type DeleteRef struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

func NewDeleteRef(action DeleteAction) DeleteRef {
	return DeleteRef{
		Group:     action.GetResource().Group,
		Kind:      action.GetResource().Resource,
		Namespace: action.GetNamespace(),
		Name:      action.GetName(),
	}
}

type DeleteCollectionRef struct {
	Group     string
	Kind      string
	Namespace string
	Labels    labels.Selector
	Fields    fields.Selector
}

func NewDeleteCollectionRef(action DeleteCollectionAction) DeleteCollectionRef {
	return DeleteCollectionRef{
		Group:     action.GetResource().Group,
		Kind:      action.GetResource().Resource,
		Namespace: action.GetNamespace(),
		Labels:    action.GetListRestrictions().Labels,
		Fields:    action.GetListRestrictions().Fields,
	}
}

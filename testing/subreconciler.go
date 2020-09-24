/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SubReconcilerTestCase holds a single testcase of a sub reconciler test.
type SubReconcilerTestCase struct {
	// Name is a descriptive name for this test suitable as a first argument to t.Run()
	Name string
	// Focus is true if and only if only this and any other focussed tests are to be executed.
	// If one or more tests are focussed, the overall test suite will fail.
	Focus bool
	// Skip is true if and only if this test should be skipped.
	Skip bool
	// Metadata contains arbitrary value that are stored with the test case
	Metadata map[string]interface{}

	// inputs

	// Parent is the initial object passed to the sub reconciler
	Parent Factory
	// GivenStashedValues adds these items to the stash passed into the reconciler. Factories are resolved to their object.
	GivenStashedValues map[reconcilers.StashKey]interface{}
	// WithReactors installs each ReactionFunc into each fake clientset. ReactionFuncs intercept
	// each call to the clientset providing the ability to mutate the resource or inject an error.
	WithReactors []ReactionFunc
	// GivenObjects build the kubernetes objects which are present at the onset of reconciliation
	GivenObjects []Factory
	// APIGivenObjects contains objects that are only available via an API reader instead of the normal cache
	APIGivenObjects []Factory

	// side effects

	// ExpectParent is the expected parent as mutated after the sub reconciler, or nil if no modification
	ExpectParent Factory
	// ExpectStashedValues ensures each value is stashed. Values in the stash that are not expected are ignored. Factories are resolved to their object.
	ExpectStashedValues map[reconcilers.StashKey]interface{}
	// ExpectTracks holds the ordered list of Track calls expected during reconciliation
	ExpectTracks []TrackRequest
	// ExpectEvents holds the ordered list of events recorded during the reconciliation
	ExpectEvents []Event
	// ExpectCreates builds the ordered list of objects expected to be created during reconciliation
	ExpectCreates []Factory
	// ExpectUpdates builds the ordered list of objects expected to be updated during reconciliation
	ExpectUpdates []Factory
	// ExpectDeletes holds the ordered list of objects expected to be deleted during reconciliation
	ExpectDeletes []DeleteRef

	// outputs

	// ShouldErr is true if and only if reconciliation is expected to return an error
	ShouldErr bool
	// ShouldPanic is true if and only if reconciliation is expected to panic. A panic should only be
	// used to indicate the reconciler is misconfigured.
	ShouldPanic bool
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

// SubReconcilerTestSuite represents a list of subreconciler test cases.
type SubReconcilerTestSuite []SubReconcilerTestCase

// Test executes the test case.
func (tc *SubReconcilerTestCase) Test(t *testing.T, scheme *runtime.Scheme, factory SubReconcilerFactory) {
	t.Helper()
	if tc.Skip {
		t.SkipNow()
	}

	// Record the given objects
	givenObjects := make([]runtime.Object, 0, len(tc.GivenObjects))
	originalGivenObjects := make([]runtime.Object, 0, len(tc.GivenObjects))
	for _, f := range tc.GivenObjects {
		object := f.CreateObject()
		givenObjects = append(givenObjects, object.DeepCopyObject())
		originalGivenObjects = append(originalGivenObjects, object.DeepCopyObject())
	}
	apiGivenObjects := make([]runtime.Object, 0, len(tc.APIGivenObjects))
	for _, f := range tc.APIGivenObjects {
		apiGivenObjects = append(apiGivenObjects, f.CreateObject())
	}

	clientWrapper := newClientWrapperWithScheme(scheme, givenObjects...)
	for i := range tc.WithReactors {
		// in reverse order since we prepend
		reactor := tc.WithReactors[len(tc.WithReactors)-1-i]
		clientWrapper.PrependReactor("*", "*", reactor)
	}
	apiReader := newClientWrapperWithScheme(scheme, apiGivenObjects...)
	tracker := createTracker()
	recorder := &eventRecorder{
		events: []Event{},
		scheme: scheme,
	}
	log := TestLogger(t)
	c := factory(t, tc, reconcilers.Config{
		DuckClient: clientWrapper,
		APIReader:  apiReader,
		Tracker:    tracker,
		Recorder:   recorder,
		Scheme:     scheme,
		Log:        log,
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

	ctx := reconcilers.WithStash(context.Background())
	for k, v := range tc.GivenStashedValues {
		if f, ok := v.(Factory); ok {
			v = f.CreateObject()
		}
		reconcilers.StashValue(ctx, k, v)
	}

	parent := tc.Parent.CreateObject()
	ctx = reconcilers.StashParentType(ctx, parent.DeepCopyObject())

	// Run the Reconcile we're testing.
	result, err := func(ctx context.Context, parent apis.Object) (reconcile.Result, error) {
		if tc.ShouldPanic {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected Reconcile() to panic")
				}
			}()
		}

		return c.Reconcile(ctx, parent)
	}(ctx, parent)

	if (err != nil) != tc.ShouldErr {
		t.Errorf("Reconcile() error = %v, ExpectErr %v", err, tc.ShouldErr)
	}
	if err == nil {
		// result is only significant if there wasn't an error
		if diff := cmp.Diff(tc.ExpectedResult, result); diff != "" {
			t.Errorf("Unexpected result (-expected, +actual): %s", diff)
		}
	}

	if tc.Verify != nil {
		tc.Verify(t, result, err)
	}

	expectedParent := tc.Parent.CreateObject()
	if tc.ExpectParent != nil {
		expectedParent = tc.ExpectParent.CreateObject()
	}
	if diff := cmp.Diff(expectedParent, parent, IgnoreLastTransitionTime, safeDeployDiff, ignoreTypeMeta, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Unexpected parent mutations(-expected, +actual): %s", diff)
	}

	for key, expected := range tc.ExpectStashedValues {
		if f, ok := expected.(Factory); ok {
			expected = f.CreateObject()
		}
		actual := reconcilers.RetrieveValue(ctx, key)
		if diff := cmp.Diff(expected, actual, IgnoreLastTransitionTime, safeDeployDiff, ignoreTypeMeta, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected stash value %q (-expected, +actual): %s", key, diff)
		}
	}

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

	compareActions(t, "create", tc.ExpectCreates, clientWrapper.createActions, IgnoreLastTransitionTime, safeDeployDiff, ignoreTypeMeta, cmpopts.EquateEmpty())
	compareActions(t, "update", tc.ExpectUpdates, clientWrapper.updateActions, IgnoreLastTransitionTime, safeDeployDiff, ignoreTypeMeta, cmpopts.EquateEmpty())

	for i, exp := range tc.ExpectDeletes {
		if i >= len(clientWrapper.deleteActions) {
			t.Errorf("Missing delete: %#v", exp)
			continue
		}
		actual := NewDeleteRef(clientWrapper.deleteActions[i])

		if diff := cmp.Diff(exp, actual); diff != "" {
			t.Errorf("Unexpected delete (-expected, +actual): %s", diff)
		}
	}
	if actual, expected := len(clientWrapper.deleteActions), len(tc.ExpectDeletes); actual > expected {
		for _, extra := range clientWrapper.deleteActions[expected:] {
			t.Errorf("Extra delete: %#v", extra)
		}
	}

	// Validate the given objects are not mutated by reconciliation
	if diff := cmp.Diff(originalGivenObjects, givenObjects, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Given objects mutated by test %s (-expected, +actual): %v", tc.Name, diff)
	}
}

// Test executes the subreconciler test suite.
func (tb SubReconcilerTestSuite) Test(t *testing.T, scheme *runtime.Scheme, factory SubReconcilerFactory) {
	t.Helper()
	focussed := SubReconcilerTestSuite{}
	for _, test := range tb {
		if test.Focus {
			focussed = append(focussed, test)
			break
		}
	}
	testsToExecute := tb
	if len(focussed) > 0 {
		testsToExecute = focussed
	}
	for _, test := range testsToExecute {
		t.Run(test.Name, func(t *testing.T) {
			t.Helper()
			test.Test(t, scheme, factory)
		})
	}
	if len(focussed) > 0 {
		t.Errorf("%d tests out of %d are still focussed, so the test suite fails", len(focussed), len(tb))
	}
}

// SubReconcilerFactory returns a Reconciler.Interface to perform reconciliation of a test case,
// ActionRecorderList/EventList to capture k8s actions/events produced during reconciliation
// and FakeStatsReporter to capture stats.
type SubReconcilerFactory func(t *testing.T, rtc *SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler

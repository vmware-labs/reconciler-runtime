/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"k8s.io/apimachinery/pkg/api/resource"
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
	// Focus is true if and only if only this and any other focussed tests are to be executed.
	// If one or more tests are focussed, the overall test suite will fail.
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
	GivenObjects []Factory
	// APIGivenObjects contains objects that are only available via an API reader instead of the normal cache
	APIGivenObjects []Factory

	// side effects

	// ExpectTracks holds the ordered list of Track calls expected during reconciliation
	ExpectTracks []TrackRequest
	// ExpectEvents holds the ordered list of events recorded during the reconciliation
	ExpectEvents []Event
	// ExpectCreates builds the ordered list of objects expected to be created during reconciliation
	ExpectCreates []Factory
	// ExpectUpdates builds the ordered list of objects expected to be updated during reconciliation
	ExpectUpdates []Factory
	// ExpectPatches holds the ordered list of objects expected to be patched during reconciliation
	ExpectPatches []PatchRef
	// ExpectDeletes holds the ordered list of objects expected to be deleted during reconciliation
	ExpectDeletes []DeleteRef
	// ExpectStatusUpdates builds the ordered list of objects whose status is updated during reconciliation
	ExpectStatusUpdates []Factory

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

// Test executes the test case.
func (tc *ReconcilerTestCase) Test(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	if tc.Skip {
		t.SkipNow()
	}

	ctx := context.TODO()

	// Record the given objects
	givenObjects := make([]client.Object, 0, len(tc.GivenObjects))
	originalGivenObjects := make([]client.Object, 0, len(tc.GivenObjects))
	for _, f := range tc.GivenObjects {
		object := f.CreateObject()
		givenObjects = append(givenObjects, object.DeepCopyObject().(client.Object))
		originalGivenObjects = append(originalGivenObjects, object.DeepCopyObject().(client.Object))
	}
	apiGivenObjects := make([]client.Object, 0, len(tc.APIGivenObjects))
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
	tracker := CreateTracker()
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

	actualTracks := tracker.GetTrackRequests()
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

	compareActions(t, "create", tc.ExpectCreates, clientWrapper.createActions, IgnoreLastTransitionTime, safeDeployDiff, ignoreTypeMeta, ignoreResourceVersion, cmpopts.EquateEmpty())
	compareActions(t, "update", tc.ExpectUpdates, clientWrapper.updateActions, IgnoreLastTransitionTime, safeDeployDiff, ignoreTypeMeta, ignoreResourceVersion, cmpopts.EquateEmpty())

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

	for i, exp := range tc.ExpectPatches {
		if i >= len(clientWrapper.patchActions) {
			t.Errorf("Missing patch: %#v", exp)
			continue
		}
		actual := NewPatchRef(clientWrapper.patchActions[i])

		if diff := cmp.Diff(exp, actual); diff != "" {
			t.Errorf("Unexpected patch (-expected, +actual): %s", diff)
		}
	}
	if actual, expected := len(clientWrapper.patchActions), len(tc.ExpectDeletes); actual > expected {
		for _, extra := range clientWrapper.patchActions[expected:] {
			t.Errorf("Extra patch: %#v", extra)
		}
	}

	compareActions(t, "status update", tc.ExpectStatusUpdates, clientWrapper.statusUpdateActions, statusSubresourceOnly, IgnoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty())

	// Validate the given objects are not mutated by reconciliation
	if diff := cmp.Diff(originalGivenObjects, givenObjects, safeDeployDiff, ignoreResourceVersion, cmpopts.EquateEmpty()); diff != "" {
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

func compareActions(t *testing.T, actionName string, expectedActionFactories []Factory, actualActions []objectAction, diffOptions ...cmp.Option) {
	t.Helper()
	for i, exp := range expectedActionFactories {
		if i >= len(actualActions) {
			t.Errorf("Missing %s: %#v", actionName, exp.CreateObject())
			continue
		}
		actual := actualActions[i].GetObject()

		if diff := cmp.Diff(exp.CreateObject(), actual, diffOptions...); diff != "" {
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
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())
	ignoreTypeMeta = cmp.FilterPath(func(p cmp.Path) bool {
		path := p.String()
		return strings.HasSuffix(path, "TypeMeta.APIVersion") || strings.HasSuffix(path, "TypeMeta.Kind")
	}, cmp.Ignore())
	ignoreResourceVersion = cmp.FilterPath(func(p cmp.Path) bool {
		path := p.String()
		return strings.HasSuffix(path, "ObjectMeta.ResourceVersion")
	}, cmp.Ignore())

	statusSubresourceOnly = cmp.FilterPath(func(p cmp.Path) bool {
		q := p.String()
		return q != "" && !strings.HasPrefix(q, "Status")
	}, cmp.Ignore())

	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)

// Test executes the reconciler test suite.
func (tb ReconcilerTestSuite) Test(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	focussed := ReconcilerTestSuite{}
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

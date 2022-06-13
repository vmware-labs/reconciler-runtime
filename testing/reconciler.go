/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
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
	// Metadata contains arbitrary values that are stored with the test case
	Metadata map[string]interface{}

	// inputs

	// Deprecated use Request
	Key types.NamespacedName
	// Request identifies the object to be reconciled
	Request controllerruntime.Request
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

	// AdditionalConfigs holds ExceptConfigs that are available to the test case and will have
	// their expectations checked again the observed config interactions. The key in this map is
	// set as the ExpectConfig's name.
	AdditionalConfigs map[string]ExpectConfig

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

// ReconcilerTests represents a map of reconciler test cases. The map key is the name of each test
// case. Test cases are executed in random order.
type ReconcilerTests map[string]ReconcilerTestCase

// Run executes the test cases.
func (rt ReconcilerTests) Run(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	rts := ReconcilerTestSuite{}
	for name, rtc := range rt {
		rtc.Name = name
		rts = append(rts, rtc)
	}
	rts.Run(t, scheme, factory)
}

// ReconcilerTestSuite represents a list of reconciler test cases. The test cases are executed in order.
type ReconcilerTestSuite []ReconcilerTestCase

// Run executes the test case.
func (tc *ReconcilerTestCase) Run(t *testing.T, scheme *runtime.Scheme, factory ReconcilerFactory) {
	t.Helper()
	if tc.Skip {
		t.SkipNow()
	}

	ctx := context.Background()

	expectConfig := &ExpectConfig{
		Name:                    "default",
		Scheme:                  scheme,
		GivenObjects:            tc.GivenObjects,
		APIGivenObjects:         tc.APIGivenObjects,
		WithReactors:            tc.WithReactors,
		ExpectTracks:            tc.ExpectTracks,
		ExpectEvents:            tc.ExpectEvents,
		ExpectCreates:           tc.ExpectCreates,
		ExpectUpdates:           tc.ExpectUpdates,
		ExpectPatches:           tc.ExpectPatches,
		ExpectDeletes:           tc.ExpectDeletes,
		ExpectDeleteCollections: tc.ExpectDeleteCollections,
		ExpectStatusUpdates:     tc.ExpectStatusUpdates,
		ExpectStatusPatches:     tc.ExpectStatusPatches,
	}

	configs := make(map[string]reconcilers.Config, len(tc.AdditionalConfigs))
	for k, v := range tc.AdditionalConfigs {
		v.Name = k
		configs[k] = v.Config()
	}
	ctx = reconcilers.StashAdditionalConfigs(ctx, configs)

	r := factory(t, tc, expectConfig.Config())

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
	request := tc.Request
	if request == (controllerruntime.Request{}) {
		request.NamespacedName = tc.Key
	}
	result, err := r.Reconcile(ctx, request)

	if (err != nil) != tc.ShouldErr {
		t.Errorf("Reconcile() error = %v, ShouldErr %v", err, tc.ShouldErr)
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

	expectConfig.AssertExpectations(t)
	for _, config := range tc.AdditionalConfigs {
		config.AssertExpectations(t)
	}
}

func normalizeResult(result controllerruntime.Result) controllerruntime.Result {
	// RequeueAfter implies Requeue, no need to set both
	if result.RequeueAfter != 0 {
		result.Requeue = false
	}
	return result
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
			test.Run(t, scheme, factory)
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

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtime "github.com/vmware-labs/reconciler-runtime/time"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	// Request identifies the object to be reconciled
	Request reconcilers.Request
	// WithReactors installs each ReactionFunc into each fake clientset. ReactionFuncs intercept
	// each call to the clientset providing the ability to mutate the resource or inject an error.
	WithReactors []ReactionFunc
	// WithClientBuilder allows a test to modify the fake client initialization.
	WithClientBuilder func(*fake.ClientBuilder) *fake.ClientBuilder
	// StatusSubResourceTypes is a set of object types that support the status sub-resource. For
	// these types, the only way to modify the resource's status is update or patch the status
	// sub-resource. Patching or updating the main resource will not mutated the status field.
	// Built-in Kubernetes types (e.g. Pod, Deployment, etc) are already accounted for and do not
	// need to be listed.
	//
	// Interacting with a status sub-resource for a type not enumerated as having a status
	// sub-resource will return a not found error.
	StatusSubResourceTypes []client.Object
	// GivenObjects build the kubernetes objects which are present at the onset of reconciliation
	GivenObjects []client.Object
	// APIGivenObjects contains objects that are only available via an API reader instead of the normal cache
	APIGivenObjects []client.Object
	// GivenTracks provide a set of tracked resources to seed the tracker with
	GivenTracks []TrackRequest

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
	ExpectedResult reconcilers.Result
	// Verify provides the reconciliation Result and error for custom assertions
	Verify VerifyFunc

	// lifecycle

	// Prepare is called before the reconciler is executed. It is intended to prepare the broader
	// environment before the specific test case is executed. For example, setting mock
	// expectations, or adding values to the context.
	Prepare func(t *testing.T, ctx context.Context, tc *ReconcilerTestCase) (context.Context, error)
	// CleanUp is called after the test case is finished and all defined assertions complete.
	// It is intended to clean up any state created in the Prepare step or during the test
	// execution, or to make assertions for mocks.
	CleanUp func(t *testing.T, ctx context.Context, tc *ReconcilerTestCase) error
	// Now is the time the test should run as, defaults to the current time. This value can be used
	// by reconcilers via the reconcilers.RetireveNow(ctx) method.
	Now time.Time
}

// VerifyFunc is a verification function for a reconciler's result
type VerifyFunc func(t *testing.T, result reconcilers.Result, err error)

// VerifyStashedValueFunc is a verification function for the entries in the stash
type VerifyStashedValueFunc func(t *testing.T, key reconcilers.StashKey, expected, actual interface{})

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
	if tc.Now == (time.Time{}) {
		tc.Now = time.Now()
	}
	ctx = rtime.StashNow(ctx, tc.Now)
	ctx = logr.NewContext(ctx, testr.New(t))
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	if tc.Metadata == nil {
		tc.Metadata = map[string]interface{}{}
	}

	if tc.Prepare != nil {
		var err error
		if ctx, err = tc.Prepare(t, ctx, tc); err != nil {
			t.Fatalf("error during prepare: %s", err)
		}
	}
	if tc.CleanUp != nil {
		defer func() {
			if err := tc.CleanUp(t, ctx, tc); err != nil {
				t.Errorf("error during clean up: %s", err)
			}
		}()
	}

	expectConfig := &ExpectConfig{
		Name:                    "default",
		Scheme:                  scheme,
		StatusSubResourceTypes:  tc.StatusSubResourceTypes,
		GivenObjects:            tc.GivenObjects,
		APIGivenObjects:         tc.APIGivenObjects,
		WithClientBuilder:       tc.WithClientBuilder,
		WithReactors:            tc.WithReactors,
		GivenTracks:             tc.GivenTracks,
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

	// Run the Reconcile we're testing.
	result, err := r.Reconcile(ctx, tc.Request)

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

func normalizeResult(result reconcilers.Result) reconcilers.Result {
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

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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vmware-labs/reconciler-runtime/duck"
	"github.com/vmware-labs/reconciler-runtime/internal"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtime "github.com/vmware-labs/reconciler-runtime/time"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// SubReconcilerTestCase holds a single testcase of a sub reconciler test.
type SubReconcilerTestCase[Type client.Object] struct {
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

	// Resource is the initial object passed to the sub reconciler
	Resource Type
	// GivenStashedValues adds these items to the stash passed into the reconciler. Factories are resolved to their object.
	GivenStashedValues map[reconcilers.StashKey]interface{}
	// WithClientBuilder allows a test to modify the fake client initialization.
	WithClientBuilder func(*fake.ClientBuilder) *fake.ClientBuilder
	// WithReactors installs each ReactionFunc into each fake clientset. ReactionFuncs intercept
	// each call to the clientset providing the ability to mutate the resource or inject an error.
	WithReactors []ReactionFunc
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

	// ExpectResource is the expected reconciled resource as mutated after the sub reconciler, or nil if no modification
	ExpectResource Type
	// ExpectStashedValues ensures each value is stashed. Values in the stash that are not expected are ignored. Factories are resolved to their object.
	ExpectStashedValues map[reconcilers.StashKey]interface{}
	// VerifyStashedValue is an optional, custom verification function for stashed values
	VerifyStashedValue VerifyStashedValueFunc
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

	// AdditionalConfigs holds configs that are available to the test case and will have their
	// expectations checked again the observed config interactions. The key in this map is set as
	// the ExpectConfig's name.
	AdditionalConfigs map[string]ExpectConfig

	// outputs

	// ShouldErr is true if and only if reconciliation is expected to return an error
	ShouldErr bool
	// ShouldPanic is true if and only if reconciliation is expected to panic. A panic should only be
	// used to indicate the reconciler is misconfigured.
	ShouldPanic bool
	// ExpectedResult is compared to the result returned from the reconciler if there was no error
	ExpectedResult reconcilers.Result
	// Verify provides the reconciliation Result and error for custom assertions
	Verify VerifyFunc

	// lifecycle

	// Prepare is called before the reconciler is executed. It is intended to prepare the broader
	// environment before the specific test case is executed. For example, setting mock
	// expectations, or adding values to the context,
	Prepare func(t *testing.T, ctx context.Context, tc *SubReconcilerTestCase[Type]) (context.Context, error)
	// CleanUp is called after the test case is finished and all defined assertions complete.
	// It is intended to clean up any state created in the Prepare step or during the test
	// execution, or to make assertions for mocks.
	CleanUp func(t *testing.T, ctx context.Context, tc *SubReconcilerTestCase[Type]) error
	// Now is the time the test should run as, defaults to the current time. This value can be used
	// by reconcilers via the reconcilers.RetireveNow(ctx) method.
	Now time.Time
}

// SubReconcilerTests represents a map of reconciler test cases. The map key is the name of each
// test case.  Test cases are executed in random order.
type SubReconcilerTests[Type client.Object] map[string]SubReconcilerTestCase[Type]

// Run executes the test cases.
func (rt SubReconcilerTests[T]) Run(t *testing.T, scheme *runtime.Scheme, factory SubReconcilerFactory[T]) {
	t.Helper()
	rts := SubReconcilerTestSuite[T]{}
	for name, rtc := range rt {
		rtc.Name = name
		rts = append(rts, rtc)
	}
	rts.Run(t, scheme, factory)
}

// SubReconcilerTestSuite represents a list of subreconciler test cases. The test cases are
// executed in order.
type SubReconcilerTestSuite[Type client.Object] []SubReconcilerTestCase[Type]

// Run executes the test case.
func (tc *SubReconcilerTestCase[T]) Run(t *testing.T, scheme *runtime.Scheme, factory SubReconcilerFactory[T]) {
	t.Helper()
	if tc.Skip {
		t.SkipNow()
	}

	ctx := reconcilers.WithStash(context.Background())
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

	// Set func for verifying stashed values
	if tc.VerifyStashedValue == nil {
		tc.VerifyStashedValue = func(t *testing.T, key reconcilers.StashKey, expected, actual interface{}) {
			if diff := cmp.Diff(expected, actual, reconcilers.IgnoreAllUnexported, IgnoreLastTransitionTime, IgnoreTypeMeta, IgnoreCreationTimestamp, IgnoreResourceVersion, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ExpectStashedValues[%q] differs (%s, %s): %s", key, DiffRemovedColor.Sprint("-expected"), DiffAddedColor.Sprint("+actual"), ColorizeDiff(diff))
			}
		}
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

	var givenResource client.Object = tc.Resource
	if duck.IsDuck(givenResource, scheme) {
		// convert the given resource duck to Unstructured so that it can be created on the fake client
		uobj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.Resource)
		if err != nil {
			t.Fatalf("unable to convert Resource to Unstructured: %s", err)
		}
		givenResource = &unstructured.Unstructured{Object: uobj}
	}

	expectConfig := &ExpectConfig{
		Name:                    "default",
		Scheme:                  scheme,
		StatusSubResourceTypes:  tc.StatusSubResourceTypes,
		GivenObjects:            append(tc.GivenObjects, givenResource),
		APIGivenObjects:         append(tc.APIGivenObjects, givenResource),
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
	}
	c := expectConfig.Config()

	r := factory(t, tc, c)

	for k, v := range tc.GivenStashedValues {
		if f, ok := v.(runtime.Object); ok {
			v = f.DeepCopyObject()
		}
		reconcilers.StashValue(ctx, k, v)
	}

	ctx = reconcilers.StashConfig(ctx, c)
	ctx = reconcilers.StashOriginalConfig(ctx, c)

	resource := tc.Resource.DeepCopyObject().(T)
	if resource.GetResourceVersion() == "" {
		// this value is also set by the test client when resource are added as givens
		resource.SetResourceVersion("999")
	}
	ctx = reconcilers.StashRequest(ctx, reconcilers.Request{
		NamespacedName: types.NamespacedName{Namespace: resource.GetNamespace(), Name: resource.GetName()},
	})
	ctx = reconcilers.StashOriginalResourceType(ctx, resource.DeepCopyObject().(T))
	ctx = reconcilers.StashResourceType(ctx, resource.DeepCopyObject().(T))

	configs := make(map[string]reconcilers.Config, len(tc.AdditionalConfigs))
	for k, v := range tc.AdditionalConfigs {
		v.Name = k
		configs[k] = v.Config()
	}
	ctx = reconcilers.StashAdditionalConfigs(ctx, configs)

	// Run the Reconcile we're testing.
	result, err := func(ctx context.Context, resource T) (reconcilers.Result, error) {
		if tc.ShouldPanic {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected Reconcile() to panic")
				}
			}()
		}

		return r.Reconcile(ctx, resource)
	}(ctx, resource)

	if (err != nil) != tc.ShouldErr {
		t.Errorf("Reconcile() error = %v, ShouldErr %v", err, tc.ShouldErr)
	}
	if err == nil {
		// result is only significant if there wasn't an error
		if diff := cmp.Diff(normalizeResult(tc.ExpectedResult), normalizeResult(result)); diff != "" {
			t.Errorf("ExpectedResult differs (%s, %s): %s", DiffRemovedColor.Sprint("-expected"), DiffAddedColor.Sprint("+actual"), ColorizeDiff(diff))
		}
	}

	if tc.Verify != nil {
		tc.Verify(t, result, err)
	}

	// compare resource
	expectedResource := tc.Resource.DeepCopyObject().(T)
	if internal.IsNilable(tc.ExpectResource) && !internal.IsNil(tc.ExpectResource) {
		expectedResource = tc.ExpectResource.DeepCopyObject().(T)
	}
	if expectedResource.GetResourceVersion() == "" {
		// mirror defaulting of the resource
		expectedResource.SetResourceVersion("999")
	}
	if diff := cmp.Diff(expectedResource, resource, reconcilers.IgnoreAllUnexported, IgnoreLastTransitionTime, IgnoreTypeMeta, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("ExpectResource differs (%s, %s): %s", DiffRemovedColor.Sprint("-expected"), DiffAddedColor.Sprint("+actual"), ColorizeDiff(diff))
	}

	// compare stashed
	for key, expected := range tc.ExpectStashedValues {
		if f, ok := expected.(runtime.Object); ok {
			expected = f.DeepCopyObject()
		}
		actual := reconcilers.RetrieveValue(ctx, key)
		tc.VerifyStashedValue(t, key, expected, actual)
	}

	expectConfig.AssertExpectations(t)
	for _, config := range tc.AdditionalConfigs {
		config.AssertExpectations(t)
	}
}

// Run executes the subreconciler test suite.
func (ts SubReconcilerTestSuite[T]) Run(t *testing.T, scheme *runtime.Scheme, factory SubReconcilerFactory[T]) {
	t.Helper()
	focused := SubReconcilerTestSuite[T]{}
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

// SubReconcilerFactory returns a Reconciler.Interface to perform reconciliation of a test case,
// ActionRecorderList/EventList to capture k8s actions/events produced during reconciliation
// and FakeStatsReporter to capture stats.
type SubReconcilerFactory[Type client.Object] func(t *testing.T, rtc *SubReconcilerTestCase[Type], c reconcilers.Config) reconcilers.SubReconciler[Type]

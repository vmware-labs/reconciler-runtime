/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/cobra"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-labs/reconciler-runtime/cli"
)

// CommandTestSuite provides a declarative model for testing interactions with Kubernetes resources
// via Cobra commands.
//
// A fake controller-runtime client is used to stub calls to the Kubernetes API server. GivenObjects populate a
// local cache for the client to respond to get and list operations (update and delete will error
// if the object does not exist and create operations will error if the resource does exist).
//
// ExpectCreates and ExpectUpdates each contain objects that are compared directly to resources
// received by the client. ExpectDeletes and ExpectDeleteCollections contain references to the
// resources impacted by the call since these calls do not receive an object.
//
// Errors can be injected into API calls by reactor functions specified in WithReactors. A
// ReactionFunc is able to intercept each client operation to observe or mutate the request or
// response.
//
// ShouldError must correctly reflect whether the command is expected to return an error,
// otherwise the testcase will fail. Custom assertions based on the content of the error object
// and the console output from the command are available with the Verify callback.
//
// Advanced state may be configured before and after each record by the Prepare and CleanUp
// callbacks respectively.
type CommandTestSuite []CommandTestCase

// CommandTestCase is a single test case within a CommandTable. All state and assertions are
// defined within the record.
type CommandTestCase struct {

	// Name is used to identify the record in the test results. A sub-test is created for each
	// record with this name.
	Name string
	// Skip suppresses the execution of this test record.
	Skip bool
	// Focus executes this record skipping all unfocused records. The containing test will fail to
	// prevent accidental check-in.
	Focus bool
	// Metadata contains arbitrary value that are stored with the test case
	Metadata map[string]interface{}

	// environment

	// Config is passed into the command factory. Mosts tests should not need to set this field.
	// If not specified, a default Config is created with a FakeClient. The Config's client will
	// always be replaced with a FakeClient configured with the given objects and reactors to
	// intercept all calls to the fake client for comparison with the expected operations.
	Config *cli.Config

	// GivenObjects represents resources that would already exist within Kubernetes. These
	// resources are passed directly to the fake client.
	GivenObjects []Factory
	// WithReactors installs each ReactionFunc into each fake client. ReactionFuncs intercept
	// each call to the client providing the ability to mutate the resource or inject an error.
	WithReactors []ReactionFunc
	// ExecHelper is a test case that will intercept exec calls receiving their arguments and
	// environment. The helper is able to control stdio and the exit code of the process. Test
	// cases that need to orchestrate multiple exec calls within a single test should instead use
	// a mock.
	//
	// The value of ExecHelper must map to a test function in the same package taking the form
	// `fmt.Sprintf("TestHelperProcess_%s", ExecHelper)`. The test function should distinguish
	// between test exec invocations and vanilla test calls by the `GO_WANT_HELPER_PROCESS` env.
	//
	// ```
	// func TestHelperProcess_Example(t *testing.T) {
	//     if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
	//         return
	//     }
	//     // insert custom behavior
	//     os.Exit(0)
	// }
	// ```
	ExecHelper string

	// inputs

	// Args are passed directly to cobra before executing the command. This is the primary
	// interface to control the behavior of the cli.
	Args []string
	// Stdin injects stub data to be read via os.Stdin for the command.
	Stdin []byte

	// side effects

	// ExpectCreates asserts each resource with the resources passed to the Create method of the
	// fake client in order.
	ExpectCreates []Factory
	// ExpectUpdates asserts each resource with the resources passed to the Update method of the
	// fake client in order.
	ExpectUpdates []Factory
	// ExpectDeletes assert references to the Delete method of the fake client in order.
	// Unlike Create and Update, Delete does not receive a full resource, so a reference is used
	// instead. The Group will be blank for 'core' resources. The Resource is not a Kind, but
	// plural lowercase name of the resource.
	ExpectDeletes []DeleteRef
	// ExpectDeleteCollections asserts references to the DeleteCollection method of the fake
	// client in order. DeleteCollections behaves similarly to Deletes. Unlike Delete,
	// DeleteCollection does not contain a resource Name, but may contain a LabelSelector.
	ExpectDeleteCollections []DeleteCollectionRef

	// outputs

	// ShouldError indicates if the table record command execution should return an error. The
	// test will fail if this value does not reflect the returned error.
	ShouldError bool
	// ShouldPanic is true if and only if command is expected to panic. A panic should only be
	// used to indicate the cli is misconfigured.
	ShouldPanic bool
	// ExpectOutput performs a direct comparison of this content with the command's output showing
	// a diff of any changes. The comparison is ignored for empty strings and ignores a leading
	// new line.
	ExpectOutput string
	// Verify provides the command output and error for custom assertions.
	Verify func(t *testing.T, output string, err error)

	// lifecycle

	// Prepare is called before the command is executed. It is intended to prepare that broader
	// environment before the specific table record is executed. For example, chaning the working
	// directory or setting mock expectations.
	Prepare func(t *testing.T, ctx context.Context, config *cli.Config, tc *CommandTestCase) (context.Context, error)
	// CleanUp is called after the table record is finished and all defined assertions complete.
	// It is indended to clean up any state created in the Prepare step or during the test
	// execution, or to make assertions for mocks.
	CleanUp func(t *testing.T, ctx context.Context, config *cli.Config, tc *CommandTestCase) error
}

// Run each record for the table. Tables with a focused record will run only the focused records
// and then fail, to prevent accidental check-in.
func (ts CommandTestSuite) Run(t *testing.T, scheme *runtime.Scheme, cmdFactory func(context.Context, *cli.Config) *cobra.Command) {
	t.Helper()
	focused := CommandTestSuite{}
	for _, tc := range ts {
		if tc.Focus && !tc.Skip {
			focused = append(focused, tc)
		}
	}
	if len(focused) != 0 {
		for _, tc := range focused {
			tc.Run(t, scheme, cmdFactory)
		}
		t.Errorf("test run focused on %d record(s), skipped %d record(s)", len(focused), len(ts)-len(focused))
		return
	}

	for _, tc := range ts {
		tc.Run(t, scheme, cmdFactory)
	}
}

// Run a single test case for the command. It is not common to run a record outside of a table.
func (tc CommandTestCase) Run(t *testing.T, scheme *runtime.Scheme, cmdFactory func(context.Context, *cli.Config) *cobra.Command) {
	t.Run(tc.Name, func(t *testing.T) {
		if tc.Skip {
			t.SkipNow()
		}

		ctx := context.Background()
		c := tc.Config
		if c == nil {
			c = cli.NewDefaultConfig(scheme)
		}

		// Record the given objects
		givenObjects := make([]client.Object, 0, len(tc.GivenObjects))
		originalGivenObjects := make([]client.Object, 0, len(tc.GivenObjects))
		for _, f := range tc.GivenObjects {
			object := f.CreateObject()
			givenObjects = append(givenObjects, object.DeepCopyObject().(client.Object))
			originalGivenObjects = append(originalGivenObjects, object.DeepCopyObject().(client.Object))
		}

		client := rtesting.NewFakeClient(c.Scheme, givenObjects...)
		c.Client = NewFakeCliClient(client)
		if tc.ExecHelper != "" {
			c.Exec = fakeExecCommand(tc.ExecHelper)
		}

		if tc.CleanUp != nil {
			defer func() {
				if err := tc.CleanUp(t, ctx, c, &tc); err != nil {
					t.Errorf("error during clean up: %s", err)
				}
			}()
		}
		if tc.Prepare != nil {
			var err error
			if ctx, err = tc.Prepare(t, ctx, c, &tc); err != nil {
				t.Errorf("error during prepare: %s", err)
			}
		}

		// Validate all objects that implement Validatable
		client.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return ValidateCreates(context.Background(), action)
		})
		client.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return ValidateUpdates(context.Background(), action)
		})

		for i := range tc.WithReactors {
			// in reverse order since we prepend
			reactor := tc.WithReactors[len(tc.WithReactors)-1-i]
			client.PrependReactor("*", "*", reactor)
		}

		cmd := cmdFactory(ctx, c)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		cmd.SetArgs(tc.Args)

		c.Stdin = bytes.NewBuffer(tc.Stdin)
		output := &bytes.Buffer{}
		cmd.SetOutput(output)
		c.Stdout = output
		c.Stderr = output

		if tc.ShouldPanic {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected Execute() to panic")
				}
			}()
		}
		cmdErr := cmd.Execute()

		if expected, actual := tc.ShouldError, cmdErr != nil; expected != actual {
			if expected {
				t.Errorf("expected command to error, actual %v", cmdErr)
			} else {
				t.Errorf("expected command not to error, actual %q", cmdErr)
			}
		}

		// Previous state is used to diff resource expected state for update requests that were missed.
		objPrevState := map[string]runtime.Object{}
		for _, o := range givenObjects {
			objPrevState[objKey(o)] = o
		}

		rtesting.CompareActions(t, "create", tc.ExpectCreates, client.CreateActions, rtesting.IgnoreLastTransitionTime, rtesting.SafeDeployDiff, rtesting.IgnoreTypeMeta, rtesting.IgnoreResourceVersion, cmpopts.EquateEmpty())
		rtesting.CompareActions(t, "update", tc.ExpectUpdates, client.UpdateActions, rtesting.IgnoreLastTransitionTime, rtesting.SafeDeployDiff, rtesting.IgnoreTypeMeta, rtesting.IgnoreResourceVersion, cmpopts.EquateEmpty())

		for i, expected := range tc.ExpectDeletes {
			if i >= len(client.DeleteActions) {
				t.Errorf("Missing delete: %#v", expected)
				continue
			}
			actual := NewDeleteRef(client.DeleteActions[i])
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Unexpected delete (-expected, +actual): %s", diff)
			}
		}
		if actual, expected := len(client.DeleteActions), len(tc.ExpectDeletes); actual > expected {
			for _, extra := range client.DeleteActions[expected:] {
				t.Errorf("Extra delete: %#v", extra)
			}
		}

		for i, expected := range tc.ExpectDeleteCollections {
			if i >= len(client.DeleteCollectionActions) {
				t.Errorf("Missing delete-collection: %#v", expected)
				continue
			}
			actual := NewDeleteCollectionRef(client.DeleteCollectionActions[i])
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("Unexpected delete collection (-expected, +actual): %s", diff)
			}
		}
		if actual, expected := len(client.DeleteCollectionActions), len(tc.ExpectDeleteCollections); actual > expected {
			for _, extra := range client.DeleteCollectionActions[expected:] {
				t.Errorf("Extra delete-collection: %#v", extra)
			}
		}

		outputString := output.String()
		if tc.ExpectOutput != "" {
			if diff := cmp.Diff(strings.TrimPrefix(tc.ExpectOutput, "\n"), outputString); diff != "" {
				t.Errorf("Unexpected output (-expected, +actual): %s", diff)
			}
		}

		if tc.Verify != nil {
			tc.Verify(t, outputString, cmdErr)
		}

		// Validate the given objects are not mutated by reconciliation
		if diff := cmp.Diff(originalGivenObjects, givenObjects, rtesting.IgnoreResourceVersion); diff != "" {
			t.Errorf("Given objects mutated by test %q (-expected, +actual): %v", tc.Name, diff)
		}
	})
}

func objKey(o runtime.Object) string {
	on := o.(metav1.ObjectMetaAccessor)
	// namespace + name is not unique, and the tests don't populate k8s kind
	// information, so use GoLang's type name as part of the key.
	return path.Join(reflect.TypeOf(o).String(), on.GetObjectMeta().GetNamespace(), on.GetObjectMeta().GetName())
}

func applyDefaults(o runtime.Object) {
	if d, ok := o.(defaultable); ok {
		d.Default()
	}
}

type DeleteRef struct {
	Group     string
	Resource  string
	Namespace string
	Name      string
}

func NewDeleteRef(action clientgotesting.DeleteAction) DeleteRef {
	return DeleteRef{
		Group:     action.GetResource().Group,
		Resource:  action.GetResource().Resource,
		Namespace: action.GetNamespace(),
		Name:      action.GetName(),
	}
}

type DeleteCollectionRef struct {
	Group         string
	Resource      string
	Namespace     string
	LabelSelector string
}

func NewDeleteCollectionRef(action clientgotesting.DeleteCollectionAction) DeleteCollectionRef {
	return DeleteCollectionRef{
		Group:         action.GetResource().Group,
		Resource:      action.GetResource().Resource,
		Namespace:     action.GetNamespace(),
		LabelSelector: action.GetListRestrictions().Labels.String(),
	}
}

func fakeExecCommand(helper string) func(context.Context, string, ...string) *exec.Cmd {
	// pattern derived from https://npf.io/2015/06/testing-exec-command/
	return func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess_" + helper, "--", command}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
}

type defaultable interface {
	Default()
}

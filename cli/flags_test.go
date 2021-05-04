/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vmware-labs/reconciler-runtime/cli"
	rtesting "github.com/vmware-labs/reconciler-runtime/cli/testing"
	"github.com/vmware-labs/reconciler-runtime/validation"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAllNamespacesFlag(t *testing.T) {
	tests := []struct {
		name                string
		args                []string
		prior               func(cmd *cobra.Command, args []string) error
		namespace           string
		actualNamespace     string
		allNamespaces       bool
		actualAllNamespaces bool
		err                 error
	}{{
		name:      "default",
		args:      []string{},
		namespace: "default",
	}, {
		name:      "explicit namespace",
		args:      []string{cli.NamespaceFlagName, "my-namespace"},
		namespace: "my-namespace",
	}, {
		name:          "all namespaces",
		args:          []string{cli.AllNamespacesFlagName},
		allNamespaces: true,
	}, {
		name: "explicit namespace and all namespaces",
		args: []string{cli.NamespaceFlagName, "default", cli.AllNamespacesFlagName},
		err:  validation.ErrMultipleOneOf(cli.NamespaceFlagName, cli.AllNamespacesFlagName).ToAggregate(),
	}, {
		name: "prior PreRunE",
		args: []string{},
		prior: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		namespace: "default",
	}, {
		name: "prior PreRunE, error",
		args: []string{},
		prior: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("prior PreRunE error")
		},
		err: fmt.Errorf("prior PreRunE error"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			c := cli.NewDefaultConfig(scheme)
			c.Client = rtesting.NewFakeCliClient(rtesting.NewFakeClient(scheme))
			cmd := &cobra.Command{
				PreRunE: test.prior,
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			cli.AllNamespacesFlag(cmd, c, &test.actualNamespace, &test.actualAllNamespaces)

			if cmd.Flag(cli.StripDash(cli.NamespaceFlagName)) == nil {
				t.Errorf("Expected %s to be defined", cli.NamespaceFlagName)
			}
			if cmd.Flag(cli.StripDash(cli.AllNamespacesFlagName)) == nil {
				t.Errorf("Expected %s to be defined", cli.AllNamespacesFlagName)
			}

			cmd.SetArgs(test.args)
			cmd.SetOutput(&bytes.Buffer{})
			err := cmd.Execute()

			if expected, actual := fmt.Sprintf("%s", test.err), fmt.Sprintf("%s", err); expected != actual {
				t.Errorf("Expected error %q, actually %q", expected, actual)
			}
			if err == nil {
				if expected, actual := test.namespace, test.actualNamespace; expected != actual {
					t.Errorf("Expected namespace %q, actually %q", expected, actual)
				}
				if expected, actual := test.allNamespaces, test.actualAllNamespaces; expected != actual {
					t.Errorf("Expected all namespace %v, actually %v", expected, actual)
				}
			}
		})
	}
}

func TestNamespaceFlag(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		prior           func(cmd *cobra.Command, args []string) error
		namespace       string
		actualNamespace string
		err             error
	}{{
		name:      "default",
		args:      []string{},
		namespace: "default",
	}, {
		name:      "explicit namespace",
		args:      []string{cli.NamespaceFlagName, "my-namespace"},
		namespace: "my-namespace",
	}, {
		name: "prior PreRunE",
		args: []string{},
		prior: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		namespace: "default",
	}, {
		name: "prior PreRunE, error",
		args: []string{},
		prior: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("prior PreRunE error")
		},
		err: fmt.Errorf("prior PreRunE error"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			c := cli.NewDefaultConfig(scheme)
			c.Client = rtesting.NewFakeCliClient(rtesting.NewFakeClient(scheme))
			cmd := &cobra.Command{
				PreRunE: test.prior,
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			cli.NamespaceFlag(cmd, c, &test.actualNamespace)

			if cmd.Flag(cli.StripDash(cli.NamespaceFlagName)) == nil {
				t.Errorf("Expected %s to be defined", cli.NamespaceFlagName)
			}
			if cmd.Flag(cli.StripDash(cli.AllNamespacesFlagName)) != nil {
				t.Errorf("Expected %s not to be defined", cli.AllNamespacesFlagName)
			}

			cmd.SetArgs(test.args)
			cmd.SetOutput(&bytes.Buffer{})
			err := cmd.Execute()

			if expected, actual := fmt.Sprintf("%s", test.err), fmt.Sprintf("%s", err); expected != actual {
				t.Errorf("Expected error %q, actually %q", expected, actual)
			}
			if err == nil {
				if expected, actual := test.namespace, test.actualNamespace; expected != actual {
					t.Errorf("Expected namespace %q, actually %q", expected, actual)
				}
			}
		})
	}
}

func TestStripDash(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{{
		name: "empty",
	}, {
		name:   "remove leading dash",
		input:  "--flag",
		output: "flag",
	}, {
		name:   "ingore extra dash",
		input:  "--flag-name",
		output: "flag-name",
	}, {
		name:   "ingore extra doubledash",
		input:  "--flag--",
		output: "flag--",
	}, {
		name:   "no dashes",
		input:  "flag",
		output: "flag",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if expected, actual := test.output, cli.StripDash(test.input); expected != actual {
				t.Errorf("Expected dash stripped string to be %q, actually %q", expected, actual)
			}
		})
	}
}

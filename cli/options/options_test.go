/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package options_test

import (
	"testing"

	"github.com/vmware-labs/reconciler-runtime/cli"
	"github.com/vmware-labs/reconciler-runtime/cli/options"
	rtesting "github.com/vmware-labs/reconciler-runtime/cli/testing"
	"github.com/vmware-labs/reconciler-runtime/cli/validation"
)

func TestListOptions(t *testing.T) {
	table := rtesting.OptionsTestSuite{
		{
			Name: "default",
			Options: &options.ListOptions{
				Namespace: "default",
			},
			ShouldValidate: true,
		},
		{
			Name: "all namespaces",
			Options: &options.ListOptions{
				AllNamespaces: true,
			},
			ShouldValidate: true,
		},
		{
			Name:              "neither",
			Options:           &options.ListOptions{},
			ExpectFieldErrors: validation.ErrMissingOneOf(cli.NamespaceFlagName, cli.AllNamespacesFlagName),
		},
		{
			Name: "both",
			Options: &options.ListOptions{
				Namespace:     "default",
				AllNamespaces: true,
			},
			ExpectFieldErrors: validation.ErrMultipleOneOf(cli.NamespaceFlagName, cli.AllNamespacesFlagName),
		},
	}

	table.Run(t)
}

func TestResourceOptions(t *testing.T) {
	table := rtesting.OptionsTestSuite{
		{
			Name:    "default",
			Options: &options.ResourceOptions{},
			ExpectFieldErrors: validation.FieldErrors{}.Also(
				validation.ErrMissingField(cli.NamespaceFlagName),
				validation.ErrMissingField(cli.NameArgumentName),
			),
		},
		{
			Name: "has both",
			Options: &options.ResourceOptions{
				Namespace: "default",
				Name:      "push-credentials",
			},
			ShouldValidate: true,
		},
		{
			Name: "missing namespace",
			Options: &options.ResourceOptions{
				Name: "push-credentials",
			},
			ExpectFieldErrors: validation.ErrMissingField(cli.NamespaceFlagName),
		},
		{
			Name: "missing name",
			Options: &options.ResourceOptions{
				Namespace: "default",
			},
			ExpectFieldErrors: validation.ErrMissingField(cli.NameArgumentName),
		},
	}

	table.Run(t)
}

func TestDeleteOptions(t *testing.T) {
	table := rtesting.OptionsTestSuite{
		{
			Name: "default",
			Options: &options.DeleteOptions{
				Namespace: "default",
			},
			ExpectFieldErrors: validation.ErrMissingOneOf(cli.AllFlagName, cli.NamesArgumentName),
		},
		{
			Name: "single name",
			Options: &options.DeleteOptions{
				Namespace: "default",
				Names:     []string{"my-resource"},
			},
			ShouldValidate: true,
		},
		{
			Name: "multiple names",
			Options: &options.DeleteOptions{
				Namespace: "default",
				Names:     []string{"my-resource", "my-other-resource"},
			},
			ShouldValidate: true,
		},
		{
			Name: "invalid name",
			Options: &options.DeleteOptions{
				Namespace: "default",
				Names:     []string{"my.resource"},
			},
			ExpectFieldErrors: validation.ErrInvalidValue("my.resource", validation.CurrentField).ViaFieldIndex(cli.NamesArgumentName, 0),
		},
		{
			Name: "all",
			Options: &options.DeleteOptions{
				Namespace: "default",
				All:       true,
			},
			ShouldValidate: true,
		},
		{
			Name: "all with name",
			Options: &options.DeleteOptions{
				Namespace: "default",
				Names:     []string{"my-resource"},
				All:       true,
			},
			ExpectFieldErrors: validation.ErrMultipleOneOf(cli.AllFlagName, cli.NamesArgumentName),
		},
		{
			Name: "missing namespace",
			Options: &options.DeleteOptions{
				Names: []string{"my-resource"},
			},
			ExpectFieldErrors: validation.ErrMissingField(cli.NamespaceFlagName),
		},
	}

	table.Run(t)
}

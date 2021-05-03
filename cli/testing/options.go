/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"github.com/vmware-labs/reconciler-runtime/cli"
	"github.com/vmware-labs/reconciler-runtime/cli/options"
	"github.com/vmware-labs/reconciler-runtime/validation"
)

const TestField = "test-field"

var (
	ValidListOptions = options.ListOptions{
		Namespace: "default",
	}
	InvalidListOptions           = options.ListOptions{}
	InvalidListOptionsFieldError = validation.ErrMissingOneOf(cli.NamespaceFlagName, cli.AllNamespacesFlagName)
)

var (
	ValidResourceOptions = options.ResourceOptions{
		Namespace: "default",
		Name:      "my-resource",
	}
	InvalidResourceOptions           = options.ResourceOptions{}
	InvalidResourceOptionsFieldError = validation.FieldErrors{}.Also(
		validation.ErrMissingField(cli.NamespaceFlagName),
		validation.ErrMissingField(cli.NameArgumentName),
	)
)

var (
	ValidDeleteOptions = options.DeleteOptions{
		Namespace: "default",
		Names:     []string{"my-resource"},
	}
	InvalidDeleteOptions = options.DeleteOptions{
		Namespace: "default",
	}
	InvalidDeleteOptionsFieldError = validation.ErrMissingOneOf(cli.AllFlagName, cli.NamesArgumentName)
)

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package options

import (
	"context"

	"github.com/vmware-labs/reconciler-runtime/cli"
	"github.com/vmware-labs/reconciler-runtime/cli/validation"
)

type ListOptions struct {
	Namespace     string
	AllNamespaces bool
}

func (opts *ListOptions) Validate(ctx context.Context) validation.FieldErrors {
	errs := validation.FieldErrors{}

	if opts.Namespace == "" && !opts.AllNamespaces {
		errs = errs.Also(validation.ErrMissingOneOf(cli.NamespaceFlagName, cli.AllNamespacesFlagName))
	}
	if opts.Namespace != "" && opts.AllNamespaces {
		errs = errs.Also(validation.ErrMultipleOneOf(cli.NamespaceFlagName, cli.AllNamespacesFlagName))
	}

	return errs
}

type ResourceOptions struct {
	Namespace string
	Name      string
}

func (opts *ResourceOptions) Validate(ctx context.Context) validation.FieldErrors {
	errs := validation.FieldErrors{}

	if opts.Namespace == "" {
		errs = errs.Also(validation.ErrMissingField(cli.NamespaceFlagName))
	}

	if opts.Name == "" {
		errs = errs.Also(validation.ErrMissingField(cli.NameArgumentName))
	} else {
		errs = errs.Also(validation.K8sName(opts.Name, cli.NameArgumentName))
	}

	return errs
}

type DeleteOptions struct {
	Namespace string
	Names     []string
	All       bool
}

func (opts *DeleteOptions) Validate(ctx context.Context) validation.FieldErrors {
	errs := validation.FieldErrors{}

	if opts.Namespace == "" {
		errs = errs.Also(validation.ErrMissingField(cli.NamespaceFlagName))
	}

	if opts.All && len(opts.Names) != 0 {
		errs = errs.Also(validation.ErrMultipleOneOf(cli.AllFlagName, cli.NamesArgumentName))
	}
	if !opts.All && len(opts.Names) == 0 {
		errs = errs.Also(validation.ErrMissingOneOf(cli.AllFlagName, cli.NamesArgumentName))
	}

	errs = errs.Also(validation.K8sNames(opts.Names, cli.NamesArgumentName))

	return errs
}

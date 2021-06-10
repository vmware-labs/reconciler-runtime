/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vmware-labs/reconciler-runtime/cli"
	"github.com/vmware-labs/reconciler-runtime/cli/options"
	clitesting "github.com/vmware-labs/reconciler-runtime/cli/testing"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type TestResourceCreateOptions struct {
	options.ResourceOptions
}

var (
	_ validation.Validatable = (*TestResourceCreateOptions)(nil)
	_ cli.Executable         = (*TestResourceCreateOptions)(nil)
)

func (opts *TestResourceCreateOptions) Validate(ctx context.Context) validation.FieldErrors {
	errs := validation.FieldErrors{}

	errs = errs.Also(opts.ResourceOptions.Validate(ctx))

	return errs
}

func (opts *TestResourceCreateOptions) Exec(ctx context.Context, c *cli.Config) error {
	resource := &rtesting.TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.Namespace,
			Name:      opts.Name,
		},
	}
	err := c.Create(ctx, resource)
	if err != nil {
		return err
	}
	c.Successf("Created resource %q\n", resource.Name)
	return nil
}

func NewTestResourceCreateCommand(ctx context.Context, c *cli.Config) *cobra.Command {
	opts := &TestResourceCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a resource",
		Example: strings.Join([]string{
			fmt.Sprintf("%s resource create my-resource", c.Name),
		}, "\n"),
		PreRunE: cli.ValidateOptions(ctx, opts),
		RunE:    cli.ExecOptions(ctx, c, opts),
	}

	cli.Args(cmd,
		cli.NameArg(&opts.Name),
	)

	cli.NamespaceFlag(cmd, c, &opts.Namespace)

	return cmd
}

func TestOptionsTestSuite(t *testing.T) {
	table := clitesting.OptionsTestSuite{
		{
			Name: "invalid resource",
			Options: &TestResourceCreateOptions{
				ResourceOptions: clitesting.InvalidResourceOptions,
			},
			ExpectFieldErrors: clitesting.InvalidResourceOptionsFieldError,
		},
		{
			Name: "valid",
			Options: &TestResourceCreateOptions{
				ResourceOptions: clitesting.ValidResourceOptions,
			},
			ShouldValidate: true,
		},
		{
			Name: "skipped test",
			Skip: true,
			Options: &TestResourceCreateOptions{
				ResourceOptions: clitesting.InvalidResourceOptions,
			},
			ShouldValidate: true,
		},
	}

	table.Run(t)
}

func TestWorkloadCreateCommand(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = rtesting.AddToScheme(scheme)

	table := clitesting.CommandTestSuite{
		{
			Name:        "invalid args",
			Args:        []string{},
			ShouldError: true,
		},
		{
			Name: "create resource",
			Args: []string{"my-resource"},
			ExpectCreates: []clitesting.Factory{
				clitesting.Wrapper(&rtesting.TestResource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "my-resource",
					},
					Spec: rtesting.TestResourceSpec{},
				}),
			},
			ExpectOutput: `
Created resource "my-resource"
`,
		},
		{
			Name: "create failed",
			Args: []string{"my-resource"},
			GivenObjects: []rtesting.Factory{
				clitesting.Wrapper(&rtesting.TestResource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "my-resource",
					},
					Spec: rtesting.TestResourceSpec{},
				}),
			},
			ExpectCreates: []clitesting.Factory{
				clitesting.Wrapper(&rtesting.TestResource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "my-resource",
					},
					Spec: rtesting.TestResourceSpec{},
				}),
			},
			ShouldError: true,
		},
	}

	table.Run(t, scheme, NewTestResourceCreateCommand)
}

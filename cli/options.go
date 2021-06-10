/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/vmware-labs/reconciler-runtime/cli/validation"
)

// ValidateOptions bridges a cobra RunE function to the Validatable interface.  All flags and
// arguments must already be bound, with explicit or default values, to the options struct being
// validated. This function is typically used to define the PreRunE phase of a command.
//
// ```
// cmd := &cobra.Command{
// 	   ...
// 	   PreRunE: cli.ValidateOptions(opts),
// }
// ```
func ValidateOptions(ctx context.Context, opts validation.Validatable) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := WithCommand(ctx, cmd)
		if err := opts.Validate(ctx); len(err) != 0 {
			return err.ToAggregate()
		}
		cmd.SilenceUsage = true
		return nil
	}
}

type Executable interface {
	Exec(ctx context.Context, c *Config) error
}

// ExecOptions bridges a cobra RunE function to the Executable interface.  All flags and
// arguments must already be bound, with explicit or default values, to the options struct being
// validated. This function is typically used to define the RunE phase of a command.
//
// ```
// cmd := &cobra.Command{
// 	   ...
// 	   RunE: cli.ExecOptions(c, opts),
// }
// ```
func ExecOptions(ctx context.Context, c *Config, opts Executable) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := WithCommand(ctx, cmd)
		if o, ok := opts.(DryRunable); ok && o.IsDryRun() {
			// reserve Stdout for resources, redirect normal stdout to stderr
			ctx = withStdout(ctx, c.Stdout)
			c.Stdout = c.Stderr
		}
		return opts.Exec(ctx, c)
	}
}

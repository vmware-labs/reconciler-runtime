/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vmware-labs/reconciler-runtime/cli"
	"github.com/vmware-labs/reconciler-runtime/cli/validation"
)

type StubValidateOptions struct {
	called        bool
	validationErr validation.FieldErrors
}

func (o *StubValidateOptions) Validate(ctx context.Context) validation.FieldErrors {
	o.called = true
	return o.validationErr
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name          string
		opts          *StubValidateOptions
		expectedErr   error
		usageSilenced bool
	}{{
		name:          "valid, no error",
		opts:          &StubValidateOptions{},
		usageSilenced: true,
	}, {
		name: "valid, empty error",
		opts: &StubValidateOptions{
			validationErr: validation.FieldErrors{},
		},
		usageSilenced: true,
	}, {
		name: "validation error",
		opts: &StubValidateOptions{
			validationErr: validation.ErrMissingField("field-name"),
		},
		expectedErr:   validation.ErrMissingField("field-name").ToAggregate(),
		usageSilenced: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			cmd := &cobra.Command{}
			err := cli.ValidateOptions(ctx, test.opts)(cmd, []string{})

			if expected, actual := true, test.opts.called; true != actual {
				t.Errorf("expected called to be %v, actually %v", expected, actual)
			}
			if expected, actual := test.expectedErr, err; fmt.Sprintf("%s", expected) != fmt.Sprintf("%s", actual) {
				t.Errorf("expected error to be %v, actually %v", expected, actual)
			}
			if expected, actual := test.usageSilenced, cmd.SilenceUsage; expected != actual {
				t.Errorf("expected cmd.SilenceUsage to be %v, actually %v", expected, actual)
			}
		})
	}
}

type StubExecOptions struct {
	dryRun  bool
	called  bool
	config  *cli.Config
	cmd     *cobra.Command
	execErr error
}

var (
	_ cli.Executable = (*StubExecOptions)(nil)
	_ cli.DryRunable = (*StubExecOptions)(nil)
)

func (o *StubExecOptions) Exec(ctx context.Context, c *cli.Config) error {
	o.called = true
	o.config = c
	o.cmd = cli.CommandFromContext(ctx)
	return o.execErr
}

func (o *StubExecOptions) IsDryRun() bool {
	return o.dryRun
}

func TestExecOptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        *StubExecOptions
		expectedErr error
	}{{
		name: "success",
		opts: &StubExecOptions{},
	}, {
		name: "failure",
		opts: &StubExecOptions{
			execErr: fmt.Errorf("test exec error"),
		},
		expectedErr: fmt.Errorf("test exec error"),
	}, {
		name: "dry run",
		opts: &StubExecOptions{
			dryRun: true,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			cmd := &cobra.Command{}
			config := &cli.Config{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			}
			err := cli.ExecOptions(ctx, config, test.opts)(cmd, []string{})

			if expected, actual := true, test.opts.called; true != actual {
				t.Errorf("expected called to be %v, actually %v", expected, actual)
			}
			if expected, actual := test.expectedErr, err; fmt.Sprintf("%s", expected) != fmt.Sprintf("%s", actual) {
				t.Errorf("expected error to be %v, actually %v", expected, actual)
			}
			if expected, actual := config, test.opts.config; expected != actual {
				t.Errorf("expected config to be %v, actually %v", expected, actual)
			}
			if expected, actual := cmd, test.opts.cmd; expected != actual {
				t.Errorf("expected command to be %v, actually %v", expected, actual)
			}
			if test.opts.dryRun {
				if config.Stdout != config.Stderr {
					t.Errorf("expected stdout and stderr to be the same, actually %v %v", config.Stdout, config.Stderr)
				}
			} else {
				if config.Stdout == config.Stderr {
					t.Errorf("expected stdout and stderr to be different, actually %v %v", config.Stdout, config.Stderr)
				}
			}
		})
	}
}

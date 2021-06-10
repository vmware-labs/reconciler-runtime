/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"bufio"
	"context"
	"io/ioutil"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

func Sequence(items ...func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for i := range items {
			if err := items[i](cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
}

func Visit(cmd *cobra.Command, f func(c *cobra.Command) error) error {
	err := f(cmd)
	if err != nil {
		return err
	}
	for _, c := range cmd.Commands() {
		err := Visit(c, f)
		if err != nil {
			return err
		}
	}
	return nil
}

func ReadStdin(c *Config, value *[]byte, prompt string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if terminal.IsTerminal(int(syscall.Stdin)) {
			c.Printf("%s: ", prompt)
			res, err := terminal.ReadPassword(int(syscall.Stdin))
			c.Printf("\n")
			if err != nil {
				return err
			}
			*value = res
		} else {
			reader := bufio.NewReader(c.Stdin)
			res, err := ioutil.ReadAll(reader)
			if err != nil {
				return err
			}
			*value = res
		}

		return nil
	}
}

type commandKey struct{}

func WithCommand(ctx context.Context, cmd *cobra.Command) context.Context {
	return context.WithValue(ctx, commandKey{}, cmd)
}

func CommandFromContext(ctx context.Context) *cobra.Command {
	if cmd, ok := ctx.Value(commandKey{}).(*cobra.Command); ok {
		return cmd
	}
	return nil
}

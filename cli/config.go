/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware-labs/reconciler-runtime/cli/printer"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Config struct {
	CompiledEnv
	Client
	Scheme          *runtime.Scheme
	ViperConfigFile string
	KubeConfigFile  string
	CurrentContext  string
	Exec            func(ctx context.Context, command string, args ...string) *exec.Cmd
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
}

func NewDefaultConfig(scheme *runtime.Scheme) *Config {
	return &Config{
		Scheme:      scheme,
		CompiledEnv: env,
		Exec:        exec.CommandContext,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}
}

func (c *Config) Tail(ctx context.Context, namespace string, selector labels.Selector, containers []string, since time.Duration) error {
	if _, err := exec.LookPath("stern"); err != nil {
		c.Infof("Install stern to view logs: https://github.com/stern/stern")
		<-ctx.Done()
	}

	args := []string{}
	args = append(args, "--namespace", namespace)
	args = append(args, "--selector", selector.String())
	for _, c := range containers {
		args = append(args, "--container", c)
	}

	cmdargs := args
	cmdargs = append(cmdargs, "--kubeconfig", c.KubeConfigFile)
	cmdargs = append(cmdargs, "--context", c.CurrentContext)
	cmdargs = append(cmdargs, "--since", since.String())
	cmdargs = append(cmdargs, "--template", "[{{color .ContainerColor .ContainerName}}] {{.Message}}")
	if color.NoColor {
		cmdargs = append(cmdargs, "--color", "never")
	} else {
		cmdargs = append(cmdargs, "--color", "always")
	}

	cmd := c.Exec(ctx, "stern", cmdargs...)
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Env = os.Environ()

	c.Efaintf("%s %s\n", cmd.Args[0], strings.Join(args, " "))
	return cmd.Run()
}

func (c *Config) Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(c.Stdout, format, a...)
}

func (c *Config) Eprintf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(c.Stderr, format, a...)
}

func (c *Config) Infof(format string, a ...interface{}) (n int, err error) {
	return printer.InfoColor.Fprintf(c.Stdout, format, a...)
}

func (c *Config) Einfof(format string, a ...interface{}) (n int, err error) {
	return printer.InfoColor.Fprintf(c.Stderr, format, a...)
}

func (c *Config) Successf(format string, a ...interface{}) (n int, err error) {
	return printer.SuccessColor.Fprintf(c.Stdout, format, a...)
}

func (c *Config) Esuccessf(format string, a ...interface{}) (n int, err error) {
	return printer.SuccessColor.Fprintf(c.Stderr, format, a...)
}

func (c *Config) Errorf(format string, a ...interface{}) (n int, err error) {
	return printer.ErrorColor.Fprintf(c.Stdout, format, a...)
}

func (c *Config) Eerrorf(format string, a ...interface{}) (n int, err error) {
	return printer.ErrorColor.Fprintf(c.Stderr, format, a...)
}

func (c *Config) Faintf(format string, a ...interface{}) (n int, err error) {
	return printer.FaintColor.Fprintf(c.Stdout, format, a...)
}

func (c *Config) Efaintf(format string, a ...interface{}) (n int, err error) {
	return printer.FaintColor.Fprintf(c.Stderr, format, a...)
}

func Initialize(scheme *runtime.Scheme) *Config {
	c := NewDefaultConfig(scheme)

	cobra.OnInitialize(c.initViperConfig)
	cobra.OnInitialize(c.initKubeConfig)
	cobra.OnInitialize(c.init)

	return c
}

// initViperConfig reads in config file and ENV variables if set.
func (c *Config) initViperConfig() {
	if c.ViperConfigFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(c.ViperConfigFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			// avoid color since we don't know if it should be enabled yet
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory with name ".txs" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("." + c.Name)
	}

	viper.SetEnvPrefix(c.Name)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	// hack for no-color since we urgently need to know if color should be disabled
	if viper.GetBool(StripDash(NoColorFlagName)) {
		color.NoColor = true
	}
	if err == nil {
		c.Einfof("Using config file: %s\n", viper.ConfigFileUsed())
	}
}

// initKubeConfig defines the default location for the kubectl config file
func (c *Config) initKubeConfig() {
	if c.KubeConfigFile != "" {
		return
	}
	if kubeEnvConf, ok := os.LookupEnv("KUBECONFIG"); ok {
		c.KubeConfigFile = kubeEnvConf
	}
}

func (c *Config) init() {
	if c.Client == nil {
		c.Client = NewClient(c.KubeConfigFile, c.CurrentContext, c.Scheme)
	}
}

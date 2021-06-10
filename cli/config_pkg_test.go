/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"bytes"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestInitialize(t *testing.T) {
	scheme := runtime.NewScheme()
	c := Initialize(scheme)

	if c.Stdin != os.Stdin {
		t.Errorf("expected Stdin to be os.Stdin")
	}
	if c.Stdout != os.Stdout {
		t.Errorf("expected Stdout to be os.Stdout")
	}
	if c.Stderr != os.Stderr {
		t.Errorf("expected Stderr to be os.Stderr")
	}
	if c.Scheme != scheme {
		t.Errorf("expected Scheme to be scheme")
	}
	if c.CompiledEnv != env {
		t.Errorf("expected CompiledEnv to be env")
	}
	if reflect.ValueOf(c.Exec).Pointer() != reflect.ValueOf(exec.CommandContext).Pointer() {
		t.Errorf("expected Exec to be exec.CommandContext")
	}
}

func TestInitViperConfig(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	defer viper.Reset()

	scheme := runtime.NewScheme()
	c := NewDefaultConfig(scheme)
	output := &bytes.Buffer{}
	c.Stdout = output
	c.Stderr = output

	c.ViperConfigFile = "testdata/.unknown.yaml"
	c.initViperConfig()

	expectedViperSettings := map[string]interface{}{
		"no-color": true,
	}
	if diff := cmp.Diff(expectedViperSettings, viper.AllSettings()); diff != "" {
		t.Errorf("Unexpected viper settings (-expected, +actual): %s", diff)
	}
	if diff := cmp.Diff("Using config file: testdata/.unknown.yaml", strings.TrimSpace(output.String())); diff != "" {
		t.Errorf("Unexpected output (-expected, +actual): %s", diff)
	}
}

func TestInitViperConfig_HomeDir(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	defer viper.Reset()

	home, homeisset := os.LookupEnv("HOME")
	defer func() {
		homedir.Reset()
		if homeisset {
			os.Setenv("HOME", home)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	scheme := runtime.NewScheme()
	c := NewDefaultConfig(scheme)
	output := &bytes.Buffer{}
	c.Stdout = output
	c.Stderr = output

	os.Setenv("HOME", "testdata")
	c.initViperConfig()

	expectedViperSettings := map[string]interface{}{
		"no-color": true,
	}
	if diff := cmp.Diff(expectedViperSettings, viper.AllSettings()); diff != "" {
		t.Errorf("Unexpected viper settings (-expected, +actual): %s", diff)
	}
}

func TestInitKubeConfig_Flag(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	scheme := runtime.NewScheme()
	c := NewDefaultConfig(scheme)
	output := &bytes.Buffer{}
	c.Stdout = output
	c.Stderr = output

	c.KubeConfigFile = "testdata/.kube/config"
	c.initKubeConfig()

	if expected, actual := "testdata/.kube/config", c.KubeConfigFile; expected != actual {
		t.Errorf("Expected kubeconfig path %q, actually %q", expected, actual)
	}
	if diff := cmp.Diff("", strings.TrimSpace(output.String())); diff != "" {
		t.Errorf("Unexpected output (-expected, +actual): %s", diff)
	}
}

func TestInitKubeConfig_EnvVar(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	kubeconfig, kubeconfigisset := os.LookupEnv("KUBECONFIG")
	defer func() {
		if kubeconfigisset {
			os.Setenv("KUBECONFIG", kubeconfig)
		} else {
			os.Unsetenv("KUBECONFIG")
		}
	}()

	scheme := runtime.NewScheme()
	c := NewDefaultConfig(scheme)
	output := &bytes.Buffer{}
	c.Stdout = output
	c.Stderr = output

	os.Setenv("KUBECONFIG", "testdata/.kube/config")
	c.initKubeConfig()

	if expected, actual := "testdata/.kube/config", c.KubeConfigFile; expected != actual {
		t.Errorf("Expected kubeconfig path %q, actually %q", expected, actual)
	}
	if diff := cmp.Diff("", strings.TrimSpace(output.String())); diff != "" {
		t.Errorf("Unexpected output (-expected, +actual): %s", diff)
	}
}

func TestInit(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = noColor }()

	scheme := runtime.NewScheme()
	c := NewDefaultConfig(scheme)
	output := &bytes.Buffer{}
	c.Stdout = output
	c.Stderr = output

	c.KubeConfigFile = "testdata/.kube/config"
	c.init()

	if expected, actual := "default", c.DefaultNamespace(); expected != actual {
		t.Errorf("Expected default namespace %q, actually %q", expected, actual)
	}
	if diff := cmp.Diff("", strings.TrimSpace(output.String())); diff != "" {
		t.Errorf("Unexpected output (-expected, +actual): %s", diff)
	}
	if c.Client == nil {
		t.Errorf("Expected c.Client tp be set, actually %v", c.Client)
	}
}

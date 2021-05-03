/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/vmware-labs/reconciler-runtime/validation"
)

const (
	AllFlagName                  = "--all"
	AllNamespacesFlagName        = "--all-namespaces"
	DryRunFlagName               = "--dry-run"
	KubeConfigFlagName           = "--kubeconfig"
	KubeConfigFlagNameDeprecated = "--kube-config"
	NamespaceFlagName            = "--namespace"
	NoColorFlagName              = "--no-color"
)

func AllNamespacesFlag(cmd *cobra.Command, c *Config, namespace *string, allNamespaces *bool) {
	prior := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if *allNamespaces {
			if cmd.Flag(StripDash(NamespaceFlagName)).Changed {
				// forbid --namespace alongside --all-namespaces
				// Check here since we need the Flag to know if the namespace came from a flag
				return validation.ErrMultipleOneOf(NamespaceFlagName, AllNamespacesFlagName).ToAggregate()
			}
			*namespace = ""
		}
		if prior != nil {
			if err := prior(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}

	NamespaceFlag(cmd, c, namespace)
	cmd.Flags().BoolVar(allNamespaces, StripDash(AllNamespacesFlagName), false, "use all kubernetes namespaces")
}

func NamespaceFlag(cmd *cobra.Command, c *Config, namespace *string) {
	prior := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if *namespace == "" {
			*namespace = c.DefaultNamespace()
		}
		if prior != nil {
			if err := prior(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}

	cmd.Flags().StringVarP(namespace, StripDash(NamespaceFlagName), "n", "", "kubernetes `name`space (defaulted from kube config)")
	_ = cmd.MarkFlagCustom(StripDash(NamespaceFlagName), "__"+c.Name+"_list_namespaces")
}

func StripDash(flagName string) string {
	return strings.Replace(flagName, "--", "", 1)
}

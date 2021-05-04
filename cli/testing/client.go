/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-labs/reconciler-runtime/cli"
)

func (c *fakeclient) DefaultNamespace() string {
	return "default"
}

func (c *fakeclient) KubeRestConfig() *rest.Config {
	panic(fmt.Errorf("not implemented"))
}

func (c *fakeclient) Discovery() discovery.DiscoveryInterface {
	panic(fmt.Errorf("not implemented"))
}

func NewFakeCliClient(c crclient.Client) cli.Client {
	return &fakeclient{
		defaultNamespace: "default",
		Client:           c,
	}
}

type fakeclient struct {
	defaultNamespace string
	crclient.Client
}

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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

func (c *fakeclient) Get(ctx context.Context, key crclient.ObjectKey, obj crclient.Object) error {
	return c.client.Get(ctx, key, obj)
}

func (c *fakeclient) List(ctx context.Context, list crclient.ObjectList, opts ...crclient.ListOption) error {
	return c.client.List(ctx, list, opts...)
}

func (c *fakeclient) Create(ctx context.Context, obj crclient.Object, opts ...crclient.CreateOption) error {
	return c.client.Create(ctx, obj, opts...)
}

func (c *fakeclient) Delete(ctx context.Context, obj crclient.Object, opts ...crclient.DeleteOption) error {
	return c.client.Delete(ctx, obj, opts...)
}

func (c *fakeclient) Update(ctx context.Context, obj crclient.Object, opts ...crclient.UpdateOption) error {
	return c.client.Update(ctx, obj, opts...)
}

func (c *fakeclient) Patch(ctx context.Context, obj crclient.Object, patch crclient.Patch, opts ...crclient.PatchOption) error {
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *fakeclient) DeleteAllOf(ctx context.Context, obj crclient.Object, opts ...crclient.DeleteAllOfOption) error {
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *fakeclient) Status() crclient.StatusWriter {
	return c.client.Status()
}

func (c *fakeclient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *fakeclient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

func NewFakeCliClient(c crclient.Client) cli.Client {
	return &fakeclient{
		defaultNamespace: "default",
		client:           c,
	}
}

type fakeclient struct {
	defaultNamespace string
	client           crclient.Client
}

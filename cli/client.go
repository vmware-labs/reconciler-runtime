/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Client interface {
	DefaultNamespace() string
	KubeRestConfig() *rest.Config
	Discovery() discovery.DiscoveryInterface
	crclient.Client
}

func (c *client) DefaultNamespace() string {
	return c.lazyLoadDefaultNamespaceOrDie()
}

func (c *client) KubeRestConfig() *rest.Config {
	return c.lazyLoadRestConfigOrDie()
}

func (c *client) Discovery() discovery.DiscoveryInterface {
	return c.lazyLoadKubernetesClientsetOrDie().Discovery()
}

func (c *client) Client() crclient.Client {
	return c.lazyLoadClientOrDie()
}

func (c *client) Get(ctx context.Context, key crclient.ObjectKey, obj crclient.Object) error {
	return c.Client().Get(ctx, key, obj)
}

func (c *client) List(ctx context.Context, list crclient.ObjectList, opts ...crclient.ListOption) error {
	return c.Client().List(ctx, list, opts...)
}

func (c *client) Create(ctx context.Context, obj crclient.Object, opts ...crclient.CreateOption) error {
	return c.Client().Create(ctx, obj, opts...)
}

func (c *client) Delete(ctx context.Context, obj crclient.Object, opts ...crclient.DeleteOption) error {
	return c.Client().Delete(ctx, obj, opts...)
}

func (c *client) Update(ctx context.Context, obj crclient.Object, opts ...crclient.UpdateOption) error {
	return c.Client().Update(ctx, obj, opts...)
}

func (c *client) Patch(ctx context.Context, obj crclient.Object, patch crclient.Patch, opts ...crclient.PatchOption) error {
	return c.Client().Patch(ctx, obj, patch, opts...)
}

func (c *client) DeleteAllOf(ctx context.Context, obj crclient.Object, opts ...crclient.DeleteAllOfOption) error {
	return c.Client().DeleteAllOf(ctx, obj, opts...)
}

func (c *client) Status() crclient.StatusWriter {
	panic(fmt.Errorf("not implemented"))
}

func (c *client) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *client) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

func NewClient(kubeConfigFile string, scheme *runtime.Scheme) Client {
	return &client{
		kubeConfigFile: kubeConfigFile,
		scheme:         scheme,
	}
}

type client struct {
	defaultNamespace string
	kubeConfigFile   string
	scheme           *runtime.Scheme
	kubeConfig       clientcmd.ClientConfig
	restConfig       *rest.Config
	kubeClientset    *kubernetes.Clientset
	client           crclient.Client
}

func (c *client) lazyLoadKubeConfig() clientcmd.ClientConfig {
	if c.kubeConfig == nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules.ExplicitPath = c.kubeConfigFile
		configOverrides := &clientcmd.ConfigOverrides{}
		c.kubeConfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	}
	return c.kubeConfig
}

func (c *client) lazyLoadRestConfigOrDie() *rest.Config {
	if c.restConfig == nil {
		kubeConfig := c.lazyLoadKubeConfig()
		restConfig, err := kubeConfig.ClientConfig()
		if err != nil {
			panic(err)
		}
		c.restConfig = restConfig
	}
	return c.restConfig
}

func (c *client) lazyLoadKubernetesClientsetOrDie() *kubernetes.Clientset {
	if c.kubeClientset == nil {
		restConfig := c.lazyLoadRestConfigOrDie()
		c.kubeClientset = kubernetes.NewForConfigOrDie(restConfig)
	}
	return c.kubeClientset
}

func (c *client) lazyLoadClientOrDie() crclient.Client {
	if c.client == nil {
		restConfig := c.lazyLoadRestConfigOrDie()
		client, err := crclient.New(restConfig, crclient.Options{Scheme: c.scheme})
		if err != nil {
			panic(err)
		}
		c.client = client
	}
	return c.client
}

func (c *client) lazyLoadDefaultNamespaceOrDie() string {
	if c.defaultNamespace == "" {
		kubeConfig := c.lazyLoadKubeConfig()
		namespace, _, err := kubeConfig.Namespace()
		if err != nil {
			panic(err)
		}
		c.defaultNamespace = namespace
	}
	return c.defaultNamespace
}

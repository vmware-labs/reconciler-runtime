/*
Copyright 2020 the original author or authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DuckClient interface {
	Client
	DuckReader
	DuckWriter
}

func NewDuckClient(client Client) DuckClient {
	return &duckClient{
		DuckReader: NewDuckReader(client),
		DuckWriter: NewDuckWriter(client),
		client:     client,
	}
}

type duckClient struct {
	DuckReader
	DuckWriter
	client Client
}

func (c *duckClient) Status() client.StatusWriter {
	return c.client.Status()
}

func (c *duckClient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *duckClient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

// DuckReader wraps a Reader adding duck type specific methods for reading
// resources via a duck type
type DuckReader interface {
	Reader

	// GetDuck retrieves an obj for the given key from the Kubernetes Cluster.
	// obj must be a struct pointer so that obj can be updated with the response
	// returned by the Server.
	//
	// Unlike Get, the API version and kind of the resource is not inferred from
	// the obj type, and must be included in the key. The representation from
	// the API server is unmarshaled into the the duck object with no type
	// checking. Incompatible structs will contain their empty values.
	GetDuck(ctx context.Context, key Key, duck client.Object) error

	// ListDuck retrieves list of objects for a given namespace and list
	// options. On a successful call, Items field in the list will be populated
	// with the result returned from the server.
	//
	// Unlike List, the API version and kind of the resource is not inferred
	// from the obj type, and must be included in the key. The representation
	// from the API server is unmarshaled into the the duck object with no type
	// checking. Incompatible structs will contain their empty values.
	ListDuck(ctx context.Context, key Key, duck client.ObjectList, opts ...ListOption) error
}

func NewDuckReader(reader Reader) DuckReader {
	return &duckReader{
		Reader: reader,
	}
}

type duckReader struct {
	Reader
}

func (c *duckReader) GetDuck(ctx context.Context, key Key, duck client.Object) error {
	u := key.UnstructuredObject()
	err := c.Reader.Get(ctx, key.ObjectKey(), u)
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), duck)
}

func (c *duckReader) ListDuck(ctx context.Context, key Key, duck client.ObjectList, opts ...ListOption) error {
	u := key.UnstructuredList()
	if key.Namespace != "" {
		opts = append(opts, InNamespace(key.Namespace))
	}
	err := c.Reader.List(ctx, u, opts...)
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), duck)
}

type DuckWriter interface {
	Writer

	// DeleteDuck deletes the obj referenced by the key from Kubernetes cluster.
	//
	// Unlike Delete, the key is used instead of an object to reference the
	// resource to delete.
	DeleteDuck(ctx context.Context, key Key, opts ...DeleteOption) error

	// PatchDuck patches the given obj in the Kubernetes cluster. obj must be a
	// struct pointer so that obj can be updated with the content returned by
	// the Server.
	PatchDuck(ctx context.Context, duck client.Object, patch Patch, opts ...PatchOption) error
}

func NewDuckWriter(writer Writer) DuckWriter {
	return &duckWriter{
		Writer: writer,
	}
}

type duckWriter struct {
	Writer
}

func (c *duckWriter) DeleteDuck(ctx context.Context, key Key, opts ...DeleteOption) error {
	obj := key.UnstructuredObject()
	return c.Delete(ctx, obj, opts...)
}

func (c *duckWriter) PatchDuck(ctx context.Context, duck client.Object, patch Patch, opts ...PatchOption) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(duck)
	if err != nil {
		return err
	}
	obj := &unstructured.Unstructured{Object: u}
	if err := c.Patch(ctx, obj, patch, opts...); err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), duck)
}

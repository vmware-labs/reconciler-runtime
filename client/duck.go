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

	"k8s.io/apimachinery/pkg/runtime"
)

type DuckClient interface {
	DuckReader
	Writer
	StatusClient
}

func NewDuckClient(client Client) DuckClient {
	return &duckClient{
		DuckReader:   NewDuckReader(client),
		Writer:       client,
		StatusClient: client,
	}
}

type duckClient struct {
	DuckReader
	Writer
	StatusClient
}

type DuckReader interface {
	Reader
	GetDuck(ctx context.Context, key Key, duck runtime.Object) error
	ListDuck(ctx context.Context, key Key, duck runtime.Object, opts ...ListOption) error
}

func NewDuckReader(reader Reader) DuckReader {
	return &duckReader{
		Reader: reader,
	}
}

type duckReader struct {
	Reader
}

func (c *duckReader) GetDuck(ctx context.Context, key Key, duck runtime.Object) error {
	u := key.Unstructured()
	err := c.Reader.Get(ctx, key.ObjectKey(), u)
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.
		FromUnstructured(u.UnstructuredContent(), duck)
}

func (c *duckReader) ListDuck(ctx context.Context, key Key, duck runtime.Object, opts ...ListOption) error {
	u := key.Unstructured()
	if key.Namespace != "" {
		opts = append(opts, InNamespace(key.Namespace))
	}
	err := c.Reader.List(ctx, u, opts...)
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.
		FromUnstructured(u.UnstructuredContent(), duck)
}

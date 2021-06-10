/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"context"
	"fmt"
	"io"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type DryRunable interface {
	IsDryRun() bool
}

func DryRunResource(ctx context.Context, resource runtime.Object, gvk schema.GroupVersionKind) {
	stdout := stdoutFromContext(ctx)
	resource = defaultTypeMeta(resource, gvk)
	b, _ := yaml.Marshal(resource)
	fmt.Fprintf(stdout, "---\n%s", b)
}

func defaultTypeMeta(resource runtime.Object, gvk schema.GroupVersionKind) runtime.Object {
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	tm := metav1.TypeMeta{
		APIVersion: apiVersion,
		Kind:       kind,
	}
	reflect.ValueOf(resource).Elem().FieldByName("TypeMeta").Set(reflect.ValueOf(tm))
	return resource
}

type stdoutKey struct{}

func withStdout(ctx context.Context, stdout io.Writer) context.Context {
	return context.WithValue(ctx, stdoutKey{}, stdout)
}

func stdoutFromContext(ctx context.Context) io.Writer {
	stdout, _ := ctx.Value(stdoutKey{}).(io.Writer)
	return stdout
}

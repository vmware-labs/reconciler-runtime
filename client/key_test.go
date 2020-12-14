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

package client_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewKey(t *testing.T) {
	expected := client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	actual := client.NewKey(corev1.ObjectReference{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	})
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

func TestNewKeyNamespaced(t *testing.T) {
	expected := client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	actual := client.NewKeyNamespaced(corev1.ObjectReference{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Name:       "my-resource",
	}, "my-namespace")
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

func TestKey_ObjectKey(t *testing.T) {
	key := &client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	expected := client.ObjectKey{
		Namespace: "my-namespace",
		Name:      "my-resource",
	}
	actual := key.ObjectKey()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

func TestKey_ObjectReference(t *testing.T) {
	key := &client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	expected := corev1.ObjectReference{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	actual := key.ObjectReference()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

func TestKey_GroupVersionKind(t *testing.T) {
	key := &client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	expected := schema.GroupVersionKind{
		Group:   "example.com",
		Version: "v1alpha1",
		Kind:    "MyKind",
	}
	actual := key.GroupVersionKind()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

func TestKey_UnstructuredObject(t *testing.T) {
	key := &client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
		Name:       "my-resource",
	}
	expected := &unstructured.Unstructured{}
	expected.SetAPIVersion("example.com/v1alpha1")
	expected.SetKind("MyKind")
	actual := key.UnstructuredObject()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

func TestKey_UnstructuredList(t *testing.T) {
	key := &client.Key{
		APIVersion: "example.com/v1alpha1",
		Kind:       "MyKind",
		Namespace:  "my-namespace",
	}
	expected := &unstructured.UnstructuredList{}
	expected.SetAPIVersion("example.com/v1alpha1")
	expected.SetKind("MyKind")
	actual := key.UnstructuredList()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual): %s", diff)
	}
}

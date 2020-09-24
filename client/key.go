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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewKey(ref corev1.ObjectReference) Key {
	return Key{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
	}
}

func NewKeyNamespaced(ref corev1.ObjectReference, namespace string) Key {
	key := NewKey(ref)
	key.Namespace = namespace
	return key
}

type Key struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

func (k *Key) ObjectKey() ObjectKey {
	return ObjectKey{
		Namespace: k.Namespace,
		Name:      k.Name,
	}
}

func (k *Key) ObjectReference() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: k.APIVersion,
		Kind:       k.Kind,
		Namespace:  k.Namespace,
		Name:       k.Name,
	}
}

func (k *Key) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(k.APIVersion, k.Kind)
}

func (k *Key) Unstructured() runtime.Unstructured {
	if k.Name == "" {
		u := &unstructured.UnstructuredList{}
		u.SetAPIVersion(k.APIVersion)
		u.SetKind(k.Kind)
		return u
	}
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(k.APIVersion)
	u.SetKind(k.Kind)
	return u
}

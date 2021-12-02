/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Deprecated Factory creates Kubernetes objects
type Factory interface {
	// CreateObject creates a new Kubernetes object
	CreateObject() client.Object
}

// Deprecated
func Wrapper(obj client.Object) Factory {
	return &wrapper{obj: obj}
}

type wrapper struct {
	obj client.Object
}

func (w *wrapper) CreateObject() client.Object {
	return w.obj.DeepCopyObject().(client.Object)
}

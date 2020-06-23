/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"github.com/vmware-labs/reconciler-runtime/apis"
)

// Factory creates Kubernetes objects
type Factory interface {
	// CreateObject creates a new Kubernetes object
	CreateObject() apis.Object
}

func Wrapper(obj apis.Object) Factory {
	return &wrapper{obj: obj}
}

type wrapper struct {
	obj apis.Object
}

func (w *wrapper) CreateObject() apis.Object {
	return w.obj.DeepCopyObject().(apis.Object)
}

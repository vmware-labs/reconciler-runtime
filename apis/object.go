/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package apis

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Object interface {
	runtime.Object
	metav1.Object
}

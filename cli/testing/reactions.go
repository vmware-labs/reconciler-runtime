/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/validation"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
)

var InduceFailure = rtesting.InduceFailure

func ValidateCreates(ctx context.Context, action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
	got := action.(clientgotesting.CreateAction).GetObject()
	obj, ok := got.(validation.FieldValidator)
	if !ok {
		return false, nil, nil
	}
	if err := obj.Validate(); err != nil {
		return true, nil, err.ToAggregate()
	}
	return false, nil, nil
}

func ValidateUpdates(ctx context.Context, action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
	got := action.(clientgotesting.UpdateAction).GetObject()
	obj, ok := got.(validation.FieldValidator)
	if !ok {
		return false, nil, nil
	}
	if err := obj.Validate(); err != nil {
		return true, nil, err.ToAggregate()
	}
	return false, nil, nil
}

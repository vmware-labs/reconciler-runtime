/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parent

import (
	"context"

	"github.com/vmware-labs/reconciler-runtime/reconcilers/stash"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const parentReconcilerStashKey stash.StashKey = "reconciler-runtime:parentReconciler"

// StashParentReconciler stashes the given parent reconciler in the given context.
func StashParentReconciler(ctx context.Context, parent reconcile.Reconciler) context.Context {
	return context.WithValue(ctx, parentReconcilerStashKey, parent)
}

// RetrieveParentReconciler retrieves any parent reconciler from the given context.
func RetrieveParentReconciler(ctx context.Context) reconcile.Reconciler {
	value := ctx.Value(parentReconcilerStashKey)
	if parentReconciler, ok := value.(reconcile.Reconciler); ok {
		return parentReconciler
	}
	return nil
}

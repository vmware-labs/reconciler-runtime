/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ SubReconciler[client.Object] = (Sequence[client.Object])(nil)
)

// Sequence is a collection of SubReconcilers called in order. If a
// reconciler errs, further reconcilers are skipped.
type Sequence[Type client.Object] []SubReconciler[Type]

func (r Sequence[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	for i, reconciler := range r {
		log := logr.FromContextOrDiscard(ctx).
			WithName(fmt.Sprintf("%d", i))
		ctx = logr.NewContext(ctx, log)

		err := reconciler.SetupWithManager(ctx, mgr, bldr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Sequence[T]) Reconcile(ctx context.Context, resource T) (Result, error) {
	aggregateResult := Result{}
	for i, reconciler := range r {
		log := logr.FromContextOrDiscard(ctx).
			WithName(fmt.Sprintf("%d", i))
		ctx = logr.NewContext(ctx, log)

		result, err := reconciler.Reconcile(ctx, resource)
		aggregateResult = AggregateResults(result, aggregateResult)
		if err != nil {
			return result, err
		}
	}

	return aggregateResult, nil
}

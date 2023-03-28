/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/vmware-labs/reconciler-runtime/tracker"
)

func EnqueueTracked(ctx context.Context, by client.Object) handler.EventHandler {
	c := RetrieveConfigOrDie(ctx)
	return handler.EnqueueRequestsFromMapFunc(
		func(a client.Object) []Request {
			var requests []Request

			gvks, _, err := c.Scheme().ObjectKinds(by)
			if err != nil {
				panic(err)
			}

			key := tracker.NewKey(
				gvks[0],
				types.NamespacedName{Namespace: a.GetNamespace(), Name: a.GetName()},
			)
			for _, item := range c.Tracker.Lookup(ctx, key) {
				requests = append(requests, Request{NamespacedName: item})
			}

			return requests
		},
	)
}

/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

func EnqueueTracked(ctx context.Context) handler.EventHandler {
	c := RetrieveConfigOrDie(ctx)
	log := logr.FromContextOrDiscard(ctx)

	return handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []Request {
			var requests []Request

			items, err := c.Tracker.GetObservers(obj)
			if err != nil {
				log.Error(err, "unable to get tracked requests")
				return nil
			}

			for _, item := range items {
				requests = append(requests, Request{NamespacedName: item})
			}

			return requests
		},
	)
}

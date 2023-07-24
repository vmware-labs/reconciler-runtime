/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"fmt"
)

const stashNonce string = "controller-stash-nonce"

type stashMap map[StashKey]interface{}

func WithStash(ctx context.Context) context.Context {
	return context.WithValue(ctx, stashNonce, stashMap{})
}

type StashKey string

func retrieveStashMap(ctx context.Context) stashMap {
	stash, ok := ctx.Value(stashNonce).(stashMap)
	if !ok {
		panic(fmt.Errorf("context not configured for stashing, call `ctx = WithStash(ctx)`"))
	}
	return stash
}

func StashValue(ctx context.Context, key StashKey, value interface{}) {
	stash := retrieveStashMap(ctx)
	stash[key] = value
}

func RetrieveValue(ctx context.Context, key StashKey) interface{} {
	stash := retrieveStashMap(ctx)
	return stash[key]
}

func ClearValue(ctx context.Context, key StashKey) interface{} {
	stash := retrieveStashMap(ctx)
	value := stash[key]
	delete(stash, key)
	return value
}

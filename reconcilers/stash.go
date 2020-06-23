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

func StashValue(ctx context.Context, key StashKey, value interface{}) {
	stash, ok := ctx.Value(stashNonce).(stashMap)
	if !ok {
		panic(fmt.Errorf("context not configured for stashing, call `ctx = WithStash(ctx)`"))
	}
	stash[key] = value
}

func RetrieveValue(ctx context.Context, key StashKey) interface{} {
	stash, ok := ctx.Value(stashNonce).(stashMap)
	if !ok {
		panic(fmt.Errorf("context not configured for stashing, call `ctx = WithStash(ctx)`"))
	}
	return stash[key]
}

/*
Copyright 2023 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package time

import (
	"context"
	"time"
)

type stashKey struct{}

var nowStashKey = stashKey{}

func StashNow(ctx context.Context, now time.Time) context.Context {
	if ctx.Value(nowStashKey) != nil {
		// avoid overwriting
		return ctx
	}
	return context.WithValue(ctx, nowStashKey, &now)
}

// RetrieveNow returns the stashed time, or the current time if not found.
func RetrieveNow(ctx context.Context) time.Time {
	value := ctx.Value(nowStashKey)
	if now, ok := value.(*time.Time); ok {
		return *now
	}
	return time.Now()
}

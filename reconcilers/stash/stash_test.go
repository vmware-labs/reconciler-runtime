/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package stash

import (
	"testing"

	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/api/equality"
)

func TestStash(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "string",
			value: "value",
		},
		{
			name:  "int",
			value: 42,
		},
		{
			name:  "map",
			value: map[string]string{"foo": "bar"},
		},
		{
			name:  "nil",
			value: nil,
		},
	}

	var key StashKey = "stash-key"
	ctx := WithStash(context.TODO())
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			StashValue(ctx, key, c.value)
			if expected, actual := c.value, RetrieveValue(ctx, key); !equality.Semantic.DeepEqual(expected, actual) {
				t.Errorf("%s: unexpected stash value, actually = %v, expected = %v", c.name, actual, expected)
			}
		})
	}
}

func TestStash_StashValue_UndecoratedContext(t *testing.T) {
	ctx := context.TODO()
	var key StashKey = "stash-key"
	value := "value"

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected StashValue() to panic")
		}
	}()
	StashValue(ctx, key, value)
}

func TestStash_RetrieveValue_UndecoratedContext(t *testing.T) {
	ctx := context.TODO()
	var key StashKey = "stash-key"

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected RetrieveValue() to panic")
		}
	}()
	RetrieveValue(ctx, key)
}

func TestStash_RetrieveValue_Undefined(t *testing.T) {
	ctx := WithStash(context.TODO())
	var key StashKey = "stash-key"

	if value := RetrieveValue(ctx, key); value != nil {
		t.Error("expected RetrieveValue() to return nil for undefined key")
	}
}

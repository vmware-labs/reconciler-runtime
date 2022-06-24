/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
)

type Reactor = clientgotesting.Reactor
type ReactionFunc = clientgotesting.ReactionFunc

type Action = clientgotesting.Action
type GetAction = clientgotesting.GetAction
type ListAction = clientgotesting.ListAction
type CreateAction = clientgotesting.CreateAction
type UpdateAction = clientgotesting.UpdateAction
type PatchAction = clientgotesting.PatchAction
type DeleteAction = clientgotesting.DeleteAction
type DeleteCollectionAction = clientgotesting.DeleteCollectionAction

var (
	// Deprecated use pointer.String
	StringPtr = pointer.String
	// Deprecated use pointer.Bool
	BoolPtr = pointer.Bool
	// Deprecated use pointer.Int32
	Int32Ptr = pointer.Int32
	// Deprecated use pointer.Int64
	Int64Ptr = pointer.Int64
)

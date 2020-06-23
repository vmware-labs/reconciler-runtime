/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	clientgotesting "k8s.io/client-go/testing"
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

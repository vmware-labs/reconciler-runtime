/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
)

type ReactionFunc = rtesting.ReactionFunc
type Factory = rtesting.Factory

var NewFakeClient = rtesting.NewFakeClient
var Wrapper = rtesting.Wrapper

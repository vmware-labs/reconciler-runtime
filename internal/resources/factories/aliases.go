/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"github.com/vmware-labs/reconciler-runtime/testing/factories"
)

type ConditionFactory = factories.ConditionFactory
type ObjectMeta = factories.ObjectMeta
type PodTemplateSpec = factories.PodTemplateSpec

var Condition = factories.Condition
var ObjectMetaFactory = factories.ObjectMetaFactory
var PodTemplateSpecFactory = factories.PodTemplateSpecFactory
var ConfigMap = factories.ConfigMap

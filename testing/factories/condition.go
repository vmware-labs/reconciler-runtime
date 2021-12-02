/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/apis"
	corev1 "k8s.io/api/core/v1"
)

// Deprecated
type ConditionFactory interface {
	Create() apis.Condition

	Type(t apis.ConditionType) ConditionFactory
	Unknown() ConditionFactory
	True() ConditionFactory
	False() ConditionFactory
	Reason(reason, message string) ConditionFactory
	Info() ConditionFactory
	Warning() ConditionFactory
	Error() ConditionFactory
}

type conditionImpl struct {
	target *apis.Condition
}

// Deprecated
func Condition(seed ...apis.Condition) ConditionFactory {
	var target *apis.Condition
	switch len(seed) {
	case 0:
		target = &apis.Condition{}
	case 1:
		target = &seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &conditionImpl{
		target: target,
	}
}

func (f *conditionImpl) deepCopy() *conditionImpl {
	return &conditionImpl{
		target: f.target.DeepCopy(),
	}
}

func (f *conditionImpl) Create() apis.Condition {
	t := f.deepCopy().target
	return *t
}

func (f *conditionImpl) mutation(m func(*apis.Condition)) ConditionFactory {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *conditionImpl) Type(t apis.ConditionType) ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Type = t
	})
}

func (f *conditionImpl) Unknown() ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Status = corev1.ConditionUnknown
	})
}

func (f *conditionImpl) True() ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Status = corev1.ConditionTrue
		c.Reason = ""
		c.Message = ""
	})
}

func (f *conditionImpl) False() ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Status = corev1.ConditionFalse
	})
}

func (f *conditionImpl) Reason(reason, message string) ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Reason = reason
		c.Message = message
	})
}

func (f *conditionImpl) Info() ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Severity = apis.ConditionSeverityInfo
	})
}

func (f *conditionImpl) Warning() ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Severity = apis.ConditionSeverityWarning
	})
}

func (f *conditionImpl) Error() ConditionFactory {
	return f.mutation(func(c *apis.Condition) {
		c.Severity = apis.ConditionSeverityError
	})
}

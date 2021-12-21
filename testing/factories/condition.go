/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Deprecated
type ConditionFactory interface {
	Create() metav1.Condition

	Type(t string) ConditionFactory
	Unknown() ConditionFactory
	True() ConditionFactory
	False() ConditionFactory
	Reason(reason, message string) ConditionFactory
}

type conditionImpl struct {
	target *metav1.Condition
}

// Deprecated
func Condition(seed ...metav1.Condition) ConditionFactory {
	var target *metav1.Condition
	switch len(seed) {
	case 0:
		target = &metav1.Condition{}
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

func (f *conditionImpl) Create() metav1.Condition {
	t := f.deepCopy().target
	return *t
}

func (f *conditionImpl) mutation(m func(*metav1.Condition)) ConditionFactory {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *conditionImpl) Type(t string) ConditionFactory {
	return f.mutation(func(c *metav1.Condition) {
		c.Type = t
	})
}

func (f *conditionImpl) Unknown() ConditionFactory {
	return f.mutation(func(c *metav1.Condition) {
		c.Status = metav1.ConditionUnknown
	})
}

func (f *conditionImpl) True() ConditionFactory {
	return f.mutation(func(c *metav1.Condition) {
		c.Status = metav1.ConditionTrue
	})
}

func (f *conditionImpl) False() ConditionFactory {
	return f.mutation(func(c *metav1.Condition) {
		c.Status = metav1.ConditionFalse
	})
}

func (f *conditionImpl) Reason(reason, message string) ConditionFactory {
	return f.mutation(func(c *metav1.Condition) {
		c.Reason = reason
		c.Message = message
	})
}

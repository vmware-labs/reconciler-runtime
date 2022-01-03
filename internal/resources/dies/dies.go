/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package dies

import (
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +die:object=true
type _ = resources.TestResource

// +die
type _ = resources.TestResourceSpec

func (d *TestResourceSpecDie) AddField(key, value string) *TestResourceSpecDie {
	return d.DieStamp(func(r *resources.TestResourceSpec) {
		if r.Fields == nil {
			r.Fields = map[string]string{}
		}
		r.Fields[key] = value
	})
}

func (d *TestResourceSpecDie) TemplateDie(fn func(d *diecorev1.PodTemplateSpecDie)) *TestResourceSpecDie {
	return d.DieStamp(func(r *resources.TestResourceSpec) {
		d := diecorev1.PodTemplateSpecBlank.DieImmutable(false).DieFeed(r.Template)
		fn(d)
		r.Template = d.DieRelease()
	})
}

// +die
type _ = resources.TestResourceStatus

func (d *TestResourceStatusDie) ConditionsDie(conditions ...*diemetav1.ConditionDie) *TestResourceStatusDie {
	return d.DieStamp(func(r *resources.TestResourceStatus) {
		r.Conditions = make([]metav1.Condition, len(conditions))
		for i := range conditions {
			r.Conditions[i] = conditions[i].DieRelease()
		}
	})
}

func (d *TestResourceStatusDie) AddField(key, value string) *TestResourceStatusDie {
	return d.DieStamp(func(r *resources.TestResourceStatus) {
		if r.Fields == nil {
			r.Fields = map[string]string{}
		}
		r.Fields[key] = value
	})
}

// +die:object=true
type _ = resources.TestResourceNoStatus

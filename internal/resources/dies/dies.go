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

// +die:object=true,spec=TestResourceSpec
type _ = resources.TestResourceEmptyStatus

// +die
type _ = resources.TestResourceEmptyStatusStatus

// +die:object=true,spec=TestResourceSpec
type _ = resources.TestResourceNoStatus

// +die:object=true,spec=TestResourceSpec
type _ = resources.TestResourceNilableStatus

// StatusDie stamps the resource's status field with a mutable die.
func (d *TestResourceNilableStatusDie) StatusDie(fn func(d *TestResourceStatusDie)) *TestResourceNilableStatusDie {
	return d.DieStamp(func(r *resources.TestResourceNilableStatus) {
		d := TestResourceStatusBlank.DieImmutable(false).DieFeedPtr(r.Status)
		fn(d)
		r.Status = d.DieReleasePtr()
	})
}

// +die:object=true
type _ = resources.TestDuck

func (d *TestDuckDie) StatusDie(fn func(d *TestResourceStatusDie)) *TestDuckDie {
	return d.DieStamp(func(r *resources.TestDuck) {
		d := TestResourceStatusBlank.DieImmutable(false).DieFeed(r.Status)
		fn(d)
		r.Status = d.DieRelease()
	})
}

// +die
type _ = resources.TestDuckSpec

func (d *TestDuckSpecDie) AddField(key, value string) *TestDuckSpecDie {
	return d.DieStamp(func(r *resources.TestDuckSpec) {
		if r.Fields == nil {
			r.Fields = map[string]string{}
		}
		r.Fields[key] = value
	})
}

// +die:object=true
type _ = resources.TestResourceUnexportedFields

// +die:ignore={unexportedFields}
type _ = resources.TestResourceUnexportedFieldsSpec

func (d *TestResourceUnexportedFieldsSpecDie) AddField(key, value string) *TestResourceUnexportedFieldsSpecDie {
	return d.DieStamp(func(r *resources.TestResourceUnexportedFieldsSpec) {
		if r.Fields == nil {
			r.Fields = map[string]string{}
		}
		r.Fields[key] = value
	})
}

func (d *TestResourceUnexportedFieldsSpecDie) AddUnexportedField(key, value string) *TestResourceUnexportedFieldsSpecDie {
	return d.DieStamp(func(r *resources.TestResourceUnexportedFieldsSpec) {
		f := r.GetUnexportedFields()
		if f == nil {
			f = map[string]string{}
		}
		f[key] = value
		r.SetUnexportedFields(f)
	})
}

func (d *TestResourceUnexportedFieldsSpecDie) TemplateDie(fn func(d *diecorev1.PodTemplateSpecDie)) *TestResourceUnexportedFieldsSpecDie {
	return d.DieStamp(func(r *resources.TestResourceUnexportedFieldsSpec) {
		d := diecorev1.PodTemplateSpecBlank.DieImmutable(false).DieFeed(r.Template)
		fn(d)
		r.Template = d.DieRelease()
	})
}

// +die:ignore={unexportedFields}
type _ = resources.TestResourceUnexportedFieldsStatus

func (d *TestResourceUnexportedFieldsStatusDie) ConditionsDie(conditions ...*diemetav1.ConditionDie) *TestResourceUnexportedFieldsStatusDie {
	return d.DieStamp(func(r *resources.TestResourceUnexportedFieldsStatus) {
		r.Conditions = make([]metav1.Condition, len(conditions))
		for i := range conditions {
			r.Conditions[i] = conditions[i].DieRelease()
		}
	})
}

func (d *TestResourceUnexportedFieldsStatusDie) AddField(key, value string) *TestResourceUnexportedFieldsStatusDie {
	return d.DieStamp(func(r *resources.TestResourceUnexportedFieldsStatus) {
		if r.Fields == nil {
			r.Fields = map[string]string{}
		}
		r.Fields[key] = value
	})
}

func (d *TestResourceUnexportedFieldsStatusDie) AddUnexportedField(key, value string) *TestResourceUnexportedFieldsStatusDie {
	return d.DieStamp(func(r *resources.TestResourceUnexportedFieldsStatus) {
		f := r.GetUnexportedFields()
		if f == nil {
			f = map[string]string{}
		}
		f[key] = value
		r.SetUnexportedFields(f)
	})
}

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_ client.Object = &TestDuck{}
)

// +kubebuilder:object:root=true
// +genclient

type TestDuck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestDuckSpec       `json:"spec"`
	Status TestResourceStatus `json:"status"`
}

func (r *TestDuck) Default() {
	if r.Spec.Fields == nil {
		r.Spec.Fields = map[string]string{}
	}
	r.Spec.Fields["Defaulter"] = "ran"
}

func (r *TestDuck) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

func (r *TestDuck) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

func (r *TestDuck) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *TestDuck) validate() field.ErrorList {
	errs := field.ErrorList{}

	if r.Spec.Fields != nil {
		if _, ok := r.Spec.Fields["invalid"]; ok {
			field.Invalid(field.NewPath("spec", "fields", "invalid"), r.Spec.Fields["invalid"], "")
		}
	}

	return errs
}

// +kubebuilder:object:generate=true
type TestDuckSpec struct {
	Fields map[string]string `json:"fields,omitempty"`
}

// +kubebuilder:object:root=true

type TestDuckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestDuck `json:"items"`
}

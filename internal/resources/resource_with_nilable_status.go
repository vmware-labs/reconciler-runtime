/*
Copyright 2022 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_ webhook.Defaulter = &TestResourceNilableStatus{}
	_ webhook.Validator = &TestResourceNilableStatus{}
	_ client.Object     = &TestResourceNilableStatus{}
)

// +kubebuilder:object:root=true
// +genclient

type TestResourceNilableStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestResourceSpec    `json:"spec"`
	Status *TestResourceStatus `json:"status"`
}

func (r *TestResourceNilableStatus) Default() {
	if r.Spec.Fields == nil {
		r.Spec.Fields = map[string]string{}
	}
	r.Spec.Fields["Defaulter"] = "ran"
}

func (r *TestResourceNilableStatus) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

func (r *TestResourceNilableStatus) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

func (r *TestResourceNilableStatus) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *TestResourceNilableStatus) validate() field.ErrorList {
	errs := field.ErrorList{}

	if r.Spec.Fields != nil {
		if _, ok := r.Spec.Fields["invalid"]; ok {
			field.Invalid(field.NewPath("spec", "fields", "invalid"), r.Spec.Fields["invalid"], "")
		}
	}

	return errs
}

// +kubebuilder:object:root=true

type TestResourceNilableStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestResourceNilableStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TestResourceNilableStatus{}, &TestResourceNilableStatusList{})
}

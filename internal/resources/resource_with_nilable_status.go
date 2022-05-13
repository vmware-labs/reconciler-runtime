/*
Copyright 2022 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	"github.com/vmware-labs/reconciler-runtime/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	_ webhook.Defaulter         = &TestResourceNilableStatus{}
	_ client.Object             = &TestResourceNilableStatus{}
	_ validation.FieldValidator = &TestResourceNilableStatus{}
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

func (r *TestResourceNilableStatus) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}

	if r.Spec.Fields != nil {
		if _, ok := r.Spec.Fields["invalid"]; ok {
			errs = errs.Also(validation.ErrInvalidValue(r.Spec.Fields["invalid"], "spec.fields.invalid"))
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

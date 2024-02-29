/*
Copyright 2022 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	"encoding/json"
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_ webhook.Defaulter = &TestResourceUnexportedFields{}
	_ webhook.Validator = &TestResourceUnexportedFields{}
	_ client.Object     = &TestResourceUnexportedFields{}
)

// +kubebuilder:object:root=true
// +genclient

type TestResourceUnexportedFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestResourceUnexportedFieldsSpec   `json:"spec"`
	Status TestResourceUnexportedFieldsStatus `json:"status"`
}

func (r *TestResourceUnexportedFields) Default() {
	if r.Spec.Fields == nil {
		r.Spec.Fields = map[string]string{}
	}
	r.Spec.Fields["Defaulter"] = "ran"
}

func (r *TestResourceUnexportedFields) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

func (r *TestResourceUnexportedFields) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

func (r *TestResourceUnexportedFields) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *TestResourceUnexportedFields) validate() field.ErrorList {
	errs := field.ErrorList{}

	if r.Spec.Fields != nil {
		if _, ok := r.Spec.Fields["invalid"]; ok {
			field.Invalid(field.NewPath("spec", "fields", "invalid"), r.Spec.Fields["invalid"], "")
		}
	}

	return errs
}

func (r *TestResourceUnexportedFields) CopyUnexportedFields() {
	r.Status.unexportedFields = r.Spec.unexportedFields
}

// +kubebuilder:object:generate=true
type TestResourceUnexportedFieldsSpec struct {
	Fields           map[string]string `json:"fields,omitempty"`
	unexportedFields map[string]string
	Template         corev1.PodTemplateSpec `json:"template,omitempty"`

	ErrOnMarshal   bool `json:"errOnMarhsal,omitempty"`
	ErrOnUnmarshal bool `json:"errOnUnmarhsal,omitempty"`
}

func (r *TestResourceUnexportedFieldsSpec) GetUnexportedFields() map[string]string {
	return r.unexportedFields
}

func (r *TestResourceUnexportedFieldsSpec) SetUnexportedFields(f map[string]string) {
	r.unexportedFields = f
}

func (r *TestResourceUnexportedFieldsSpec) AddUnexportedFields(key, value string) {
	if r.unexportedFields == nil {
		r.unexportedFields = map[string]string{}
	}
	r.unexportedFields[key] = value
}

func (r *TestResourceUnexportedFieldsSpec) MarshalJSON() ([]byte, error) {
	if r.ErrOnMarshal {
		return nil, fmt.Errorf("ErrOnMarshal true")
	}
	return json.Marshal(&struct {
		Fields         map[string]string      `json:"fields,omitempty"`
		Template       corev1.PodTemplateSpec `json:"template,omitempty"`
		ErrOnMarshal   bool                   `json:"errOnMarshal,omitempty"`
		ErrOnUnmarshal bool                   `json:"errOnUnmarshal,omitempty"`
	}{
		Fields:         r.Fields,
		Template:       r.Template,
		ErrOnMarshal:   r.ErrOnMarshal,
		ErrOnUnmarshal: r.ErrOnUnmarshal,
	})
}

func (r *TestResourceUnexportedFieldsSpec) UnmarshalJSON(data []byte) error {
	type alias struct {
		Fields         map[string]string      `json:"fields,omitempty"`
		Template       corev1.PodTemplateSpec `json:"template,omitempty"`
		ErrOnMarshal   bool                   `json:"errOnMarshal,omitempty"`
		ErrOnUnmarshal bool                   `json:"errOnUnmarshal,omitempty"`
	}
	a := &alias{}
	if err := json.Unmarshal(data, a); err != nil {
		return err
	}
	r.Fields = a.Fields
	r.Template = a.Template
	r.ErrOnMarshal = a.ErrOnMarshal
	r.ErrOnUnmarshal = a.ErrOnUnmarshal
	if r.ErrOnUnmarshal {
		return fmt.Errorf("ErrOnUnmarshal true")
	}
	return nil
}

// +kubebuilder:object:generate=true
type TestResourceUnexportedFieldsStatus struct {
	apis.Status      `json:",inline"`
	Fields           map[string]string `json:"fields,omitempty"`
	unexportedFields map[string]string
}

func (r *TestResourceUnexportedFieldsStatus) GetUnexportedFields() map[string]string {
	return r.unexportedFields
}

func (r *TestResourceUnexportedFieldsStatus) SetUnexportedFields(f map[string]string) {
	r.unexportedFields = f
}

func (r *TestResourceUnexportedFieldsStatus) AddUnexportedFields(key, value string) {
	if r.unexportedFields == nil {
		r.unexportedFields = map[string]string{}
	}
	r.unexportedFields[key] = value
}

// +kubebuilder:object:root=true

type TestResourceUnexportedFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestResourceUnexportedFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TestResourceUnexportedFields{}, &TestResourceUnexportedFieldsList{})
}

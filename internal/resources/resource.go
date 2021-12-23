/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	"encoding/json"
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	_ webhook.Defaulter         = &TestResource{}
	_ client.Object             = &TestResource{}
	_ validation.FieldValidator = &TestResource{}
)

// +kubebuilder:object:root=true
// +genclient

type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestResourceSpec   `json:"spec"`
	Status TestResourceStatus `json:"status"`
}

func (r *TestResource) Default() {
	if r.Spec.Fields == nil {
		r.Spec.Fields = map[string]string{}
	}
	r.Spec.Fields["Defaulter"] = "ran"
}

func (r *TestResource) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}

	if r.Spec.Fields != nil {
		if _, ok := r.Spec.Fields["invalid"]; ok {
			errs = errs.Also(validation.ErrInvalidValue(r.Spec.Fields["invalid"], "spec.fields.invalid"))
		}
	}

	return errs
}

// +kubebuilder:object:generate=true
type TestResourceSpec struct {
	Fields   map[string]string      `json:"fields,omitempty"`
	Template corev1.PodTemplateSpec `json:"template,omitempty"`

	ErrOnMarshal   bool `json:"errOnMarhsal,omitempty"`
	ErrOnUnmarshal bool `json:"errOnUnmarhsal,omitempty"`
}

func (r *TestResourceSpec) MarshalJSON() ([]byte, error) {
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

func (r *TestResourceSpec) UnmarshalJSON(data []byte) error {
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
type TestResourceStatus struct {
	apis.Status `json:",inline"`
	Fields      map[string]string `json:"fields,omitempty"`
}

func (rs *TestResourceStatus) InitializeConditions() {
	rs.SetConditions([]metav1.Condition{
		{
			Type:               apis.ConditionReady,
			Status:             metav1.ConditionUnknown,
			Reason:             "Initializing",
			LastTransitionTime: metav1.Now(),
		},
	})
}

func (rs *TestResourceStatus) MarkReady() {
	rs.SetConditions([]metav1.Condition{
		{
			Type:               apis.ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Ready",
			LastTransitionTime: metav1.Now(),
		},
	})
}

func (rs *TestResourceStatus) MarkNotReady(reason, message string, messageA ...interface{}) {
	rs.SetConditions([]metav1.Condition{
		{
			Type:               apis.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            fmt.Sprintf(message, messageA...),
			LastTransitionTime: metav1.Now(),
		},
	})
}

// +kubebuilder:object:root=true

type TestResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestResource `json:"items"`
}

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "testing.reconciler.runtime", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// compatibility with k8s.io/code-generator
var SchemeGroupVersion = GroupVersion

func init() {
	SchemeBuilder.Register(&TestResource{}, &TestResourceList{})
}

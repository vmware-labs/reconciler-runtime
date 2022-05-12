/*
Copyright 2022 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Defaulter = &TestResourceEmptyStatus{}

// +kubebuilder:object:root=true
// +genclient

type TestResourceEmptyStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestResourceSpec              `json:"spec"`
	Status TestResourceEmptyStatusStatus `json:"status"`
}

func (r *TestResourceEmptyStatus) Default() {
	if r.Spec.Fields == nil {
		r.Spec.Fields = map[string]string{}
	}
	r.Spec.Fields["Defaulter"] = "ran"
}

// +kubebuilder:object:generate=true
type TestResourceEmptyStatusStatus struct {
}

// +kubebuilder:object:root=true

type TestResourceEmptyStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestResourceNoStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TestResourceEmptyStatus{}, &TestResourceEmptyStatusList{})
}

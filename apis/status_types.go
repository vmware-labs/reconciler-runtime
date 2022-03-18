/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
/*
Copyright 2019-2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package apis

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Status is the minimally expected status subresource. Use this or provide your own. It also shows how Conditions are
// expected to be embedded in the Status field.
//
// Example:
//   type MyResourceStatus struct {
//   	apis.Status `json:",inline"`
//   	UsefulMessage string `json:"usefulMessage,omitempty"`
//   }
//
// WARNING: Adding fields to this struct will add them to all resources.
// +k8s:deepcopy-gen=true
type Status struct {
	// ObservedGeneration is the 'Generation' of the resource that
	// was last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions the latest available observations of a resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

var _ ConditionsAccessor = (*Status)(nil)

// GetConditions implements ConditionsAccessor
func (s *Status) GetConditions() []metav1.Condition {
	return s.Conditions
}

// SetConditions implements ConditionsAccessor
func (s *Status) SetConditions(c []metav1.Condition) {
	s.Conditions = c
}

// GetCondition fetches the condition of the specified type.
func (s *Status) GetCondition(t string) *metav1.Condition {
	for _, cond := range s.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

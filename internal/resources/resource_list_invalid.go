/*
Copyright 2024 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

type TestResourceInvalidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []string `json:"items"`
}

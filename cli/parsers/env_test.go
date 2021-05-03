/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parsers_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/cli/parsers"
	corev1 "k8s.io/api/core/v1"
)

func TestEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		expected corev1.EnvVar
		value    string
	}{{
		name:  "valid",
		value: "MY_VAR=my-value",
		expected: corev1.EnvVar{
			Name:  "MY_VAR",
			Value: "my-value",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := parsers.EnvVar(test.value)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

func TestEnvVarFrom(t *testing.T) {
	tests := []struct {
		name     string
		expected corev1.EnvVar
		value    string
	}{{
		name:  "configmap",
		value: "MY_VAR=configMapKeyRef:my-configmap:my-key",
		expected: corev1.EnvVar{
			Name: "MY_VAR",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-configmap",
					},
					Key: "my-key",
				},
			},
		},
	}, {
		name:  "secret",
		value: "MY_VAR=secretKeyRef:my-secret:my-key",
		expected: corev1.EnvVar{
			Name: "MY_VAR",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-secret",
					},
					Key: "my-key",
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := test.expected
			actual := parsers.EnvVarFrom(test.value)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("%s() = (-expected, +actual): %s", test.name, diff)
			}
		})
	}
}

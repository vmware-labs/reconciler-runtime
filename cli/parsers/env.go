/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parsers

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func EnvVar(str string) corev1.EnvVar {
	parts := KeyValue(str)

	return corev1.EnvVar{
		Name:  parts[0],
		Value: parts[1],
	}
}

func EnvVarFrom(str string) corev1.EnvVar {
	parts := KeyValue(str)
	source := strings.SplitN(parts[1], ":", 3)

	envvar := corev1.EnvVar{
		Name: parts[0],
	}
	if source[0] == "secretKeyRef" {
		envvar.ValueFrom = &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: source[1],
				},
				Key: source[2],
			},
		}
	}
	if source[0] == "configMapKeyRef" {
		envvar.ValueFrom = &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: source[1],
				},
				Key: source[2],
			},
		}
	}

	return envvar
}

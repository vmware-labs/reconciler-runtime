/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package parsers

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func ObjectReference(str string) corev1.ObjectReference {
	parts := strings.SplitN(str, ":", 3)

	return corev1.ObjectReference{
		APIVersion: parts[0],
		Kind:       parts[1],
		Name:       parts[2],
	}
}

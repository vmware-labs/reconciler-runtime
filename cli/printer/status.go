/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func ResourceStatus(name string, condition *metav1.Condition) string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("# %s: %s\n", name, ConditionStatus(condition)))
	if condition != nil {
		s, _ := yaml.Marshal(condition)
		b.WriteString("---\n")
		b.Write(s)
	}
	return b.String()
}

func FindCondition(conditions []metav1.Condition, ct string) *metav1.Condition {
	for _, c := range conditions {
		if c.Type == ct {
			return &c
		}
	}
	return nil
}

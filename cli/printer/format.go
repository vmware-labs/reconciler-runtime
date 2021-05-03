/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

func TimestampSince(timestamp metav1.Time, now time.Time) string {
	if timestamp.IsZero() {
		return Swarnf("<unknown>")
	}
	return duration.HumanDuration(now.Sub(timestamp.Time))
}

func EmptyString(str string) string {
	if str == "" {
		return Sfaintf("<empty>")
	}
	return str
}

func ConditionStatus(cond *metav1.Condition) string {
	if cond == nil || cond.Status == "" {
		return Swarnf("<unknown>")
	}
	status := string(cond.Status)
	switch status {
	case "True":
		return Ssuccessf(string(cond.Type))
	case "False":
		if cond.Reason == "" {
			// display something if there is no reason
			return Serrorf("not-" + string(cond.Type))
		}
		return Serrorf(cond.Reason)
	default:
		return Sinfof(status)
	}
}

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

import (
	"reflect"
	"sort"
	"time"

	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConditionReady specifies that the resource is ready.
	// For long-running resources.
	ConditionReady = "Ready"
	// ConditionSucceeded specifies that the resource has finished.
	// For resource which run to completion.
	ConditionSucceeded = "Succeeded"
)

// Conditions is the interface for a Resource that implements the getter and
// setter for accessing a Condition collection.
type ConditionsAccessor interface {
	GetConditions() []metav1.Condition
	SetConditions([]metav1.Condition)
}

type NowFunc func() time.Time

// ConditionSet is an abstract collection of the possible ConditionType values
// that a particular resource might expose.  It also holds the "happy condition"
// for that resource, which we define to be one of Ready or Succeeded depending
// on whether it is a Living or Batch process respectively.
type ConditionSet struct {
	happyType   string
	happyReason string
	dependents  []string
}

// ConditionManager allows a resource to operate on its Conditions using higher
// order operations.
type ConditionManager interface {
	// IsHappy looks at the happy condition and returns true if that condition is
	// set to true.
	IsHappy() bool

	// GetCondition finds and returns the Condition that matches the ConditionType
	// previously set on Conditions.
	GetCondition(t string) *metav1.Condition

	// SetCondition sets or updates the Condition on Conditions for Condition.Type.
	// If there is an update, Conditions are stored back sorted.
	SetCondition(new metav1.Condition)

	// ClearCondition removes the non terminal condition that matches the ConditionType
	ClearCondition(t string) error

	// MarkTrue sets the status of t to true, and then marks the happy condition to
	// true if all dependents are true.
	MarkTrue(t string, reason, messageFormat string, messageA ...interface{})

	// MarkUnknown sets the status of t to Unknown and also sets the happy condition
	// to Unknown if no other dependent condition is in an error state.
	MarkUnknown(t string, reason, messageFormat string, messageA ...interface{})

	// MarkFalse sets the status of t and the happy condition to False.
	MarkFalse(t string, reason, messageFormat string, messageA ...interface{})

	// InitializeConditions updates all Conditions in the ConditionSet to Unknown
	// if not set.
	InitializeConditions()
}

// NewLivingConditionSet returns a ConditionSet to hold the conditions for the
// living resource. ConditionReady is used as the happy condition.
// The set of condition types provided are those of the terminal subconditions.
func NewLivingConditionSet(d ...string) ConditionSet {
	return NewLivingConditionSetWithHappyReason("Ready", d...)
}

// NewLivingConditionSetWithHappyReason returns a ConditionSet to hold the conditions for the
// living resource. ConditionReady is used as the happy condition with the provided happy reason.
// The set of condition types provided are those of the terminal subconditions.
func NewLivingConditionSetWithHappyReason(happyReason string, d ...string) ConditionSet {
	return newConditionSet(ConditionReady, happyReason, d...)
}

// NewBatchConditionSet returns a ConditionSet to hold the conditions for the
// batch resource. ConditionSucceeded is used as the happy condition.
// The set of condition types provided are those of the terminal subconditions.
func NewBatchConditionSet(d ...string) ConditionSet {
	return NewBatchConditionSetWithHappyReason("Succeeded", d...)
}

// NewBatchConditionSetWithHappyReason returns a ConditionSet to hold the conditions for the
// batch resource. ConditionSucceeded is used as the happy condition with the provided happy reason.
// The set of condition types provided are those of the terminal subconditions.
func NewBatchConditionSetWithHappyReason(happyReason string, d ...string) ConditionSet {
	return newConditionSet(ConditionSucceeded, happyReason, d...)
}

// newConditionSet returns a ConditionSet to hold the conditions that are
// important for the caller. The first ConditionType is the overarching status
// for that will be used to signal the resources' status is Ready or Succeeded.
func newConditionSet(happyType, happyReason string, dependents ...string) ConditionSet {
	var deps []string
	for _, d := range dependents {
		// Skip duplicates
		if d == happyType || contains(deps, d) {
			continue
		}
		deps = append(deps, d)
	}
	return ConditionSet{
		happyType:   happyType,
		happyReason: happyReason,
		dependents:  deps,
	}
}

func contains(ct []string, t string) bool {
	for _, c := range ct {
		if c == t {
			return true
		}
	}
	return false
}

// Check that conditionsImpl implements ConditionManager.
var _ ConditionManager = (*conditionsImpl)(nil)

// conditionsImpl implements the helper methods for evaluating Conditions.
// +k8s:deepcopy-gen=false
type conditionsImpl struct {
	ConditionSet
	accessor ConditionsAccessor
}

// Manage creates a ConditionManager from an accessor object using the original
// ConditionSet as a reference. Status must be a pointer to a struct.
func (r ConditionSet) Manage(status ConditionsAccessor) ConditionManager {
	return conditionsImpl{
		accessor:     status,
		ConditionSet: r,
	}
}

// IsHappy looks at the happy condition and returns true if that condition is
// set to true.
func (r conditionsImpl) IsHappy() bool {
	if c := r.GetCondition(r.happyType); c == nil || !ConditionIsTrue(c) {
		return false
	}
	return true
}

// GetCondition finds and returns the Condition that matches the ConditionType
// previously set on Conditions.
func (r conditionsImpl) GetCondition(t string) *metav1.Condition {
	if r.accessor == nil {
		return nil
	}

	for _, c := range r.accessor.GetConditions() {
		if c.Type == t {
			return &c
		}
	}
	return nil
}

// SetCondition sets or updates the Condition on Conditions for Condition.Type.
// If there is an update, Conditions are stored back sorted.
func (r conditionsImpl) SetCondition(new metav1.Condition) {
	if r.accessor == nil {
		return
	}
	t := new.Type
	var conditions []metav1.Condition
	for _, c := range r.accessor.GetConditions() {
		if c.Type != t {
			conditions = append(conditions, c)
		} else {
			// If we'd only update the LastTransitionTime, then return.
			new.LastTransitionTime = c.LastTransitionTime
			if reflect.DeepEqual(&new, &c) {
				return
			}
		}
	}
	new.LastTransitionTime = metav1.NewTime(time.Now())
	conditions = append(conditions, new)
	// Sorted for convenience of the consumer, i.e. kubectl.
	sort.Slice(conditions, func(i, j int) bool { return conditions[i].Type < conditions[j].Type })
	r.accessor.SetConditions(conditions)
}

func (r conditionsImpl) isTerminal(t string) bool {
	for _, cond := range r.dependents {
		if cond == t {
			return true
		}
	}
	return t == r.happyType
}

// RemoveCondition removes the non terminal condition that matches the ConditionType
// Not implemented for terminal conditions
func (r conditionsImpl) ClearCondition(t string) error {
	var conditions []metav1.Condition

	if r.accessor == nil {
		return nil
	}
	// Terminal conditions are not handled as they can't be nil
	if r.isTerminal(t) {
		return fmt.Errorf("Clearing terminal conditions not implemented")
	}
	cond := r.GetCondition(t)
	if cond == nil {
		return nil
	}
	for _, c := range r.accessor.GetConditions() {
		if c.Type != t {
			conditions = append(conditions, c)
		}
	}

	// Sorted for convenience of the consumer, i.e. kubectl.
	sort.Slice(conditions, func(i, j int) bool { return conditions[i].Type < conditions[j].Type })
	r.accessor.SetConditions(conditions)

	return nil
}

// MarkTrue sets the status of t to true, and then marks the happy condition to
// true if all other dependents are also true.
func (r conditionsImpl) MarkTrue(t string, reason, messageFormat string, messageA ...interface{}) {
	// set the specified condition
	r.SetCondition(metav1.Condition{
		Type:    t,
		Status:  metav1.ConditionTrue,
		Reason:  reason,
		Message: fmt.Sprintf(messageFormat, messageA...),
	})

	// check the dependents.
	for _, cond := range r.dependents {
		c := r.GetCondition(cond)
		// Failed or Unknown conditions trump true conditions
		if !ConditionIsTrue(c) {
			return
		}
	}

	// set the happy condition
	r.SetCondition(metav1.Condition{
		Type:   r.happyType,
		Reason: r.happyReason,
		Status: metav1.ConditionTrue,
	})
}

// MarkUnknown sets the status of t to Unknown and also sets the happy condition
// to Unknown if no other dependent condition is in an error state.
func (r conditionsImpl) MarkUnknown(t string, reason, messageFormat string, messageA ...interface{}) {
	// set the specified condition
	r.SetCondition(metav1.Condition{
		Type:    t,
		Status:  metav1.ConditionUnknown,
		Reason:  reason,
		Message: fmt.Sprintf(messageFormat, messageA...),
	})

	// check the dependents.
	isDependent := false
	for _, cond := range r.dependents {
		c := r.GetCondition(cond)
		// Failed conditions trump Unknown conditions
		if ConditionIsFalse(c) {
			// Double check that the happy condition is also false.
			happy := r.GetCondition(r.happyType)
			if !ConditionIsFalse(happy) {
				r.MarkFalse(r.happyType, reason, messageFormat, messageA...)
			}
			return
		}
		if cond == t {
			isDependent = true
		}
	}

	if isDependent {
		// set the happy condition, if it is one of our dependent subconditions.
		r.SetCondition(metav1.Condition{
			Type:    r.happyType,
			Status:  metav1.ConditionUnknown,
			Reason:  reason,
			Message: fmt.Sprintf(messageFormat, messageA...),
		})
	}
}

// MarkFalse sets the status of t and the happy condition to False.
func (r conditionsImpl) MarkFalse(t string, reason, messageFormat string, messageA ...interface{}) {
	types := []string{t}
	for _, cond := range r.dependents {
		if cond == t {
			types = append(types, r.happyType)
		}
	}

	for _, t := range types {
		r.SetCondition(metav1.Condition{
			Type:    t,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: fmt.Sprintf(messageFormat, messageA...),
		})
	}
}

// InitializeConditions updates all Conditions in the ConditionSet to Unknown
// if not set.
func (r conditionsImpl) InitializeConditions() {
	happy := r.GetCondition(r.happyType)
	if happy == nil {
		happy = &metav1.Condition{
			Type:   r.happyType,
			Status: metav1.ConditionUnknown,
			Reason: "Initializing",
		}
		r.SetCondition(*happy)
	}
	// If the happy state is true, it implies that all of the terminal
	// subconditions must be true, so initialize any unset conditions to
	// true if our happy condition is true, otherwise unknown.
	status := metav1.ConditionUnknown
	if happy.Status == metav1.ConditionTrue {
		status = metav1.ConditionTrue
	}
	for _, t := range r.dependents {
		r.initializeTerminalCondition(t, "Initializing", status)
	}
}

// initializeTerminalCondition initializes a Condition to the given status if unset.
func (r conditionsImpl) initializeTerminalCondition(t, reason string, status metav1.ConditionStatus) *metav1.Condition {
	if c := r.GetCondition(t); c != nil {
		return c
	}
	c := metav1.Condition{
		Type:   t,
		Reason: reason,
		Status: status,
	}
	r.SetCondition(c)
	return &c
}

// ConditionIsTrue returns true if the condition's status is True
func ConditionIsTrue(c *metav1.Condition) bool {
	return c != nil && c.Status == metav1.ConditionTrue
}

// ConditionIsFalse returns true if the condition's status is False
func ConditionIsFalse(c *metav1.Condition) bool {
	return c != nil && c.Status == metav1.ConditionFalse
}

// ConditionIsUnknown returns true if the condition's status is Unknown or not known
func ConditionIsUnknown(c *metav1.Condition) bool {
	return !ConditionIsTrue(c) && !ConditionIsFalse(c)
}

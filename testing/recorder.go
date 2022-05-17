/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Event struct {
	metav1.TypeMeta
	types.NamespacedName
	Type    string
	Reason  string
	Message string
}

func NewEvent(factory client.Object, scheme *runtime.Scheme, eventtype, reason, messageFormat string, a ...interface{}) Event {
	obj := factory.DeepCopyObject()
	objref, err := ref.GetReference(scheme, obj)
	if err != nil {
		panic(fmt.Sprintf("Could not construct reference to: '%#v' due to: '%v'. Will not report event: '%v' '%v' '%v'", obj, err, eventtype, reason, fmt.Sprintf(messageFormat, a...)))
	}

	return Event{
		TypeMeta: metav1.TypeMeta{
			APIVersion: objref.APIVersion,
			Kind:       objref.Kind,
		},
		NamespacedName: types.NamespacedName{
			Namespace: objref.Namespace,
			Name:      objref.Name,
		},
		Type:    eventtype,
		Reason:  reason,
		Message: fmt.Sprintf(messageFormat, a...),
	}
}

type eventRecorder struct {
	events []Event
	scheme *runtime.Scheme
}

var (
	_ record.EventRecorder = (*eventRecorder)(nil)
)

func (r *eventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	r.Eventf(object, eventtype, reason, message)
}

func (r *eventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	r.events = append(r.events, NewEvent(object.(client.Object), r.scheme, eventtype, reason, messageFmt, args...))
}

func (r *eventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	r.Eventf(object, eventtype, reason, messageFmt, args...)
}

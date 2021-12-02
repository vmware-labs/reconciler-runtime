/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type service struct {
	NullObjectMeta
	target *corev1.Service
}

var (
	_ rtesting.Factory = (*service)(nil)
	_ client.Object    = (*service)(nil)
)

// Deprecated
func Service(seed ...*corev1.Service) *service {
	var target *corev1.Service
	switch len(seed) {
	case 0:
		target = &corev1.Service{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &service{
		target: target,
	}
}

func (f *service) DeepCopyObject() runtime.Object {
	return f.CreateObject()
}

func (f *service) GetObjectKind() schema.ObjectKind {
	return f.CreateObject().GetObjectKind()
}

func (f *service) deepCopy() *service {
	return Service(f.target.DeepCopy())
}

func (f *service) Create() *corev1.Service {
	return f.deepCopy().target
}

func (f *service) CreateObject() client.Object {
	return f.Create()
}

func (f *service) mutation(m func(*corev1.Service)) *service {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *service) NamespaceName(namespace, name string) *service {
	return f.mutation(func(sa *corev1.Service) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *service) ObjectMeta(nf func(ObjectMeta)) *service {
	return f.mutation(func(sa *corev1.Service) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *service) AddSelectorLabel(key, value string) *service {
	return f.mutation(func(service *corev1.Service) {
		if service.Spec.Selector == nil {
			service.Spec.Selector = map[string]string{}
		}
		service.Spec.Selector[key] = value
	})
}

func (f *service) Ports(ports ...corev1.ServicePort) *service {
	return f.mutation(func(service *corev1.Service) {
		service.Spec.Ports = ports
	})
}

func (f *service) ClusterIP(ip string) *service {
	return f.mutation(func(service *corev1.Service) {
		service.Spec.ClusterIP = ip
	})
}

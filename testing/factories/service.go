/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type service struct {
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

func (f *service) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *service) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *service) GetNamespace() string                            { panic("not implemeneted") }
func (f *service) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *service) GetName() string                                 { panic("not implemeneted") }
func (f *service) SetName(name string)                             { panic("not implemeneted") }
func (f *service) GetGenerateName() string                         { panic("not implemeneted") }
func (f *service) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *service) GetUID() types.UID                               { panic("not implemeneted") }
func (f *service) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *service) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *service) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *service) GetGeneration() int64                            { panic("not implemeneted") }
func (f *service) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *service) GetSelfLink() string                             { panic("not implemeneted") }
func (f *service) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *service) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *service) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *service) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *service) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *service) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *service) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *service) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *service) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *service) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *service) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *service) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *service) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *service) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *service) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *service) GetClusterName() string                          { panic("not implemeneted") }
func (f *service) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *service) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *service) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

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

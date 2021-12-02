/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ingress struct {
	target *networkingv1beta1.Ingress
}

var (
	_ rtesting.Factory = (*ingress)(nil)
	_ client.Object    = (*ingress)(nil)
)

// Deprecated
func Ingress(seed ...*networkingv1beta1.Ingress) *ingress {
	var target *networkingv1beta1.Ingress
	switch len(seed) {
	case 0:
		target = &networkingv1beta1.Ingress{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &ingress{
		target: target,
	}
}

func (f *ingress) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *ingress) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *ingress) GetNamespace() string                            { panic("not implemeneted") }
func (f *ingress) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *ingress) GetName() string                                 { panic("not implemeneted") }
func (f *ingress) SetName(name string)                             { panic("not implemeneted") }
func (f *ingress) GetGenerateName() string                         { panic("not implemeneted") }
func (f *ingress) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *ingress) GetUID() types.UID                               { panic("not implemeneted") }
func (f *ingress) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *ingress) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *ingress) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *ingress) GetGeneration() int64                            { panic("not implemeneted") }
func (f *ingress) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *ingress) GetSelfLink() string                             { panic("not implemeneted") }
func (f *ingress) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *ingress) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *ingress) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *ingress) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *ingress) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *ingress) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *ingress) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *ingress) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *ingress) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *ingress) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *ingress) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *ingress) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *ingress) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *ingress) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *ingress) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *ingress) GetClusterName() string                          { panic("not implemeneted") }
func (f *ingress) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *ingress) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *ingress) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *ingress) deepCopy() *ingress {
	return Ingress(f.target.DeepCopy())
}

func (f *ingress) Create() *networkingv1beta1.Ingress {
	return f.deepCopy().target
}

func (f *ingress) CreateObject() client.Object {
	return f.Create()
}

func (f *ingress) mutation(m func(*networkingv1beta1.Ingress)) *ingress {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *ingress) NamespaceName(namespace, name string) *ingress {
	return f.mutation(func(sa *networkingv1beta1.Ingress) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *ingress) ObjectMeta(nf func(ObjectMeta)) *ingress {
	return f.mutation(func(sa *networkingv1beta1.Ingress) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *ingress) HostToService(host, serviceName string) *ingress {
	return f.mutation(func(i *networkingv1beta1.Ingress) {
		i.Spec = networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{{
				Host: host,
				IngressRuleValue: networkingv1beta1.IngressRuleValue{
					HTTP: &networkingv1beta1.HTTPIngressRuleValue{
						Paths: []networkingv1beta1.HTTPIngressPath{{
							Path: "/",
							Backend: networkingv1beta1.IngressBackend{
								ServiceName: serviceName,
								ServicePort: intstr.FromInt(80),
							},
						}},
					},
				},
			}},
		}
	})
}

func (f *ingress) StatusLoadBalancer(ingress ...corev1.LoadBalancerIngress) *ingress {
	return f.mutation(func(i *networkingv1beta1.Ingress) {
		i.Status.LoadBalancer.Ingress = ingress
	})
}

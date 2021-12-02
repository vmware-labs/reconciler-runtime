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

type secret struct {
	target *corev1.Secret
}

var (
	_ rtesting.Factory = (*secret)(nil)
	_ client.Object    = (*secret)(nil)
)

// Deprecated
func Secret(seed ...*corev1.Secret) *secret {
	var target *corev1.Secret
	switch len(seed) {
	case 0:
		target = &corev1.Secret{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &secret{
		target: target,
	}
}

func (f *secret) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *secret) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *secret) GetNamespace() string                            { panic("not implemeneted") }
func (f *secret) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *secret) GetName() string                                 { panic("not implemeneted") }
func (f *secret) SetName(name string)                             { panic("not implemeneted") }
func (f *secret) GetGenerateName() string                         { panic("not implemeneted") }
func (f *secret) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *secret) GetUID() types.UID                               { panic("not implemeneted") }
func (f *secret) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *secret) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *secret) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *secret) GetGeneration() int64                            { panic("not implemeneted") }
func (f *secret) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *secret) GetSelfLink() string                             { panic("not implemeneted") }
func (f *secret) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *secret) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *secret) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *secret) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *secret) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *secret) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *secret) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *secret) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *secret) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *secret) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *secret) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *secret) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *secret) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *secret) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *secret) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *secret) GetClusterName() string                          { panic("not implemeneted") }
func (f *secret) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *secret) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *secret) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *secret) deepCopy() *secret {
	return Secret(f.target.DeepCopy())
}

func (f *secret) Create() *corev1.Secret {
	return f.deepCopy().target
}

func (f *secret) CreateObject() client.Object {
	return f.Create()
}

func (f *secret) mutation(m func(*corev1.Secret)) *secret {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *secret) NamespaceName(namespace, name string) *secret {
	return f.mutation(func(s *corev1.Secret) {
		s.ObjectMeta.Namespace = namespace
		s.ObjectMeta.Name = name
	})
}

func (f *secret) ObjectMeta(nf func(ObjectMeta)) *secret {
	return f.mutation(func(s *corev1.Secret) {
		omf := ObjectMetaFactory(s.ObjectMeta)
		nf(omf)
		s.ObjectMeta = omf.Create()
	})
}

func (f *secret) Type(t corev1.SecretType) *secret {
	return f.mutation(func(s *corev1.Secret) {
		s.Type = t
	})
}

func (f *secret) AddData(key, value string) *secret {
	return f.mutation(func(s *corev1.Secret) {
		if s.Data == nil {
			s.Data = map[string][]byte{}
		}
		s.Data[key] = []byte(value)
	})
}

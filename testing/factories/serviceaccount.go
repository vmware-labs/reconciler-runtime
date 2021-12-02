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

type serviceAccount struct {
	target *corev1.ServiceAccount
}

var (
	_ rtesting.Factory = (*serviceAccount)(nil)
	_ client.Object    = (*serviceAccount)(nil)
)

// Deprecated
func ServiceAccount(seed ...*corev1.ServiceAccount) *serviceAccount {
	var target *corev1.ServiceAccount
	switch len(seed) {
	case 0:
		target = &corev1.ServiceAccount{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &serviceAccount{
		target: target,
	}
}

func (f *serviceAccount) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *serviceAccount) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *serviceAccount) GetNamespace() string                            { panic("not implemeneted") }
func (f *serviceAccount) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *serviceAccount) GetName() string                                 { panic("not implemeneted") }
func (f *serviceAccount) SetName(name string)                             { panic("not implemeneted") }
func (f *serviceAccount) GetGenerateName() string                         { panic("not implemeneted") }
func (f *serviceAccount) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *serviceAccount) GetUID() types.UID                               { panic("not implemeneted") }
func (f *serviceAccount) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *serviceAccount) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *serviceAccount) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *serviceAccount) GetGeneration() int64                            { panic("not implemeneted") }
func (f *serviceAccount) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *serviceAccount) GetSelfLink() string                             { panic("not implemeneted") }
func (f *serviceAccount) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *serviceAccount) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *serviceAccount) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *serviceAccount) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *serviceAccount) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *serviceAccount) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *serviceAccount) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *serviceAccount) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *serviceAccount) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *serviceAccount) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *serviceAccount) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *serviceAccount) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *serviceAccount) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *serviceAccount) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *serviceAccount) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *serviceAccount) GetClusterName() string                          { panic("not implemeneted") }
func (f *serviceAccount) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *serviceAccount) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *serviceAccount) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *serviceAccount) deepCopy() *serviceAccount {
	return ServiceAccount(f.target.DeepCopy())
}

func (f *serviceAccount) Create() *corev1.ServiceAccount {
	return f.deepCopy().target
}

func (f *serviceAccount) CreateObject() client.Object {
	return f.Create()
}

func (f *serviceAccount) mutation(m func(*corev1.ServiceAccount)) *serviceAccount {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *serviceAccount) NamespaceName(namespace, name string) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *serviceAccount) ObjectMeta(nf func(ObjectMeta)) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *serviceAccount) Secrets(secrets ...string) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		sa.Secrets = make([]corev1.ObjectReference, len(secrets))
		for i, secret := range secrets {
			sa.Secrets[i] = corev1.ObjectReference{Name: secret}
		}
	})
}

func (f *serviceAccount) ImagePullSecrets(secrets ...string) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		sa.ImagePullSecrets = make([]corev1.LocalObjectReference, len(secrets))
		for i, secret := range secrets {
			sa.ImagePullSecrets[i] = corev1.LocalObjectReference{Name: secret}
		}
	})
}

func (f *serviceAccount) AutomountServiceAccountToken(b bool) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		sa.AutomountServiceAccountToken = &b
	})
}

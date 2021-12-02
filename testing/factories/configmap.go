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

type configMap struct {
	target *corev1.ConfigMap
}

var (
	_ rtesting.Factory = (*configMap)(nil)
	_ client.Object    = (*configMap)(nil)
)

// Deprecated
func ConfigMap(seed ...*corev1.ConfigMap) *configMap {
	var target *corev1.ConfigMap
	switch len(seed) {
	case 0:
		target = &corev1.ConfigMap{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &configMap{
		target: target,
	}
}

func (f *configMap) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *configMap) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *configMap) GetNamespace() string                            { panic("not implemeneted") }
func (f *configMap) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *configMap) GetName() string                                 { panic("not implemeneted") }
func (f *configMap) SetName(name string)                             { panic("not implemeneted") }
func (f *configMap) GetGenerateName() string                         { panic("not implemeneted") }
func (f *configMap) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *configMap) GetUID() types.UID                               { panic("not implemeneted") }
func (f *configMap) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *configMap) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *configMap) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *configMap) GetGeneration() int64                            { panic("not implemeneted") }
func (f *configMap) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *configMap) GetSelfLink() string                             { panic("not implemeneted") }
func (f *configMap) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *configMap) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *configMap) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *configMap) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *configMap) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *configMap) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *configMap) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *configMap) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *configMap) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *configMap) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *configMap) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *configMap) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *configMap) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *configMap) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *configMap) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *configMap) GetClusterName() string                          { panic("not implemeneted") }
func (f *configMap) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *configMap) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *configMap) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *configMap) deepCopy() *configMap {
	return ConfigMap(f.target.DeepCopy())
}

func (f *configMap) Create() *corev1.ConfigMap {
	return f.deepCopy().target
}

func (f *configMap) CreateObject() client.Object {
	return f.Create()
}

func (f *configMap) mutation(m func(*corev1.ConfigMap)) *configMap {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *configMap) NamespaceName(namespace, name string) *configMap {
	return f.mutation(func(cm *corev1.ConfigMap) {
		cm.ObjectMeta.Namespace = namespace
		cm.ObjectMeta.Name = name
	})
}

func (f *configMap) ObjectMeta(nf func(ObjectMeta)) *configMap {
	return f.mutation(func(cm *corev1.ConfigMap) {
		omf := ObjectMetaFactory(cm.ObjectMeta)
		nf(omf)
		cm.ObjectMeta = omf.Create()
	})
}

func (f *configMap) AddData(key, value string) *configMap {
	return f.mutation(func(cm *corev1.ConfigMap) {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[key] = value
	})
}

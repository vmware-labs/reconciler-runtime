/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type rolebinding struct {
	target *rbacv1.RoleBinding
}

var (
	_ rtesting.Factory = (*rolebinding)(nil)
	_ client.Object    = (*rolebinding)(nil)
)

// Deprecated
func RoleBinding(seed ...*rbacv1.RoleBinding) *rolebinding {
	var target *rbacv1.RoleBinding
	switch len(seed) {
	case 0:
		target = &rbacv1.RoleBinding{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &rolebinding{
		target: target,
	}
}

func (f *rolebinding) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *rolebinding) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *rolebinding) GetNamespace() string                            { panic("not implemeneted") }
func (f *rolebinding) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *rolebinding) GetName() string                                 { panic("not implemeneted") }
func (f *rolebinding) SetName(name string)                             { panic("not implemeneted") }
func (f *rolebinding) GetGenerateName() string                         { panic("not implemeneted") }
func (f *rolebinding) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *rolebinding) GetUID() types.UID                               { panic("not implemeneted") }
func (f *rolebinding) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *rolebinding) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *rolebinding) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *rolebinding) GetGeneration() int64                            { panic("not implemeneted") }
func (f *rolebinding) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *rolebinding) GetSelfLink() string                             { panic("not implemeneted") }
func (f *rolebinding) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *rolebinding) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *rolebinding) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *rolebinding) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *rolebinding) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *rolebinding) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *rolebinding) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *rolebinding) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *rolebinding) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *rolebinding) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *rolebinding) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *rolebinding) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *rolebinding) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *rolebinding) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *rolebinding) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *rolebinding) GetClusterName() string                          { panic("not implemeneted") }
func (f *rolebinding) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *rolebinding) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *rolebinding) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *rolebinding) deepCopy() *rolebinding {
	return RoleBinding(f.target.DeepCopy())
}

func (f *rolebinding) Create() *rbacv1.RoleBinding {
	return f.deepCopy().target
}

func (f *rolebinding) CreateObject() client.Object {
	return f.Create()
}

func (f *rolebinding) mutation(m func(*rbacv1.RoleBinding)) *rolebinding {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *rolebinding) NamespaceName(namespace, name string) *rolebinding {
	return f.mutation(func(sa *rbacv1.RoleBinding) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *rolebinding) ObjectMeta(nf func(ObjectMeta)) *rolebinding {
	return f.mutation(func(sa *rbacv1.RoleBinding) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *rolebinding) Subjects(subjects []rbacv1.Subject) *rolebinding {
	return f.mutation(func(s *rbacv1.RoleBinding) {
		s.Subjects = subjects
	})
}

func (f *rolebinding) RoleRef(roleRef rbacv1.RoleRef) *rolebinding {
	return f.mutation(func(s *rbacv1.RoleBinding) {
		s.RoleRef = roleRef
	})
}

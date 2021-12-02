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

type role struct {
	target *rbacv1.Role
}

var (
	_ rtesting.Factory = (*role)(nil)
	_ client.Object    = (*role)(nil)
)

// Deprecated
func Role(seed ...*rbacv1.Role) *role {
	var target *rbacv1.Role
	switch len(seed) {
	case 0:
		target = &rbacv1.Role{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &role{
		target: target,
	}
}

func (f *role) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *role) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *role) GetNamespace() string                            { panic("not implemeneted") }
func (f *role) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *role) GetName() string                                 { panic("not implemeneted") }
func (f *role) SetName(name string)                             { panic("not implemeneted") }
func (f *role) GetGenerateName() string                         { panic("not implemeneted") }
func (f *role) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *role) GetUID() types.UID                               { panic("not implemeneted") }
func (f *role) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *role) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *role) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *role) GetGeneration() int64                            { panic("not implemeneted") }
func (f *role) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *role) GetSelfLink() string                             { panic("not implemeneted") }
func (f *role) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *role) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *role) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *role) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *role) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *role) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *role) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *role) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *role) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *role) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *role) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *role) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *role) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *role) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *role) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *role) GetClusterName() string                          { panic("not implemeneted") }
func (f *role) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *role) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *role) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *role) deepCopy() *role {
	return Role(f.target.DeepCopy())
}

func (f *role) Create() *rbacv1.Role {
	return f.deepCopy().target
}

func (f *role) CreateObject() client.Object {
	return f.Create()
}

func (f *role) mutation(m func(*rbacv1.Role)) *role {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *role) NamespaceName(namespace, name string) *role {
	return f.mutation(func(sa *rbacv1.Role) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *role) ObjectMeta(nf func(ObjectMeta)) *role {
	return f.mutation(func(sa *rbacv1.Role) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *role) Rules(rules []rbacv1.PolicyRule) *role {
	return f.mutation(func(s *rbacv1.Role) {
		s.Rules = rules
	})
}

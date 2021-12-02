/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type testresourcenostatus struct {
	target *rtesting.TestResourceNoStatus
}

var (
	_ rtesting.Factory = (*testresourcenostatus)(nil)
	_ client.Object    = (*testresourcenostatus)(nil)
)

// Deprecated
func TestResourceNoStatus(seed ...*rtesting.TestResourceNoStatus) *testresourcenostatus {
	var target *rtesting.TestResourceNoStatus
	switch len(seed) {
	case 0:
		target = &rtesting.TestResourceNoStatus{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &testresourcenostatus{
		target: target,
	}
}

func (f *testresourcenostatus) DeepCopyObject() runtime.Object { return f.CreateObject() }
func (f *testresourcenostatus) GetObjectKind() schema.ObjectKind {
	return f.CreateObject().GetObjectKind()
}
func (f *testresourcenostatus) GetNamespace() string                       { panic("not implemeneted") }
func (f *testresourcenostatus) SetNamespace(namespace string)              { panic("not implemeneted") }
func (f *testresourcenostatus) GetName() string                            { panic("not implemeneted") }
func (f *testresourcenostatus) SetName(name string)                        { panic("not implemeneted") }
func (f *testresourcenostatus) GetGenerateName() string                    { panic("not implemeneted") }
func (f *testresourcenostatus) SetGenerateName(name string)                { panic("not implemeneted") }
func (f *testresourcenostatus) GetUID() types.UID                          { panic("not implemeneted") }
func (f *testresourcenostatus) SetUID(uid types.UID)                       { panic("not implemeneted") }
func (f *testresourcenostatus) GetResourceVersion() string                 { panic("not implemeneted") }
func (f *testresourcenostatus) SetResourceVersion(version string)          { panic("not implemeneted") }
func (f *testresourcenostatus) GetGeneration() int64                       { panic("not implemeneted") }
func (f *testresourcenostatus) SetGeneration(generation int64)             { panic("not implemeneted") }
func (f *testresourcenostatus) GetSelfLink() string                        { panic("not implemeneted") }
func (f *testresourcenostatus) SetSelfLink(selfLink string)                { panic("not implemeneted") }
func (f *testresourcenostatus) GetCreationTimestamp() metav1.Time          { panic("not implemeneted") }
func (f *testresourcenostatus) SetCreationTimestamp(timestamp metav1.Time) { panic("not implemeneted") }
func (f *testresourcenostatus) GetDeletionTimestamp() *metav1.Time         { panic("not implemeneted") }
func (f *testresourcenostatus) SetDeletionTimestamp(timestamp *metav1.Time) {
	panic("not implemeneted")
}
func (f *testresourcenostatus) GetDeletionGracePeriodSeconds() *int64 { panic("not implemeneted") }
func (f *testresourcenostatus) SetDeletionGracePeriodSeconds(*int64)  { panic("not implemeneted") }
func (f *testresourcenostatus) GetLabels() map[string]string          { panic("not implemeneted") }
func (f *testresourcenostatus) SetLabels(labels map[string]string)    { panic("not implemeneted") }
func (f *testresourcenostatus) GetAnnotations() map[string]string     { panic("not implemeneted") }
func (f *testresourcenostatus) SetAnnotations(annotations map[string]string) {
	panic("not implemeneted")
}
func (f *testresourcenostatus) GetFinalizers() []string           { panic("not implemeneted") }
func (f *testresourcenostatus) SetFinalizers(finalizers []string) { panic("not implemeneted") }
func (f *testresourcenostatus) GetOwnerReferences() []metav1.OwnerReference {
	panic("not implemeneted")
}
func (f *testresourcenostatus) SetOwnerReferences([]metav1.OwnerReference) { panic("not implemeneted") }
func (f *testresourcenostatus) GetClusterName() string                     { panic("not implemeneted") }
func (f *testresourcenostatus) SetClusterName(clusterName string)          { panic("not implemeneted") }
func (f *testresourcenostatus) GetManagedFields() []metav1.ManagedFieldsEntry {
	panic("not implemeneted")
}
func (f *testresourcenostatus) SetManagedFields(mf []metav1.ManagedFieldsEntry) {
	panic("not implemeneted")
}

func (f *testresourcenostatus) deepCopy() *testresourcenostatus {
	return TestResourceNoStatus(f.target.DeepCopy())
}

func (f *testresourcenostatus) Create() *rtesting.TestResourceNoStatus {
	return f.deepCopy().target
}

func (f *testresourcenostatus) CreateObject() client.Object {
	return f.Create()
}

func (f *testresourcenostatus) mutation(m func(*rtesting.TestResourceNoStatus)) *testresourcenostatus {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *testresourcenostatus) NamespaceName(namespace, name string) *testresourcenostatus {
	return f.mutation(func(sa *rtesting.TestResourceNoStatus) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *testresourcenostatus) ObjectMeta(nf func(ObjectMeta)) *testresourcenostatus {
	return f.mutation(func(sa *rtesting.TestResourceNoStatus) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

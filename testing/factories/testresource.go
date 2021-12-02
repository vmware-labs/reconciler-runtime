/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/apis"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type testresource struct {
	target *rtesting.TestResource
}

var (
	_ rtesting.Factory = (*testresource)(nil)
	_ client.Object    = (*testresource)(nil)
)

// Deprecated
func TestResource(seed ...*rtesting.TestResource) *testresource {
	var target *rtesting.TestResource
	switch len(seed) {
	case 0:
		target = &rtesting.TestResource{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &testresource{
		target: target,
	}
}

func (f *testresource) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *testresource) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *testresource) GetNamespace() string                            { panic("not implemeneted") }
func (f *testresource) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *testresource) GetName() string                                 { panic("not implemeneted") }
func (f *testresource) SetName(name string)                             { panic("not implemeneted") }
func (f *testresource) GetGenerateName() string                         { panic("not implemeneted") }
func (f *testresource) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *testresource) GetUID() types.UID                               { panic("not implemeneted") }
func (f *testresource) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *testresource) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *testresource) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *testresource) GetGeneration() int64                            { panic("not implemeneted") }
func (f *testresource) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *testresource) GetSelfLink() string                             { panic("not implemeneted") }
func (f *testresource) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *testresource) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *testresource) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *testresource) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *testresource) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *testresource) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *testresource) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *testresource) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *testresource) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *testresource) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *testresource) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *testresource) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *testresource) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *testresource) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *testresource) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *testresource) GetClusterName() string                          { panic("not implemeneted") }
func (f *testresource) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *testresource) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *testresource) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *testresource) deepCopy() *testresource {
	return TestResource(f.target.DeepCopy())
}

func (f *testresource) Create() *rtesting.TestResource {
	return f.deepCopy().target
}

func (f *testresource) CreateObject() client.Object {
	return f.Create()
}

func (f *testresource) mutation(m func(*rtesting.TestResource)) *testresource {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *testresource) NamespaceName(namespace, name string) *testresource {
	return f.mutation(func(sa *rtesting.TestResource) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *testresource) ObjectMeta(nf func(ObjectMeta)) *testresource {
	return f.mutation(func(sa *rtesting.TestResource) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *testresource) AddField(key string, value string) *testresource {
	return f.mutation(func(r *rtesting.TestResource) {
		if r.Spec.Fields == nil {
			r.Spec.Fields = map[string]string{}
		}
		r.Spec.Fields[key] = value
	})
}

func (f *testresource) PodTemplateSpec(nf func(PodTemplateSpec)) *testresource {
	return f.mutation(func(r *rtesting.TestResource) {
		ptsf := PodTemplateSpecFactory(r.Spec.Template)
		nf(ptsf)
		r.Spec.Template = ptsf.Create()
	})
}

func (f *testresource) ErrorOn(marshal, unmarshal bool) *testresource {
	return f.mutation(func(r *rtesting.TestResource) {
		r.Spec.ErrOnMarshal = marshal
		r.Spec.ErrOnUnmarshal = unmarshal
	})
}

func (f *testresource) StatusConditions(conditions ...ConditionFactory) *testresource {
	return f.mutation(func(testresource *rtesting.TestResource) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Create()
		}
		testresource.Status.Conditions = c
	})
}

func (f *testresource) AddStatusField(key string, value string) *testresource {
	return f.mutation(func(r *rtesting.TestResource) {
		if r.Status.Fields == nil {
			r.Status.Fields = map[string]string{}
		}
		r.Status.Fields[key] = value
	})
}

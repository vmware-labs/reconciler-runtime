/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Deprecated
type ObjectMeta interface {
	Create() metav1.ObjectMeta

	Namespace(namespace string) ObjectMeta
	Name(format string, a ...interface{}) ObjectMeta
	GenerateName(format string, a ...interface{}) ObjectMeta
	AddLabel(key, value string) ObjectMeta
	AddAnnotation(key, value string) ObjectMeta
	Generation(generation int64) ObjectMeta
	ControlledBy(owner rtesting.Factory, scheme *runtime.Scheme) ObjectMeta
	Created(sec int64) ObjectMeta
	Deleted(sec int64) ObjectMeta
	UID(uid string) ObjectMeta
}

type objectMetaImpl struct {
	target *metav1.ObjectMeta
}

// Deprecated
func ObjectMetaFactory(seed metav1.ObjectMeta) ObjectMeta {
	return &objectMetaImpl{
		target: &seed,
	}
}

func (f *objectMetaImpl) Create() metav1.ObjectMeta {
	return *(f.target.DeepCopy())
}

func (f *objectMetaImpl) mutate(m func(*metav1.ObjectMeta)) ObjectMeta {
	m(f.target)
	return f
}

func (f *objectMetaImpl) Namespace(namespace string) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		om.Namespace = namespace
	})
}

func (f *objectMetaImpl) Name(format string, a ...interface{}) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		om.Name = fmt.Sprintf(format, a...)
	})
}

func (f *objectMetaImpl) GenerateName(format string, a ...interface{}) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		om.GenerateName = fmt.Sprintf(format, a...)
	})
}

func (f *objectMetaImpl) AddLabel(key, value string) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		if om.Labels == nil {
			om.Labels = map[string]string{}
		}
		om.Labels[key] = value
	})
}

func (f *objectMetaImpl) AddAnnotation(key, value string) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		if om.Annotations == nil {
			om.Annotations = map[string]string{}
		}
		om.Annotations[key] = value
	})
}

func (f *objectMetaImpl) Generation(generation int64) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		om.Generation = generation
	})
}

func (f *objectMetaImpl) ControlledBy(owner rtesting.Factory, scheme *runtime.Scheme) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		err := ctrl.SetControllerReference(owner.CreateObject(), om, scheme)
		if err != nil {
			panic(err)
		}
	})
}

func (f *objectMetaImpl) Created(sec int64) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		timestamp := metav1.Unix(sec, 0)
		om.CreationTimestamp = timestamp
	})
}

func (f *objectMetaImpl) Deleted(sec int64) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		timestamp := metav1.Unix(sec, 0)
		om.DeletionTimestamp = &timestamp
	})
}

func (f *objectMetaImpl) UID(uid string) ObjectMeta {
	return f.mutate(func(om *metav1.ObjectMeta) {
		om.UID = types.UID(uid)
	})
}

// Deprecated
type NullObjectMeta struct{}

func (*NullObjectMeta) GetNamespace() string                           { panic("not implemeneted") }
func (*NullObjectMeta) SetNamespace(_ string)                          { panic("not implemeneted") }
func (*NullObjectMeta) GetName() string                                { panic("not implemeneted") }
func (*NullObjectMeta) SetName(_ string)                               { panic("not implemeneted") }
func (*NullObjectMeta) GetGenerateName() string                        { panic("not implemeneted") }
func (*NullObjectMeta) SetGenerateName(_ string)                       { panic("not implemeneted") }
func (*NullObjectMeta) GetUID() types.UID                              { panic("not implemeneted") }
func (*NullObjectMeta) SetUID(_ types.UID)                             { panic("not implemeneted") }
func (*NullObjectMeta) GetResourceVersion() string                     { panic("not implemeneted") }
func (*NullObjectMeta) SetResourceVersion(_ string)                    { panic("not implemeneted") }
func (*NullObjectMeta) GetGeneration() int64                           { panic("not implemeneted") }
func (*NullObjectMeta) SetGeneration(_ int64)                          { panic("not implemeneted") }
func (*NullObjectMeta) GetSelfLink() string                            { panic("not implemeneted") }
func (*NullObjectMeta) SetSelfLink(_ string)                           { panic("not implemeneted") }
func (*NullObjectMeta) GetCreationTimestamp() metav1.Time              { panic("not implemeneted") }
func (*NullObjectMeta) SetCreationTimestamp(_ metav1.Time)             { panic("not implemeneted") }
func (*NullObjectMeta) GetDeletionTimestamp() *metav1.Time             { panic("not implemeneted") }
func (*NullObjectMeta) SetDeletionTimestamp(_ *metav1.Time)            { panic("not implemeneted") }
func (*NullObjectMeta) GetDeletionGracePeriodSeconds() *int64          { panic("not implemeneted") }
func (*NullObjectMeta) SetDeletionGracePeriodSeconds(*int64)           { panic("not implemeneted") }
func (*NullObjectMeta) GetLabels() map[string]string                   { panic("not implemeneted") }
func (*NullObjectMeta) SetLabels(_ map[string]string)                  { panic("not implemeneted") }
func (*NullObjectMeta) GetAnnotations() map[string]string              { panic("not implemeneted") }
func (*NullObjectMeta) SetAnnotations(_ map[string]string)             { panic("not implemeneted") }
func (*NullObjectMeta) GetFinalizers() []string                        { panic("not implemeneted") }
func (*NullObjectMeta) SetFinalizers(_ []string)                       { panic("not implemeneted") }
func (*NullObjectMeta) GetOwnerReferences() []metav1.OwnerReference    { panic("not implemeneted") }
func (*NullObjectMeta) SetOwnerReferences([]metav1.OwnerReference)     { panic("not implemeneted") }
func (*NullObjectMeta) GetClusterName() string                         { panic("not implemeneted") }
func (*NullObjectMeta) SetClusterName(_ string)                        { panic("not implemeneted") }
func (*NullObjectMeta) GetManagedFields() []metav1.ManagedFieldsEntry  { panic("not implemeneted") }
func (*NullObjectMeta) SetManagedFields(_ []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

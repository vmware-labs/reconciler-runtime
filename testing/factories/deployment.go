/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deployment struct {
	target *appsv1.Deployment
}

var (
	_ rtesting.Factory = (*deployment)(nil)
	_ client.Object    = (*deployment)(nil)
)

// Deprecated
func Deployment(seed ...*appsv1.Deployment) *deployment {
	var target *appsv1.Deployment
	switch len(seed) {
	case 0:
		target = &appsv1.Deployment{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &deployment{
		target: target,
	}
}

func (f *deployment) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *deployment) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *deployment) GetNamespace() string                            { panic("not implemeneted") }
func (f *deployment) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *deployment) GetName() string                                 { panic("not implemeneted") }
func (f *deployment) SetName(name string)                             { panic("not implemeneted") }
func (f *deployment) GetGenerateName() string                         { panic("not implemeneted") }
func (f *deployment) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *deployment) GetUID() types.UID                               { panic("not implemeneted") }
func (f *deployment) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *deployment) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *deployment) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *deployment) GetGeneration() int64                            { panic("not implemeneted") }
func (f *deployment) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *deployment) GetSelfLink() string                             { panic("not implemeneted") }
func (f *deployment) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *deployment) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *deployment) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *deployment) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *deployment) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *deployment) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *deployment) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *deployment) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *deployment) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *deployment) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *deployment) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *deployment) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *deployment) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *deployment) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *deployment) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *deployment) GetClusterName() string                          { panic("not implemeneted") }
func (f *deployment) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *deployment) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *deployment) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

func (f *deployment) deepCopy() *deployment {
	return Deployment(f.target.DeepCopy())
}

func (f *deployment) Create() *appsv1.Deployment {
	return f.deepCopy().target
}

func (f *deployment) CreateObject() client.Object {
	return f.Create()
}

func (f *deployment) mutation(m func(*appsv1.Deployment)) *deployment {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *deployment) NamespaceName(namespace, name string) *deployment {
	return f.mutation(func(sa *appsv1.Deployment) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *deployment) ObjectMeta(nf func(ObjectMeta)) *deployment {
	return f.mutation(func(sa *appsv1.Deployment) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *deployment) PodTemplateSpec(nf func(PodTemplateSpec)) *deployment {
	return f.mutation(func(deployment *appsv1.Deployment) {
		ptsf := PodTemplateSpecFactory(deployment.Spec.Template)
		nf(ptsf)
		deployment.Spec.Template = ptsf.Create()
	})
}

func (f *deployment) HandlerContainer(cb func(*corev1.Container)) *deployment {
	return f.PodTemplateSpec(func(pts PodTemplateSpec) {
		pts.ContainerNamed("handler", cb)
	})
}

func (f *deployment) Replicas(replicas int32) *deployment {
	return f.mutation(func(deployment *appsv1.Deployment) {
		deployment.Spec.Replicas = rtesting.Int32Ptr(replicas)
	})
}

func (f *deployment) AddSelectorLabel(key, value string) *deployment {
	return f.mutation(func(deployment *appsv1.Deployment) {
		if deployment.Spec.Selector == nil {
			deployment.Spec.Selector = &metav1.LabelSelector{}
		}
		metav1.AddLabelToSelector(deployment.Spec.Selector, key, value)
		deployment.Spec.Template = PodTemplateSpecFactory(deployment.Spec.Template).AddLabel(key, value).Create()
	})
}

func (f *deployment) StatusConditions(conditions ...ConditionFactory) *deployment {
	return f.mutation(func(deployment *appsv1.Deployment) {
		c := make([]appsv1.DeploymentCondition, len(conditions))
		for i, cg := range conditions {
			dc := cg.Create()
			c[i] = appsv1.DeploymentCondition{
				Type:    appsv1.DeploymentConditionType(dc.Type),
				Status:  dc.Status,
				Reason:  dc.Reason,
				Message: dc.Message,
			}
		}
		deployment.Status.Conditions = c
	})
}

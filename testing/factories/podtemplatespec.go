/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	corev1 "k8s.io/api/core/v1"
)

// Deprecated
type PodTemplateSpec interface {
	Create() corev1.PodTemplateSpec

	AddLabel(key, value string) PodTemplateSpec
	AddAnnotation(key, value string) PodTemplateSpec
	ObjectMeta(func(ObjectMeta)) PodTemplateSpec
	ContainerNamed(name string, cb func(*corev1.Container)) PodTemplateSpec
	Volumes(volumes ...corev1.Volume) PodTemplateSpec
	ServiceAccountName(name string) PodTemplateSpec
	ImagePullSecrets(imagePullSecrets ...corev1.LocalObjectReference) PodTemplateSpec
}

type podTemplateSpecImpl struct {
	target *corev1.PodTemplateSpec
}

// Deprecated
func PodTemplateSpecFactory(seed corev1.PodTemplateSpec) PodTemplateSpec {
	return &podTemplateSpecImpl{
		target: &seed,
	}
}

func (f *podTemplateSpecImpl) Create() corev1.PodTemplateSpec {
	return *(f.target.DeepCopy())
}

func (f *podTemplateSpecImpl) mutate(m func(*corev1.PodTemplateSpec)) PodTemplateSpec {
	m(f.target)
	return f
}

func (f *podTemplateSpecImpl) AddLabel(key, value string) PodTemplateSpec {
	return f.ObjectMeta(func(om ObjectMeta) {
		om.AddLabel(key, value)
	})
}

func (f *podTemplateSpecImpl) AddAnnotation(key, value string) PodTemplateSpec {
	return f.ObjectMeta(func(om ObjectMeta) {
		om.AddAnnotation(key, value)
	})
}

func (f *podTemplateSpecImpl) ObjectMeta(nf func(ObjectMeta)) PodTemplateSpec {
	return f.mutate(func(pts *corev1.PodTemplateSpec) {
		omf := ObjectMetaFactory(pts.ObjectMeta)
		nf(omf)
		pts.ObjectMeta = omf.Create()
	})
}

func (f *podTemplateSpecImpl) ContainerNamed(name string, cb func(*corev1.Container)) PodTemplateSpec {
	return f.mutate(func(pts *corev1.PodTemplateSpec) {
		found := false
		// check for existing container
		for i, container := range pts.Spec.Containers {
			if container.Name == name {
				found = true
				if cb != nil {
					// container mutations
					cb(&container)
					pts.Spec.Containers[i] = container
				}
				break
			}
		}
		if !found {
			// not found, create new container
			container := corev1.Container{Name: name}
			if cb != nil {
				// container mutations
				cb(&container)
			}
			pts.Spec.Containers = append(pts.Spec.Containers, container)
		}
	})
}

func (f *podTemplateSpecImpl) Volumes(volumes ...corev1.Volume) PodTemplateSpec {
	return f.mutate(func(pts *corev1.PodTemplateSpec) {
		pts.Spec.Volumes = volumes
	})
}

func (f *podTemplateSpecImpl) ServiceAccountName(name string) PodTemplateSpec {
	return f.mutate(func(pts *corev1.PodTemplateSpec) {
		pts.Spec.ServiceAccountName = name
	})
}

func (f *podTemplateSpecImpl) ImagePullSecrets(imagePullSecrets ...corev1.LocalObjectReference) PodTemplateSpec {
	return f.mutate(func(pts *corev1.PodTemplateSpec) {
		pts.Spec.ImagePullSecrets = imagePullSecrets
	})
}

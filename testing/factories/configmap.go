/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type configMap struct {
	target *corev1.ConfigMap
}

var (
	_ rtesting.Factory = (*configMap)(nil)
)

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

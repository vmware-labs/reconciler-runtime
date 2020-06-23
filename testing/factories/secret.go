/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/apis"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
)

type secret struct {
	target *corev1.Secret
}

var (
	_ rtesting.Factory = (*secret)(nil)
)

func Secret(seed ...*corev1.Secret) *secret {
	var target *corev1.Secret
	switch len(seed) {
	case 0:
		target = &corev1.Secret{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &secret{
		target: target,
	}
}

func (f *secret) deepCopy() *secret {
	return Secret(f.target.DeepCopy())
}

func (f *secret) Create() *corev1.Secret {
	return f.deepCopy().target
}

func (f *secret) CreateObject() apis.Object {
	return f.Create()
}

func (f *secret) mutation(m func(*corev1.Secret)) *secret {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *secret) NamespaceName(namespace, name string) *secret {
	return f.mutation(func(s *corev1.Secret) {
		s.ObjectMeta.Namespace = namespace
		s.ObjectMeta.Name = name
	})
}

func (f *secret) ObjectMeta(nf func(ObjectMeta)) *secret {
	return f.mutation(func(s *corev1.Secret) {
		omf := ObjectMetaFactory(s.ObjectMeta)
		nf(omf)
		s.ObjectMeta = omf.Create()
	})
}

func (f *secret) Type(t corev1.SecretType) *secret {
	return f.mutation(func(s *corev1.Secret) {
		s.Type = t
	})
}

func (f *secret) AddData(key, value string) *secret {
	return f.mutation(func(s *corev1.Secret) {
		if s.Data == nil {
			s.Data = map[string][]byte{}
		}
		s.Data[key] = []byte(value)
	})
}

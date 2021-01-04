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

type serviceAccount struct {
	target *corev1.ServiceAccount
}

var (
	_ rtesting.Factory = (*serviceAccount)(nil)
)

func ServiceAccount(seed ...*corev1.ServiceAccount) *serviceAccount {
	var target *corev1.ServiceAccount
	switch len(seed) {
	case 0:
		target = &corev1.ServiceAccount{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &serviceAccount{
		target: target,
	}
}

func (f *serviceAccount) deepCopy() *serviceAccount {
	return ServiceAccount(f.target.DeepCopy())
}

func (f *serviceAccount) Create() *corev1.ServiceAccount {
	return f.deepCopy().target
}

func (f *serviceAccount) CreateObject() client.Object {
	return f.Create()
}

func (f *serviceAccount) mutation(m func(*corev1.ServiceAccount)) *serviceAccount {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *serviceAccount) NamespaceName(namespace, name string) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *serviceAccount) ObjectMeta(nf func(ObjectMeta)) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *serviceAccount) Secrets(secrets ...string) *serviceAccount {
	return f.mutation(func(sa *corev1.ServiceAccount) {
		sa.Secrets = make([]corev1.ObjectReference, len(secrets))
		for i, secret := range secrets {
			sa.Secrets[i] = corev1.ObjectReference{Name: secret}
		}
	})
}

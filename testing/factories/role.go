/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type role struct {
	target *rbacv1.Role
}

var (
	_ rtesting.Factory = (*role)(nil)
)

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

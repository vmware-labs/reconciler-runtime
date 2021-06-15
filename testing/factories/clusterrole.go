package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterRole struct {
	target *rbacv1.ClusterRole
}

var (
	_ rtesting.Factory = (*clusterRole)(nil)
)

func ClusterRole(seed ...*rbacv1.ClusterRole) *clusterRole {
	var target *rbacv1.ClusterRole
	switch len(seed) {
	case 0:
		target = &rbacv1.ClusterRole{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &clusterRole{
		target: target,
	}
}

func (f *clusterRole) deepCopy() *clusterRole {
	return ClusterRole(f.target.DeepCopy())
}

func (f *clusterRole) Create() *rbacv1.ClusterRole {
	return f.deepCopy().target
}

func (f *clusterRole) CreateObject() client.Object {
	return f.Create()
}

func (f *clusterRole) mutation(m func(*rbacv1.ClusterRole)) *clusterRole {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *clusterRole) Name(name string) *clusterRole {
	return f.mutation(func(sa *rbacv1.ClusterRole) {
		sa.ObjectMeta.Name = name
	})
}

func (f *clusterRole) ObjectMeta(nf func(meta ObjectMeta)) *clusterRole {
	return f.mutation(func(sa *rbacv1.ClusterRole) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *clusterRole) Rules(rules []rbacv1.PolicyRule) *clusterRole {
	return f.mutation(func(s *rbacv1.ClusterRole) {
		s.Rules = rules
	})
}

func (f *clusterRole) AggregationRule(rule *rbacv1.AggregationRule) *clusterRole {
	return f.mutation(func(role *rbacv1.ClusterRole) {
		role.AggregationRule = rule
	})
}

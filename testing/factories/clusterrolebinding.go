package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterRoleBinding struct {
	target *rbacv1.ClusterRoleBinding
}

var (
	_ rtesting.Factory = (*clusterRoleBinding)(nil)
)

func ClusterRoleBinding(seed ...*rbacv1.ClusterRoleBinding) *clusterRoleBinding {
	var target *rbacv1.ClusterRoleBinding
	switch len(seed) {
	case 0:
		target = &rbacv1.ClusterRoleBinding{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &clusterRoleBinding{
		target: target,
	}
}

func (f *clusterRoleBinding) deepCopy() *clusterRoleBinding {
	return ClusterRoleBinding(f.target.DeepCopy())
}

func (f *clusterRoleBinding) Create() *rbacv1.ClusterRoleBinding {
	return f.deepCopy().target
}

func (f *clusterRoleBinding) CreateObject() client.Object {
	return f.Create()
}

func (f *clusterRoleBinding) mutation(m func(*rbacv1.ClusterRoleBinding)) *clusterRoleBinding {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *clusterRoleBinding) Name(name string) *clusterRoleBinding {
	return f.mutation(func(sa *rbacv1.ClusterRoleBinding) {
		sa.ObjectMeta.Name = name
	})
}

func (f *clusterRoleBinding) ObjectMeta(nf func(meta ObjectMeta)) *clusterRoleBinding {
	return f.mutation(func(sa *rbacv1.ClusterRoleBinding) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

func (f *clusterRoleBinding) Subjects(subjects []rbacv1.Subject) *clusterRoleBinding {
	return f.mutation(func(s *rbacv1.ClusterRoleBinding) {
		s.Subjects = subjects
	})
}

func (f clusterRoleBinding) RoleRef(ref rbacv1.RoleRef) *clusterRoleBinding {
	return f.mutation(func(s *rbacv1.ClusterRoleBinding) {
		s.RoleRef = ref
	})
}

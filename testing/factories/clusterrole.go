package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterRole struct {
	target *rbacv1.ClusterRole
}

var (
	_ rtesting.Factory = (*clusterRole)(nil)
	_ client.Object    = (*clusterRole)(nil)
)

// Deprecated
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

func (f *clusterRole) DeepCopyObject() runtime.Object                  { return f.CreateObject() }
func (f *clusterRole) GetObjectKind() schema.ObjectKind                { return f.CreateObject().GetObjectKind() }
func (f *clusterRole) GetNamespace() string                            { panic("not implemeneted") }
func (f *clusterRole) SetNamespace(namespace string)                   { panic("not implemeneted") }
func (f *clusterRole) GetName() string                                 { panic("not implemeneted") }
func (f *clusterRole) SetName(name string)                             { panic("not implemeneted") }
func (f *clusterRole) GetGenerateName() string                         { panic("not implemeneted") }
func (f *clusterRole) SetGenerateName(name string)                     { panic("not implemeneted") }
func (f *clusterRole) GetUID() types.UID                               { panic("not implemeneted") }
func (f *clusterRole) SetUID(uid types.UID)                            { panic("not implemeneted") }
func (f *clusterRole) GetResourceVersion() string                      { panic("not implemeneted") }
func (f *clusterRole) SetResourceVersion(version string)               { panic("not implemeneted") }
func (f *clusterRole) GetGeneration() int64                            { panic("not implemeneted") }
func (f *clusterRole) SetGeneration(generation int64)                  { panic("not implemeneted") }
func (f *clusterRole) GetSelfLink() string                             { panic("not implemeneted") }
func (f *clusterRole) SetSelfLink(selfLink string)                     { panic("not implemeneted") }
func (f *clusterRole) GetCreationTimestamp() metav1.Time               { panic("not implemeneted") }
func (f *clusterRole) SetCreationTimestamp(timestamp metav1.Time)      { panic("not implemeneted") }
func (f *clusterRole) GetDeletionTimestamp() *metav1.Time              { panic("not implemeneted") }
func (f *clusterRole) SetDeletionTimestamp(timestamp *metav1.Time)     { panic("not implemeneted") }
func (f *clusterRole) GetDeletionGracePeriodSeconds() *int64           { panic("not implemeneted") }
func (f *clusterRole) SetDeletionGracePeriodSeconds(*int64)            { panic("not implemeneted") }
func (f *clusterRole) GetLabels() map[string]string                    { panic("not implemeneted") }
func (f *clusterRole) SetLabels(labels map[string]string)              { panic("not implemeneted") }
func (f *clusterRole) GetAnnotations() map[string]string               { panic("not implemeneted") }
func (f *clusterRole) SetAnnotations(annotations map[string]string)    { panic("not implemeneted") }
func (f *clusterRole) GetFinalizers() []string                         { panic("not implemeneted") }
func (f *clusterRole) SetFinalizers(finalizers []string)               { panic("not implemeneted") }
func (f *clusterRole) GetOwnerReferences() []metav1.OwnerReference     { panic("not implemeneted") }
func (f *clusterRole) SetOwnerReferences([]metav1.OwnerReference)      { panic("not implemeneted") }
func (f *clusterRole) GetClusterName() string                          { panic("not implemeneted") }
func (f *clusterRole) SetClusterName(clusterName string)               { panic("not implemeneted") }
func (f *clusterRole) GetManagedFields() []metav1.ManagedFieldsEntry   { panic("not implemeneted") }
func (f *clusterRole) SetManagedFields(mf []metav1.ManagedFieldsEntry) { panic("not implemeneted") }

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

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

type clusterRoleBinding struct {
	target *rbacv1.ClusterRoleBinding
}

var (
	_ rtesting.Factory = (*clusterRoleBinding)(nil)
	_ client.Object    = (*clusterRoleBinding)(nil)
)

// Deprecated
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

func (f *clusterRoleBinding) DeepCopyObject() runtime.Object { return f.CreateObject() }
func (f *clusterRoleBinding) GetObjectKind() schema.ObjectKind {
	return f.CreateObject().GetObjectKind()
}
func (f *clusterRoleBinding) GetNamespace() string                         { panic("not implemeneted") }
func (f *clusterRoleBinding) SetNamespace(namespace string)                { panic("not implemeneted") }
func (f *clusterRoleBinding) GetName() string                              { panic("not implemeneted") }
func (f *clusterRoleBinding) SetName(name string)                          { panic("not implemeneted") }
func (f *clusterRoleBinding) GetGenerateName() string                      { panic("not implemeneted") }
func (f *clusterRoleBinding) SetGenerateName(name string)                  { panic("not implemeneted") }
func (f *clusterRoleBinding) GetUID() types.UID                            { panic("not implemeneted") }
func (f *clusterRoleBinding) SetUID(uid types.UID)                         { panic("not implemeneted") }
func (f *clusterRoleBinding) GetResourceVersion() string                   { panic("not implemeneted") }
func (f *clusterRoleBinding) SetResourceVersion(version string)            { panic("not implemeneted") }
func (f *clusterRoleBinding) GetGeneration() int64                         { panic("not implemeneted") }
func (f *clusterRoleBinding) SetGeneration(generation int64)               { panic("not implemeneted") }
func (f *clusterRoleBinding) GetSelfLink() string                          { panic("not implemeneted") }
func (f *clusterRoleBinding) SetSelfLink(selfLink string)                  { panic("not implemeneted") }
func (f *clusterRoleBinding) GetCreationTimestamp() metav1.Time            { panic("not implemeneted") }
func (f *clusterRoleBinding) SetCreationTimestamp(timestamp metav1.Time)   { panic("not implemeneted") }
func (f *clusterRoleBinding) GetDeletionTimestamp() *metav1.Time           { panic("not implemeneted") }
func (f *clusterRoleBinding) SetDeletionTimestamp(timestamp *metav1.Time)  { panic("not implemeneted") }
func (f *clusterRoleBinding) GetDeletionGracePeriodSeconds() *int64        { panic("not implemeneted") }
func (f *clusterRoleBinding) SetDeletionGracePeriodSeconds(*int64)         { panic("not implemeneted") }
func (f *clusterRoleBinding) GetLabels() map[string]string                 { panic("not implemeneted") }
func (f *clusterRoleBinding) SetLabels(labels map[string]string)           { panic("not implemeneted") }
func (f *clusterRoleBinding) GetAnnotations() map[string]string            { panic("not implemeneted") }
func (f *clusterRoleBinding) SetAnnotations(annotations map[string]string) { panic("not implemeneted") }
func (f *clusterRoleBinding) GetFinalizers() []string                      { panic("not implemeneted") }
func (f *clusterRoleBinding) SetFinalizers(finalizers []string)            { panic("not implemeneted") }
func (f *clusterRoleBinding) GetOwnerReferences() []metav1.OwnerReference  { panic("not implemeneted") }
func (f *clusterRoleBinding) SetOwnerReferences([]metav1.OwnerReference)   { panic("not implemeneted") }
func (f *clusterRoleBinding) GetClusterName() string                       { panic("not implemeneted") }
func (f *clusterRoleBinding) SetClusterName(clusterName string)            { panic("not implemeneted") }
func (f *clusterRoleBinding) GetManagedFields() []metav1.ManagedFieldsEntry {
	panic("not implemeneted")
}
func (f *clusterRoleBinding) SetManagedFields(mf []metav1.ManagedFieldsEntry) {
	panic("not implemeneted")
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

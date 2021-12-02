/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package factories

import (
	"fmt"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type testresourcenostatus struct {
	NullObjectMeta
	target *rtesting.TestResourceNoStatus
}

var (
	_ rtesting.Factory = (*testresourcenostatus)(nil)
	_ client.Object    = (*testresourcenostatus)(nil)
)

// Deprecated
func TestResourceNoStatus(seed ...*rtesting.TestResourceNoStatus) *testresourcenostatus {
	var target *rtesting.TestResourceNoStatus
	switch len(seed) {
	case 0:
		target = &rtesting.TestResourceNoStatus{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &testresourcenostatus{
		target: target,
	}
}

func (f *testresourcenostatus) DeepCopyObject() runtime.Object {
	return f.CreateObject()
}

func (f *testresourcenostatus) GetObjectKind() schema.ObjectKind {
	return f.CreateObject().GetObjectKind()
}

func (f *testresourcenostatus) deepCopy() *testresourcenostatus {
	return TestResourceNoStatus(f.target.DeepCopy())
}

func (f *testresourcenostatus) Create() *rtesting.TestResourceNoStatus {
	return f.deepCopy().target
}

func (f *testresourcenostatus) CreateObject() client.Object {
	return f.Create()
}

func (f *testresourcenostatus) mutation(m func(*rtesting.TestResourceNoStatus)) *testresourcenostatus {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *testresourcenostatus) NamespaceName(namespace, name string) *testresourcenostatus {
	return f.mutation(func(sa *rtesting.TestResourceNoStatus) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (f *testresourcenostatus) ObjectMeta(nf func(ObjectMeta)) *testresourcenostatus {
	return f.mutation(func(sa *rtesting.TestResourceNoStatus) {
		omf := ObjectMetaFactory(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Create()
	})
}

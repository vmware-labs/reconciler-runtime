/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"context"
	"testing"

	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewClient(t *testing.T) {
	scheme := runtime.NewScheme()
	rtesting.AddToScheme(scheme)
	c := NewClient("testdata/.kube/config", scheme)
	r := &rtesting.TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "my-namespace",
			Name:      "my-resource",
		},
	}
	c.(*client).client = rtesting.NewFakeClient(scheme, r.DeepCopy())
	ctx := context.TODO()

	if c.KubeRestConfig() == nil {
		t.Errorf("unexpected restconfig")
	}
	if c.Scheme() != scheme {
		t.Errorf("unexpected scheme")
	}
	if c.RESTMapper() != nil {
		// the fake client always returns nil right now
		t.Errorf("unexpected rest mapper")
	}
	if c.Discovery() == nil {
		t.Errorf("expected discovery client")
	}

	tr := &rtesting.TestResource{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: "my-namespace", Name: "my-resource"}, tr); err != nil {
		t.Errorf("error durring Get(): %v", err)
	}
	trl := &rtesting.TestResourceList{}
	if err := c.List(ctx, trl); err != nil {
		t.Errorf("error durring List(): %v", err)
	} else if len(trl.Items) != 1 {
		t.Errorf("unexpected list item")
	}
	if err := c.Update(ctx, tr); err != nil {
		t.Errorf("error durring Update(): %v", err)
	}
	if err := c.Create(ctx, tr); apierrs.IsAlreadyExists(err) {
		t.Errorf("expected AlreadyExists error durring Create(): %v", err)
	}
	if err := c.Delete(ctx, tr); err != nil {
		t.Errorf("error durring Delete(): %v", err)
	}
	if err := c.DeleteAllOf(ctx, tr); err != nil {
		t.Errorf("error durring DeleteAllOf(): %v", err)
	}
}

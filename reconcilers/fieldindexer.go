/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IndexControllersOfType(ctx context.Context, mgr ctrl.Manager, field string, owner, ownee client.Object, scheme *runtime.Scheme) error {
	gvks, _, err := scheme.ObjectKinds(owner)
	if err != nil {
		return err
	}
	ownerAPIVersion, ownerKind := gvks[0].ToAPIVersionAndKind()

	return mgr.GetFieldIndexer().IndexField(ctx, ownee, field, func(rawObj client.Object) []string {
		ownerRef := metav1.GetControllerOf(rawObj.(metav1.Object))
		if ownerRef == nil {
			return nil
		}
		if ownerRef.APIVersion != ownerAPIVersion || ownerRef.Kind != ownerKind {
			return nil
		}
		return []string{ownerRef.Name}
	})
}

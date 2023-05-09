/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tracker

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reference is modeled after corev1.ObjectReference, but omits fields
// unsupported by the tracker, and permits us to extend things in
// divergent ways.
//
// APIVersion is reduce to APIGroup as the version of a tracked object
// is irrelevant.
type Reference struct {
	// APIGroup of the referent.
	// +optional
	APIGroup string

	// Kind of the referent.
	// +optional
	Kind string

	// Namespace of the referent.
	// +optional
	Namespace string

	// Name of the referent.
	// Mutually exclusive with Selector.
	// +optional
	Name string

	// Selector of the referents.
	// Mutually exclusive with Name.
	// +optional
	Selector labels.Selector
}

// Tracker defines the interface through which an object can register
// that it is tracking another object by reference.
type Tracker interface {
	// TrackReference tells us that "obj" is tracking changes to the
	// referenced object.
	TrackReference(ref Reference, obj client.Object) error

	// TrackObject tells us that "obj" is tracking changes to the
	// referenced object.
	TrackObject(ref client.Object, obj client.Object) error

	// GetObservers returns the names of all observers for the given
	// object.
	GetObservers(obj client.Object) ([]types.NamespacedName, error)
}

// Deprecated: use Reference
func NewKey(gvk schema.GroupVersionKind, namespacedName types.NamespacedName) Key {
	return Key{
		GroupKind:      gvk.GroupKind(),
		NamespacedName: namespacedName,
	}
}

// Deprecated: use Reference
type Key struct {
	GroupKind      schema.GroupKind
	NamespacedName types.NamespacedName
}

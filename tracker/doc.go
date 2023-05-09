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

// Package tracker defines a utility to enable Reconcilers to trigger
// reconciliations when objects that are cross-referenced change, so
// that the level-based reconciliation can react to the change.  The
// prototypical cross-reference in Kubernetes is corev1.ObjectReference.
//
// Imported from https://github.com/knative/pkg/tree/db8a35330281c41c7e8e90df6059c23a36af0643/tracker
// liberated from Knative runtime dependencies and evolved to better
// fit controller-runtime patterns.
package tracker

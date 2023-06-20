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
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns an implementation of Interface that lets a Reconciler
// register a particular resource as watching an ObjectReference for
// a particular lease duration.  This watch must be refreshed
// periodically (e.g. by a controller resync) or it will expire.
func New(scheme *runtime.Scheme, lease time.Duration) Tracker {
	return &impl{
		scheme:        scheme,
		leaseDuration: lease,
	}
}

type impl struct {
	m sync.Mutex
	// exact maps from an object reference to the set of
	// keys for objects watching it.
	exact map[Reference]exactSet
	// inexact maps from a partial object reference (no name/selector) to
	// a map from watcher keys to the compiled selector and expiry.
	inexact map[Reference]inexactSet

	// scheme used to convert typed objects to GVKs
	scheme *runtime.Scheme
	// The amount of time that an object may watch another
	// before having to renew the lease.
	leaseDuration time.Duration
}

// Check that impl implements Interface.
var _ Tracker = (*impl)(nil)

// exactSet is a map from keys to expirations.
type exactSet map[types.NamespacedName]time.Time

// inexactSet is a map from keys to matchers.
type inexactSet map[types.NamespacedName]matchers

// matchers is a map from matcherKeys to matchers
type matchers map[matcherKey]matcher

// matcher holds the selector and expiry for matching tracked objects.
type matcher struct {
	// The selector to complete the match.
	selector labels.Selector

	// When this lease expires.
	expiry time.Time
}

// matcherKey holds a stringified selector and namespace.
type matcherKey struct {
	// The selector for the matcher stringified
	selector string

	// The namespace to complete the match. Empty matches cluster scope
	// and all namespaced resources.
	namespace string
}

// Track implements Interface.
func (i *impl) TrackObject(ref client.Object, obj client.Object) error {
	or, err := reference.GetReference(i.scheme, ref)
	if err != nil {
		return err
	}

	apiGroup := ""
	if l := strings.Index(or.APIVersion, "/"); l >= 0 { // is not a core resource
		apiGroup = or.APIVersion[:l]
	}

	return i.TrackReference(Reference{
		APIGroup:  apiGroup,
		Kind:      or.Kind,
		Namespace: or.Namespace,
		Name:      or.Name,
	}, obj)
}

func (i *impl) TrackReference(ref Reference, obj client.Object) error {
	invalidFields := map[string][]string{
		"Kind": validation.IsCIdentifier(ref.Kind),
	}
	// Allow apiGroup to be empty for core resources
	if ref.APIGroup != "" {
		invalidFields["APIGroup"] = validation.IsDNS1123Subdomain(ref.APIGroup)
	}
	// Allow namespace to be empty for cluster-scoped references.
	if ref.Namespace != "" {
		invalidFields["Namespace"] = validation.IsDNS1123Label(ref.Namespace)
	}
	fieldErrors := []string{}
	switch {
	case ref.Selector != nil && ref.Name != "":
		fieldErrors = append(fieldErrors, "cannot provide both Name and Selector")
	case ref.Name != "":
		invalidFields["Name"] = validation.IsDNS1123Subdomain(ref.Name)
	case ref.Selector != nil:
	default:
		fieldErrors = append(fieldErrors, "must provide either Name or Selector")
	}
	for k, v := range invalidFields {
		for _, msg := range v {
			fieldErrors = append(fieldErrors, fmt.Sprintf("%s: %s", k, msg))
		}
	}
	if len(fieldErrors) > 0 {
		sort.Strings(fieldErrors)
		return fmt.Errorf("invalid Reference:\n%s", strings.Join(fieldErrors, "\n"))
	}

	key := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}

	i.m.Lock()
	defer i.m.Unlock()

	if i.exact == nil {
		i.exact = make(map[Reference]exactSet)
	}
	if i.inexact == nil {
		i.inexact = make(map[Reference]inexactSet)
	}

	// If the reference uses Name then it is an exact match.
	if ref.Selector == nil {
		l, ok := i.exact[ref]
		if !ok {
			l = exactSet{}
		}

		// Overwrite the key with a new expiration.
		l[key] = time.Now().Add(i.leaseDuration)

		i.exact[ref] = l
		return nil
	}

	// Otherwise, it is an inexact match by selector.
	partialRef := Reference{
		APIGroup: ref.APIGroup,
		Kind:     ref.Kind,
		// Exclude the namespace and selector, they are captured in the matcher.
	}
	is, ok := i.inexact[partialRef]
	if !ok {
		is = inexactSet{}
	}
	m, ok := is[key]
	if !ok {
		m = matchers{}
	}

	// Overwrite the key with a new expiration.
	m[matcherKey{
		selector:  ref.Selector.String(),
		namespace: ref.Namespace,
	}] = matcher{
		selector: ref.Selector,
		expiry:   time.Now().Add(i.leaseDuration),
	}

	is[key] = m

	i.inexact[partialRef] = is
	return nil
}

// GetObservers implements Interface.
func (i *impl) GetObservers(obj client.Object) ([]types.NamespacedName, error) {
	or, err := reference.GetReference(i.scheme, obj)
	if err != nil {
		return nil, err
	}

	gv, err := schema.ParseGroupVersion(or.APIVersion)
	if err != nil {
		return nil, err
	}
	ref := Reference{
		APIGroup:  gv.Group,
		Kind:      or.Kind,
		Namespace: or.Namespace,
		Name:      or.Name,
	}

	keys := sets.Set[types.NamespacedName]{}

	i.m.Lock()
	defer i.m.Unlock()

	now := time.Now()

	// Handle exact matches.
	s, ok := i.exact[ref]
	if ok {
		for key, expiry := range s {
			// If the expiration has lapsed, then delete the key.
			if now.After(expiry) {
				delete(s, key)
				continue
			}
			keys.Insert(key)
		}
		if len(s) == 0 {
			delete(i.exact, ref)
		}
	}

	// Handle inexact matches.
	ref.Name = ""
	ref.Namespace = ""
	is, ok := i.inexact[ref]
	if ok {
		ls := labels.Set(obj.GetLabels())
		for key, ms := range is {
			for k, m := range ms {
				// If the expiration has lapsed, then delete the key.
				if now.After(m.expiry) {
					delete(ms, k)
					continue
				}
				// Match namespace, allowing for a cluster wide match.
				if k.namespace != "" && k.namespace != obj.GetNamespace() {
					continue
				}
				if !m.selector.Matches(ls) {
					continue
				}
				keys.Insert(key)
			}
			if len(ms) == 0 {
				delete(is, key)
			}
		}
		if len(is) == 0 {
			delete(i.inexact, ref)
		}
	}

	return keys.UnsortedList(), nil
}

/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package testing

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type clientWrapper struct {
	client                  client.Client
	CreateActions           []objectAction
	UpdateActions           []objectAction
	PatchActions            []PatchAction
	DeleteActions           []DeleteAction
	DeleteCollectionActions []DeleteCollectionAction
	StatusUpdateActions     []objectAction
	StatusPatchActions      []PatchAction
	genCount                int
	reactionChain           []Reactor
}

var _ client.Client = &clientWrapper{}

func NewFakeClient(scheme *runtime.Scheme, objs ...client.Object) *clientWrapper {
	o := make([]runtime.Object, len(objs))
	for i := range objs {
		obj := objs[i].DeepCopyObject().(client.Object)
		// default to a non-zero creation timestamp
		if obj.GetCreationTimestamp().Time.IsZero() {
			obj.SetCreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
		}
		o[i] = obj
	}
	client := &clientWrapper{
		client:                  fakeclient.NewFakeClientWithScheme(scheme, o...),
		CreateActions:           []objectAction{},
		UpdateActions:           []objectAction{},
		PatchActions:            []PatchAction{},
		DeleteActions:           []DeleteAction{},
		DeleteCollectionActions: []DeleteCollectionAction{},
		StatusUpdateActions:     []objectAction{},
		StatusPatchActions:      []PatchAction{},
		genCount:                0,
		reactionChain:           []Reactor{},
	}
	// generate names on create
	client.AddReactor("create", "*", func(action Action) (bool, runtime.Object, error) {
		if createAction, ok := action.(CreateAction); ok {
			obj := createAction.GetObject()
			if accessor, ok := obj.(metav1.ObjectMetaAccessor); ok {
				objmeta := accessor.GetObjectMeta()
				if objmeta.GetName() == "" && objmeta.GetGenerateName() != "" {
					client.genCount++
					// mutate the existing obj
					objmeta.SetName(fmt.Sprintf("%s%03d", objmeta.GetGenerateName(), client.genCount))
				}
			}
		}
		// never handle the action
		return false, nil, nil
	})
	return client
}

func (w *clientWrapper) AddReactor(verb, kind string, reaction ReactionFunc) {
	w.reactionChain = append(w.reactionChain, &clientgotesting.SimpleReactor{Verb: verb, Resource: kind, Reaction: reaction})
}

func (w *clientWrapper) PrependReactor(verb, kind string, reaction ReactionFunc) {
	w.reactionChain = append([]Reactor{&clientgotesting.SimpleReactor{Verb: verb, Resource: kind, Reaction: reaction}}, w.reactionChain...)
}

func (w *clientWrapper) objmeta(obj runtime.Object) (schema.GroupVersionResource, string, string, error) {
	objref, err := ref.GetReference(w.Scheme(), obj)
	if err != nil {
		return schema.GroupVersionResource{}, "", "", err
	}

	// NOTE kind != resource, but for this purpose it's good enough
	gvr := schema.FromAPIVersionAndKind(objref.APIVersion, objref.Kind).GroupVersion().WithResource(objref.Kind)
	return gvr, objref.Namespace, objref.Name, nil
}

func (w *clientWrapper) react(action Action) error {
	for _, reactor := range w.reactionChain {
		if !reactor.Handles(action) {
			continue
		}
		handled, _, err := reactor.React(action)
		if !handled {
			continue
		}
		return err
	}
	return nil
}

func (w *clientWrapper) Scheme() *runtime.Scheme {
	return w.client.Scheme()
}

func (w *clientWrapper) RESTMapper() meta.RESTMapper {
	return w.client.RESTMapper()
}

func (w *clientWrapper) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvr, namespace, name, err := w.objmeta(obj)
	if err != nil {
		return err
	}

	// call reactor chain
	err = w.react(clientgotesting.NewGetAction(gvr, namespace, name))
	if err != nil {
		return err
	}

	return w.client.Get(ctx, key, obj, opts...)
}

func (w *clientWrapper) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvr, _, _, err := w.objmeta(list)
	if err != nil {
		return err
	}
	gvk := schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    gvr.Resource,
	}
	listopts := &client.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(listopts)
	}

	// call reactor chain
	err = w.react(clientgotesting.NewListAction(gvr, gvk, listopts.Namespace, metav1.ListOptions{}))
	if err != nil {
		return err
	}

	return w.client.List(ctx, list, opts...)
}

func (w *clientWrapper) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	gvr, namespace, _, err := w.objmeta(obj)
	if err != nil {
		return err
	}

	// capture action
	w.CreateActions = append(w.CreateActions, clientgotesting.NewCreateAction(gvr, namespace, obj.DeepCopyObject()))

	// call reactor chain
	err = w.react(clientgotesting.NewCreateAction(gvr, namespace, obj))
	if err != nil {
		return err
	}

	return w.client.Create(ctx, obj, opts...)
}

func (w *clientWrapper) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	gvr, namespace, name, err := w.objmeta(obj)
	if err != nil {
		return err
	}

	// capture action
	w.DeleteActions = append(w.DeleteActions, clientgotesting.NewDeleteAction(gvr, namespace, name))

	// call reactor chain
	err = w.react(clientgotesting.NewDeleteAction(gvr, namespace, name))
	if err != nil {
		return err
	}

	return w.client.Delete(ctx, obj, opts...)
}

func (w *clientWrapper) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	gvr, namespace, _, err := w.objmeta(obj)
	if err != nil {
		return err
	}

	// capture action
	w.UpdateActions = append(w.UpdateActions, clientgotesting.NewUpdateAction(gvr, namespace, obj.DeepCopyObject()))

	// call reactor chain
	err = w.react(clientgotesting.NewUpdateAction(gvr, namespace, obj))
	if err != nil {
		return err
	}

	return w.client.Update(ctx, obj, opts...)
}

func (w *clientWrapper) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	gvr, _, _, err := w.objmeta(obj)
	if err != nil {
		return err
	}
	b, err := patch.Data(obj)
	if err != nil {
		return err
	}

	// capture action
	w.PatchActions = append(w.PatchActions, clientgotesting.NewPatchAction(gvr, obj.GetNamespace(), obj.GetName(), patch.Type(), b))

	// call reactor chain
	err = w.react(clientgotesting.NewPatchAction(gvr, obj.GetNamespace(), obj.GetName(), patch.Type(), b))
	if err != nil {
		return err
	}

	return w.client.Patch(ctx, obj, patch, opts...)
}

func (w *clientWrapper) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	gvr, _, _, err := w.objmeta(obj)
	if err != nil {
		return err
	}
	deleteopts := &client.DeleteAllOfOptions{}
	for _, opt := range opts {
		opt.ApplyToDeleteAllOf(deleteopts)
	}
	labels := ""
	if s := deleteopts.LabelSelector; s != nil && !s.Empty() {
		labels = s.String()
	}
	fields := ""
	if s := deleteopts.FieldSelector; s != nil && !s.Empty() {
		fields = s.String()
	}

	// capture action
	w.DeleteCollectionActions = append(w.DeleteCollectionActions, clientgotesting.NewDeleteCollectionAction(gvr, deleteopts.Namespace, metav1.ListOptions{
		LabelSelector: labels,
		FieldSelector: fields,
	}))

	// call reactor chain
	err = w.react(clientgotesting.NewDeleteCollectionAction(gvr, deleteopts.Namespace, metav1.ListOptions{
		LabelSelector: labels,
		FieldSelector: fields,
	}))
	if err != nil {
		return err
	}

	return w.client.DeleteAllOf(ctx, obj, opts...)
}

func (w *clientWrapper) Status() client.StatusWriter {
	return &statusWriterWrapper{
		statusWriter:  w.client.Status(),
		clientWrapper: w,
	}
}

type statusWriterWrapper struct {
	statusWriter  client.StatusWriter
	clientWrapper *clientWrapper
}

var _ client.StatusWriter = &statusWriterWrapper{}

func (w *statusWriterWrapper) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	gvr, namespace, _, err := w.clientWrapper.objmeta(obj)
	if err != nil {
		return err
	}

	// capture action
	w.clientWrapper.StatusUpdateActions = append(w.clientWrapper.StatusUpdateActions, clientgotesting.NewUpdateSubresourceAction(gvr, "status", namespace, obj.DeepCopyObject()))

	// call reactor chain
	err = w.clientWrapper.react(clientgotesting.NewUpdateSubresourceAction(gvr, "status", namespace, obj))
	if err != nil {
		return err
	}

	return w.statusWriter.Update(ctx, obj, opts...)
}

func (w *statusWriterWrapper) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	gvr, _, _, err := w.clientWrapper.objmeta(obj)
	if err != nil {
		return err
	}
	b, err := patch.Data(obj)
	if err != nil {
		return err
	}

	// capture action
	w.clientWrapper.StatusPatchActions = append(w.clientWrapper.StatusPatchActions, clientgotesting.NewPatchSubresourceAction(gvr, obj.GetNamespace(), obj.GetName(), patch.Type(), b, "status"))

	// call reactor chain
	err = w.clientWrapper.react(clientgotesting.NewPatchSubresourceAction(gvr, obj.GetNamespace(), obj.GetName(), patch.Type(), b, "status"))
	if err != nil {
		return err
	}

	return w.statusWriter.Patch(ctx, obj, patch, opts...)
}

// InduceFailure is used in conjunction with reconciler test's WithReactors field.
// Tests that want to induce a failure in a testcase of a reconciler test would add:
//
//	WithReactors: []rtesting.ReactionFunc{
//	   // Makes calls to create stream return an error.
//	   rtesting.InduceFailure("create", "Stream"),
//	},
func InduceFailure(verb, kind string, o ...InduceFailureOpts) ReactionFunc {
	var opts *InduceFailureOpts
	switch len(o) {
	case 0:
		opts = &InduceFailureOpts{}
	case 1:
		opts = &o[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one InduceFailureOpts, got %v", o))
	}
	return func(action Action) (handled bool, ret runtime.Object, err error) {
		if !action.Matches(verb, kind) {
			return false, nil, nil
		}
		if opts.Namespace != "" && opts.Namespace != action.GetNamespace() {
			return false, nil, nil
		}
		if opts.Name != "" {
			switch a := action.(type) {
			case namedAction: // matches GetAction, PatchAction, DeleteAction
				if opts.Name != a.GetName() {
					return false, nil, nil
				}
			case objectAction: // matches CreateAction, UpdateAction
				obj, ok := a.GetObject().(client.Object)
				if ok && opts.Name != obj.GetName() {
					return false, nil, nil
				}
			}
		}
		if opts.SubResource != "" && opts.SubResource != action.GetSubresource() {
			return false, nil, nil
		}
		err = opts.Error
		if err == nil {
			err = fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
		}
		return true, nil, err
	}
}

type namedAction interface {
	Action
	GetName() string
}

type objectAction interface {
	Action
	GetObject() runtime.Object
}

type InduceFailureOpts struct {
	Error       error
	Namespace   string
	Name        string
	SubResource string
}

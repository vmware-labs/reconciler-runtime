/*
Copyright 2022 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v3"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// AdmissionWebhookAdapter allows using sub reconcilers to process admission webhooks. The full
// suite of sub reconcilers are available, however, behavior that is generally not accepted within
// a webhook is discouraged. For example, new requests against the API server are discouraged
// (reading from an informer is ok), mutation requests against the API Server can cause a loop with
// the webhook processing its own requests.
//
// All requests are allowed by default unless the response.Allowed field is explicitly unset, or
// the reconciler returns an error. The raw admission request and response can be retrieved from
// the context via the RetrieveAdmissionRequest and RetrieveAdmissionResponse methods,
// respectively. The Result typically returned by a reconciler is unused.
//
// The request object is unmarshaled from the request object for most operations, and the old
// object for delete operations. If the webhhook handles multiple resources or versions of the
// same resource with different shapes, use of an unstructured type is recommended.
//
// If the resource being reconciled is mutated and the response does not already define a patch, a
// json patch is computed for the mutation and set on the response.
type AdmissionWebhookAdapter struct {
	// Name used to identify this reconciler.  Defaults to `{Type}AdmissionWebhookAdapter`.  Ideally
	// unique, but not required to be so.
	//
	// +optional
	Name string

	// Type of resource to reconcile. If the webhook can handle multiple types, use
	// *unstructured.Unstructured{}.
	Type client.Object

	// Reconciler is called for each reconciler request with the resource being reconciled.
	// Typically, Reconciler is a Sequence of multiple SubReconcilers.
	Reconciler SubReconciler

	Config Config
}

func (r *AdmissionWebhookAdapter) Build() *admission.Webhook {
	name := r.Name
	if name == "" {
		name = fmt.Sprintf("%sAdmissionWebhookAdapter", typeName(r.Type))
	}
	return &admission.Webhook{
		Handler: r,
		WithContextFunc: func(ctx context.Context, req *http.Request) context.Context {
			log := crlog.FromContext(ctx).
				WithName("controller-runtime.webhook.webhooks").
				WithName(name).
				WithValues(
					"webhook", req.URL.Path,
				)
			ctx = logr.NewContext(ctx, log)

			ctx = WithStash(ctx)

			ctx = StashConfig(ctx, r.Config)
			ctx = StashOriginalConfig(ctx, r.Config)
			ctx = StashResourceType(ctx, r.Type)
			ctx = StashOriginalResourceType(ctx, r.Type)
			ctx = StashHTTPRequest(ctx, req)

			return ctx
		},
	}
}

// Handle implements admission.Handler
func (r *AdmissionWebhookAdapter) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logr.FromContextOrDiscard(ctx).
		WithValues(
			"UID", req.UID,
			"kind", req.Kind,
			"resource", req.Resource,
			"operation", req.Operation,
		)
	ctx = logr.NewContext(ctx, log)

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			UID: req.UID,
			// allow by default, a reconciler can flip this off, or return an err
			Allowed: true,
		},
	}

	ctx = StashAdmissionRequest(ctx, req)
	ctx = StashAdmissionResponse(ctx, resp)

	// defined for compatibility since this is not a reconciler
	ctx = StashRequest(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: req.Namespace,
			Name:      req.Name,
		},
	})

	if err := r.reconcile(ctx, req, resp); err != nil {
		log.Error(err, "reconcile error")
		resp.Allowed = false
		if resp.Result == nil {
			resp.Result = &metav1.Status{
				Code:    500,
				Message: err.Error(),
			}
		}
	}

	return *resp
}

func (r *AdmissionWebhookAdapter) reconcile(ctx context.Context, req admission.Request, resp *admission.Response) error {
	log := logr.FromContextOrDiscard(ctx)

	resource := r.Type.DeepCopyObject().(client.Object)
	resourceBytes := req.Object.Raw
	if req.Operation == admissionv1.Delete {
		resourceBytes = req.OldObject.Raw
	}
	if err := json.Unmarshal(resourceBytes, resource); err != nil {
		return err
	}

	if defaulter, ok := resource.(webhook.Defaulter); ok {
		// resource.Default()
		defaulter.Default()
	}

	originalResource := resource.DeepCopyObject()
	if _, err := r.Reconciler.Reconcile(ctx, resource); err != nil {
		return err
	}

	if resp.Patches == nil && resp.Patch == nil && resp.PatchType == nil && !equality.Semantic.DeepEqual(originalResource, resource) {
		// add patch to response

		mutationBytes, err := json.Marshal(resource)
		if err != nil {
			return err
		}

		// create patch using jsonpatch v3 since it preserves order, then convert back to v2 used by AdmissionResponse
		patch, err := jsonpatch.CreatePatch(resourceBytes, mutationBytes)
		if err != nil {
			return err
		}
		data, _ := json.Marshal(patch)
		_ = json.Unmarshal(data, &resp.Patches)
		log.Info("mutating resource", "patch", resp.Patches)
	}

	return nil
}

const admissionRequestStashKey StashKey = "reconciler-runtime:admission-request"
const admissionResponseStashKey StashKey = "reconciler-runtime:admission-response"
const httpRequestStashKey StashKey = "reconciler-runtime:http-request"

func StashAdmissionRequest(ctx context.Context, req admission.Request) context.Context {
	return context.WithValue(ctx, admissionRequestStashKey, req)
}

// RetrieveAdmissionRequest returns the admission Request from the context, or empty if not found.
func RetrieveAdmissionRequest(ctx context.Context) admission.Request {
	value := ctx.Value(admissionRequestStashKey)
	if req, ok := value.(admission.Request); ok {
		return req
	}
	return admission.Request{}
}

func StashAdmissionResponse(ctx context.Context, resp *admission.Response) context.Context {
	return context.WithValue(ctx, admissionResponseStashKey, resp)
}

// RetrieveAdmissionResponse returns the admission Response from the context, or nil if not found.
func RetrieveAdmissionResponse(ctx context.Context) *admission.Response {
	value := ctx.Value(admissionResponseStashKey)
	if resp, ok := value.(*admission.Response); ok {
		return resp
	}
	return nil
}

func StashHTTPRequest(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestStashKey, req)
}

// RetrieveHTTPRequest returns the http Request from the context, or nil if not found.
func RetrieveHTTPRequest(ctx context.Context) *http.Request {
	value := ctx.Value(httpRequestStashKey)
	if req, ok := value.(*http.Request); ok {
		return req
	}
	return nil
}

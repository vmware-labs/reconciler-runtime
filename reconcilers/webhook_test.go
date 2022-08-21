/*
Copyright 2020 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	dieadmissionv1 "dies.dev/apis/admission/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/internal/resources"
	"github.com/vmware-labs/reconciler-runtime/internal/resources/dies"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestAdmissionWebhookAdapter(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-resource"

	scheme := runtime.NewScheme()
	_ = resources.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	resource := dies.TestResourceBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		StatusDie(func(d *dies.TestResourceStatusDie) {
			d.ConditionsDie(
				diemetav1.ConditionBlank.Type(apis.ConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing"),
			)
		})

	requestUID := types.UID("9deefaa1-2c90-4f40-9c7b-3f5c1fd75dde")

	httpRequest := &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Path: "/path"},
	}
	request := dieadmissionv1.AdmissionRequestBlank.
		Namespace(testNamespace).
		Name(testName).
		UID(requestUID).
		Operation(admissionv1.Create)
	response := dieadmissionv1.AdmissionResponseBlank.
		Allowed(true)

	wts := rtesting.AdmissionWebhookTests{
		"allowed by default with no mutation": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return nil
						},
					}
				},
			},
		},
		"mutations generate patches in response": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      "/spec/fields",
						Value: map[string]interface{}{
							"hello": "world",
						},
					},
				},
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							resource.Spec.Fields = map[string]string{
								"hello": "world",
							}
							return nil
						},
					}
				},
			},
		},
		"reconcile errors return http errors": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.
					Allowed(false).
					ResultDie(func(d *diemetav1.StatusDie) {
						d.Code(500)
						d.Message("reconcile error")
					}).
					DieRelease(),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return fmt.Errorf("reconcile error")
						},
					}
				},
			},
		},
		"invalid json returns http errors": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(runtime.RawExtension{
						Raw: []byte("{"),
					}).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.
					Allowed(false).
					ResultDie(func(d *diemetav1.StatusDie) {
						d.Code(500)
						d.Message("unexpected end of JSON input")
					}).
					DieRelease(),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							return nil
						},
					}
				},
			},
		},
		"delete operations load resource from old object": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Operation(admissionv1.Delete).
					OldObject(
						resource.
							SpecDie(func(d *dies.TestResourceSpecDie) {
								d.AddField("hello", "world")
							}).
							DieReleaseRawExtension(),
					).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							if resource.Spec.Fields["hello"] != "world" {
								t.Errorf("expected field %q to have value %q", "hello", "world")
							}
							return nil
						},
					}
				},
			},
		},
		"config observations are asserted, should be extremely rare in standard use": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			GivenObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      testName,
					},
				},
			},
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{
						Namespace: testNamespace,
						Name:      testName,
					},
					Tracked: tracker.Key{
						GroupKind: schema.GroupKind{
							Group: "",
							Kind:  "ConfigMap",
						},
						NamespacedName: types.NamespacedName{
							Namespace: testNamespace,
							Name:      testName,
						},
					},
				},
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							c := reconcilers.RetrieveConfigOrDie(ctx)
							cm := &corev1.ConfigMap{}
							if err := c.TrackAndGet(ctx, types.NamespacedName{Namespace: testNamespace, Name: testName}, cm); err != nil {
								t.Errorf("unexpected client error: %s", err)
							}
							return nil
						},
					}
				},
			},
		},
		"context is defined": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			HTTPRequest: httpRequest,
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, _ *resources.TestResource) error {
							actualRequest := reconcilers.RetrieveAdmissionRequest(ctx)
							expectedRequest := admission.Request{
								AdmissionRequest: request.
									Object(resource.DieReleaseRawExtension()).
									DieRelease(),
							}
							if diff := cmp.Diff(actualRequest, expectedRequest); diff != "" {
								t.Errorf("expected stashed admission request to match actual request: %s", diff)
							}

							actualResponse := reconcilers.RetrieveAdmissionResponse(ctx)
							expectedResponse := &admission.Response{
								AdmissionResponse: dieadmissionv1.AdmissionResponseBlank.
									Allowed(true).
									UID(requestUID).
									DieRelease(),
							}
							if diff := cmp.Diff(actualResponse, expectedResponse); diff != "" {
								t.Errorf("expected stashed admission response to match actual response: %s", diff)
							}

							actualHTTPRequest := reconcilers.RetrieveHTTPRequest(ctx)
							expectedHTTPRequest := httpRequest
							if actualHTTPRequest != expectedHTTPRequest {
								t.Errorf("expected stashed http request to match actual request")
							}

							return nil
						},
					}
				},
			},
		},
		"context is stashable": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Metadata: map[string]interface{}{
				"SubReconciler": func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return reconcilers.Sequence{
						&reconcilers.SyncReconciler{
							Sync: func(ctx context.Context, _ *resources.TestResource) error {
								// StashValue will panic if context is not setup for stashing
								reconcilers.StashValue(ctx, reconcilers.StashKey("greeting"), "hello")

								return nil
							},
						},
						&reconcilers.SyncReconciler{
							Sync: func(ctx context.Context, _ *resources.TestResource) error {
								// StashValue will panic if context is not setup for stashing
								greeting := reconcilers.RetrieveValue(ctx, reconcilers.StashKey("greeting"))
								if greeting != "hello" {
									t.Errorf("unexpected stash value retrieved")
								}

								return nil
							},
						},
					}
				},
			},
		},
		"context can be augmented in Prepare and accessed in Cleanup": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(resource.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Prepare: func(t *testing.T, ctx context.Context, tc *rtesting.AdmissionWebhookTestCase) (context.Context, error) {
				key := "test-key"
				value := "test-value"
				ctx = context.WithValue(ctx, key, value)

				tc.Metadata["SubReconciler"] = func(t *testing.T, c reconcilers.Config) reconcilers.SubReconciler {
					return &reconcilers.SyncReconciler{
						Sync: func(ctx context.Context, resource *resources.TestResource) error {
							if v := ctx.Value(key); v != value {
								t.Errorf("expected %s to be in context", key)
							}
							return nil
						},
					}
				}
				tc.CleanUp = func(t *testing.T, ctx context.Context, tc *rtesting.AdmissionWebhookTestCase) error {
					if v := ctx.Value(key); v != value {
						t.Errorf("expected %s to be in context", key)
					}
					return nil
				}

				return ctx, nil
			},
		},
	}

	wts.Run(t, scheme, func(t *testing.T, wtc *rtesting.AdmissionWebhookTestCase, c reconcilers.Config) *admission.Webhook {
		return (&reconcilers.AdmissionWebhookAdapter{
			Type:       &resources.TestResource{},
			Reconciler: wtc.Metadata["SubReconciler"].(func(*testing.T, reconcilers.Config) reconcilers.SubReconciler)(t, c),
			Config:     c,
		}).Build()
	})
}

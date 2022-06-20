# reconciler-runtime <!-- omit in toc -->

![CI](https://github.com/vmware-labs/reconciler-runtime/workflows/CI/badge.svg?branch=main)
[![GoDoc](https://godoc.org/github.com/vmware-labs/reconciler-runtime?status.svg)](https://godoc.org/github.com/vmware-labs/reconciler-runtime)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmware-labs/reconciler-runtime)](https://goreportcard.com/report/github.com/vmware-labs/reconciler-runtime)
[![codecov](https://codecov.io/gh/vmware-labs/reconciler-runtime/branch/main/graph/badge.svg)](https://codecov.io/gh/vmware-labs/reconciler-runtime)

`reconciler-runtime` is an opinionated framework for authoring and testing Kubernetes reconcilers using [`controller-runtime`](https://github.com/kubernetes-sigs/controller-runtime) project. `controller-runtime` provides infrastructure for creating and operating controllers, but provides little support for the business logic of implementing a reconciler within the controller. The [`Reconciler` interface](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler) provided by `controller-runtime` is the primary handoff point with `reconciler-runtime`.

<!-- ToC managed by https://marketplace.visualstudio.com/items?itemName=yzhang.markdown-all-in-one -->
- [Reconcilers](#reconcilers)
	- [ResourceReconciler](#resourcereconciler)
	- [AggregateReconciler](#aggregatereconciler)
	- [SubReconciler](#subreconciler)
		- [SyncReconciler](#syncreconciler)
		- [ChildReconciler](#childreconciler)
	- [Higher-order Reconcilers](#higher-order-reconcilers)
		- [CastResource](#castresource)
		- [Sequence](#sequence)
		- [WithConfig](#withconfig)
		- [WithFinalizer](#withfinalizer)
	- [AdmissionWebhookAdapter](#admissionwebhookadapter)
- [Testing](#testing)
	- [ReconcilerTests](#reconcilertests)
	- [SubReconcilerTests](#subreconcilertests)
	- [AdmissionWebhookTests](#admissionwebhooktests)
	- [ExpectConfig](#expectconfig)
- [Utilities](#utilities)
	- [Config](#config)
	- [Stash](#stash)
	- [Tracker](#tracker)
	- [Status](#status)
	- [Finalizers](#finalizers)
	- [ResourceManager](#resourcemanager)
- [Breaking Changes](#breaking-changes)
- [Contributing](#contributing)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Reconcilers

<a name="parentreconciler" />

### ResourceReconciler

A [`ResourceReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#ResourceReconciler) (formerly ParentReconciler) is responsible for orchestrating the reconciliation of a single resource. The reconciler delegates the manipulation of other resources to SubReconcilers.

The resource reconciler is responsible for:
- fetching the resource being reconciled
- creating a stash to pass state between sub reconcilers
- passing the resource to each sub reconciler in turn
- initialize conditions on the status by calling status.InitializeConditions() if defined
- normalizing the .status.conditions[].lastTransitionTime for status conditions that are metav1.Condition (the previous timestamp is preserved if the condition is otherwise unchanged)
- reflects the observed generation on the status
- updates the resource status if it was modified
- logging the reconcilers activities
- records events for mutations and errors

The implementor is responsible for:
- defining the set of sub reconcilers

**Example:**

Resource reconcilers tend to be quite simple, as they delegate their work to sub reconcilers. We'll use an example from projectriff of the Function resource, which uses Kpack to build images from a git repo. In this case the FunctionTargetImageReconciler resolves the target image for the function, and FunctionChildImageReconciler creates a child Kpack Image resource based on the resolve value. 

```go
func FunctionReconciler(c reconcilers.Config) *reconcilers.ResourceReconciler {
	return &reconcilers.ResourceReconciler{
		Name: "Function",
		Type: &buildv1alpha1.Function{},
		Reconciler: reconcilers.Sequence{
			FunctionTargetImageReconciler(c),
			FunctionChildImageReconciler(c),
		},

		Config: c,
	}
}
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/build/function_reconciler.go#L39-L51)

**Recommended RBAC:**

Replace `<group>` and `<resource>` with values for the reconciled resource type.

```go
// +kubebuilder:rbac:groups=<group>,resources=<resource>,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=<group>,resources=<resource>/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
```

or

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: # any name that is bound to the ServiceAccount used by the client
rules:
- apiGroups: ["<group>"]
  resources: ["<resource>"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["<group>"]
  resources: ["<resource>/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["core"]
  resources: ["events"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### AggregateReconciler

An [`AggregateReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#AggregateReconciler) is responsible for synthesizing a single resource, aggregated from other state. The AggregateReconciler is a fusion of the [ResourceReconciler](#resourcereconciler) and [ChildReconciler](#childreconciler). Instead of operating on all resources of a type, it will only operate on a specific resource identified by the type and request (namespace and name). Unlike the child reconciler, the "parent" and "child" resources are the same.

The aggregate reconciler is responsible for:
- fetching the resource being reconciled
- creating a stash to pass state between sub reconcilers
- passing the resource to each sub reconciler in turn
- creates the resource if it does not exist
- updates the resource if it drifts from the desired state
- deletes the resource if no longer desired
- logging the reconcilers activities
- records events for mutations and errors

The implementor is responsible for:
- specifying the type, namespace and name of the aggregate resource
- defining the desired state
- indicating if two resources are semantically equal
- merging the actual resource with the desired state (often as simple as copying the spec and labels)

**Example:**

Aggregate reconcilers resemble a simplified child reconciler with many of the same methods combined directly into a parent reconciler. The `Reconcile` method is used to collect reference data and the `DesiredResource` method defines the desired state. Unlike with a child reconciler, the desired resource may be a direct mutation of the argument.

In the example, we are controlling and existing `ValidatingWebhookConfiguration` named `my-trigger` (defined by `Request`). Based on other state in the cluster, the Reconcile method delegates to `DeriveWebhookRules()` to stash the rules for the webhook. Those rules are retrieved in the `DesiredResource` method, augmenting the `ValidatingWebhookConfiguration`. The `SemanticEquals` detects when the desired webhook config has changed in a meaningful way from the actual resource and needs to be updated,  and `MergeBeforeUpdate` is responsible for merging the desired state into the actual resource, which is then updated on the api server.

The resulting `ValidatingWebhookConfiguration` will have the current desired rules defined by this reconciler, combined with existing state like the location of the webhook server, and other policies.

```go
// AdmissionTriggerReconciler reconciles a ValidatingWebhookConfiguration object to
// dynamically be notified of resource mutations. A less reliable, but potentially more
// efficient than an informer watching each tracked resource.
func AdmissionTriggerReconciler(c reconcilers.Config) *reconcilers.AggregateReconciler {
	return &reconcilers.AggregateReconciler{
		Name:     "AdmissionTrigger",
		Type:     &admissionregistrationv1.ValidatingWebhookConfiguration{},
		ListType: &admissionregistrationv1.ValidatingWebhookConfigurationList{},
		Request:  reconcile.Request{
			NamesspacedName: types.NamesspacedName{
				// no namespace since ValidatingWebhookConfiguration is cluster scoped
				Name: "my-trigger",
			},
		},
		Reconciler: reconcilers.Sequence{
			DeriveWebhookRules(),
		},
		DesiredResource: func(ctx context.Context, resource *admissionregistrationv1.ValidatingWebhookConfiguration) (client.Object, error) {
			// assumes other aspects of the webhook config are part of a preexisting
			// install, and that there is a server ready to receive the requests.
			rules := RetrieveWebhookRules(ctx)
			resource.Webhooks[0].Rules = rules
			return resource, nil
		},
		SemanticEquals: func(a1, a2 *admissionregistrationv1.ValidatingWebhookConfiguration) bool {
			return equality.Semantic.DeepEqual(a1.Webhooks[0].Rules, a2.Webhooks[0].Rules)
		},
		MergeBeforeUpdate: func(current, desired *admissionregistrationv1.ValidatingWebhookConfiguration) {
			current.Webhooks[0].Rules = desired.Webhooks[0].Rules
		},
		Sanitize: func(resource *admissionregistrationv1.ValidatingWebhookConfiguration) []admissionregistrationv1.RuleWithOperations {
			return resource.Webhooks[0].Rules
		},

		Config: c,
	}
}
```
[full source](https://github.com/scothis/servicebinding-runtime/blob/8ae0b1fb8b7a37856fa18171bc34e3462c35348b/controllers/webhook_controller.go#L171-L221)

**Recommended RBAC:**

Replace `<group>` and `<resource>` with values for the reconciled resource type.

```go
// +kubebuilder:rbac:groups=<group>,resources=<resource>,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
```

or

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: # any name that is bound to the ServiceAccount used by the client
rules:
- apiGroups: ["<group>"]
  resources: ["<resource>"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["core"]
  resources: ["events"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### SubReconciler

The [`SubReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#SubReconciler) interface defines the contract between the host and sub reconcilers.

#### SyncReconciler

The [`SyncReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#SyncReconciler) is the minimal type-aware sub reconciler. It is used to manage a portion of the resource reconciliation that is custom, or whose behavior is not covered by another sub reconciler type. Common uses include looking up reference data for the reconciliation, or controlling APIs that are not Kubernetes resources.

When a resource is deleted that has pending finalizers, the Finalize method is called instead of the Sync method. If the SyncDuringFinalization field is true, the Sync method will also by called. If creating state that must be manually cleaned up, it is the users responsibility to define and clear finalizers. Using the [finalizer helper methods](#finalizers) is strongly encouraged with working under a [ResourceReconciler](#resourcereconciler).

**Example:**

While sync reconcilers have the ability to do anything a reconciler can do, it's best to keep them focused on a single goal, letting the resource reconciler structure multiple sub reconcilers together. In this case, we use the reconciled resource and the client to resolve the target image and stash the value on the resource's status. The status is a good place to stash simple values that can be made public. More [advanced forms of stashing](#stash) are also available. Learn more about [status and its contract](#status).

```go
func FunctionTargetImageReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "TargetImage",
		Sync: func(ctx context.Context, resource *buildv1alpha1.Function) error {
			log := logr.FromContextOrDiscard(ctx)

			targetImage, err := resolveTargetImage(ctx, c.Client, resource)
			if err != nil {
				return err
			}
			resource.Status.MarkImageResolved()
			resource.Status.TargetImage = targetImage
			return nil
		},
	}
}
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/build/function_reconciler.go#L53-L74)

#### ChildReconciler

The [`ChildReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#ChildReconciler) is a sub reconciler that is responsible for managing a single controlled resource. Within a child reconciler, the reconciled resource is referred to as the parent resource to avoid ambiguity with the child resource. A developer defines their desired state for the child resource (if any), and the reconciler creates/updates/deletes the resource to match the desired state. The child resource is also used to update the parent's status. Mutations and errors are recorded for the parent.

The ChildReconciler is responsible for:
- looking up an existing child
- creating/updating/deleting the child resource based on the desired state
- setting the owner reference on the child resource (when not using a finalizer)
- logging the reconcilers activities
- recording child mutations and errors for the parent resource
- adapting to child resource changes applied by mutating webhooks
- adding and clearing of a finalizer, if specified

The implementor is responsible for:
- defining the desired resource
- indicating if two resources are semantically equal
- merging the actual resource with the desired state (often as simple as copying the spec and labels)
- updating the parent's status from the child
- defining the status subresource [according to the contract](#status) 

When a finalizer is defined, the parent resource is patched to add the finalizer before creating the child; it is removed after the child is deleted. If the parent resource is pending deletion, the desired child method is not called, and existing children are deleted.

Using a finalizer means that the child resource will not use an owner reference. The OurChild method must be implemented in a way that can uniquely and unambiguously identify the child that this parent resource is responsible for from any other resources of the same kind. The child resource is tracked explicitly.

> Warning: It is crucial that each ChildReconciler using a finalizer have a unique and stable finalizer name. Two reconcilers that use the same finalizer, or a reconciler that changed the name of its finalizer, may leak the child resource when the parent is deleted, or the parent resource may never terminate.

**Example:**

Now it's time to create the child Image resource that will do the work of building our Function. This reconciler looks more more complex than what we have seen so far, each function on the reconciler provides a focused hook into the lifecycle being orchestrated by the ChildReconciler.

```go
func FunctionChildImageReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	return &reconcilers.ChildReconciler{
		Name:          "ChildImage",
		ChildType:     &kpackbuildv1alpha1.Image{},
		ChildListType: &kpackbuildv1alpha1.ImageList{},

		DesiredChild: func(ctx context.Context, parent *buildv1alpha1.Function) (*kpackbuildv1alpha1.Image, error) {
			if parent.Spec.Source == nil {
				// don't create an Image, and delete any existing Image
				return nil, nil
			}

			child := &kpackbuildv1alpha1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Labels: reconcilers.MergeMaps(parent.Labels, map[string]string{
						buildv1alpha1.FunctionLabelKey: parent.Name,
					}),
					Annotations:  make(map[string]string),
					// Name or GenerateName are supported
					GenerateName: fmt.Sprintf("%s-function-", parent.Name),
					Namespace:    parent.Namespace,
				},
				Spec: kpackbuildv1alpha1.ImageSpec{
					Tag: parent.Status.TargetImage, // value set by sync reconciler
					// ... abbreviated
				},
			}

			return child, nil
		},
		SemanticEquals: func(r1, r2 *kpackbuildv1alpha1.Image) bool {
			// if the two resources are semantically equal, then we don't need
			// to update the server
			return equality.Semantic.DeepEqual(r1.Spec, r2.Spec) &&
				equality.Semantic.DeepEqual(r1.Labels, r2.Labels)
		},
		MergeBeforeUpdate: func(actual, desired *kpackbuildv1alpha1.Image) {
			// mutate actual resource with desired state
			actual.Labels = desired.Labels
			actual.Spec = desired.Spec
		},
		ReflectChildStatusOnParent: func(parent *buildv1alpha1.Function, child *kpackbuildv1alpha1.Image, err error) {
			// child is the value of the freshly created/updated/deleted child
			// resource as returned from the api server

			// If a fixed desired resource name is used instead of a generated
			// name, check if the err is because the resource already exists.
			// The ChildReconciler will not claim ownership of another resource.
			//
			// See https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/core/deployer_reconciler.go#L277-L283

			if child == nil {
				// image was deleted
				parent.Status.LatestImage = parent.Status.TargetImage
				parent.Status.MarkBuildNotUsed()
			} else {
				// image was created/updated/unchanged
				parent.Status.KpackImageRef = refs.NewTypedLocalObjectReferenceForObject(child, c.Scheme)
				parent.Status.LatestImage = child.Status.LatestImage
				parent.Status.PropagateKpackImageStatus(&child.Status)
			}
		},
		Sanitize: func(child *kpackbuildv1alpha1.Image) interface{} {
			// log only the resources spec. If the resource contained sensitive
			// values (like a Secret) we'd remove them here so they don't end
			// up in our logs
			return child.Spec
		},
	}
}
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/build/function_reconciler.go#L76-L151)

**Recommended RBAC:**

Replace `<group>` and `<resource>` with values for the child type.

```go
// +kubebuilder:rbac:groups=<group>,resources=<resource>,verbs=get;list;watch;create;update;patch;delete
```

or

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: # any name that is bound to the ServiceAccount used by the client
rules:
- apiGroups: ["<group>"]
  resources: ["<resource>"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```


### Higher-order Reconcilers

Higher order reconcilers are SubReconcilers that do not perform work directly, but instead compose other SubReconcilers in new patterns.

<a name="castparent" />

#### CastResource

A [`CastResource`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#CastResource) (formerly CastParent) casts the ResourceReconciler's type by projecting the resource data onto a new struct. Casting the reconciled resource is useful to create cross cutting reconcilers that can operate on common portion of multiple  resource kinds, commonly referred to as a duck type.

**Example:**

```go
func FunctionReconciler(c reconcilers.Config) *reconcilers.ResourceReconciler {
	return &reconcilers.ResourceReconciler{
		Name: "Function",
		Type: &buildv1alpha1.Function{},
		Reconciler: reconcilers.Sequence{
			&reconcilers.CastResource{
				Type: &duckv1alpha1.ImageRef{},
				// Reconciler that now operates on the ImageRef type. This SubReconciler is likely
				// shared between multiple ResourceReconcilers that operate on different types,
				// otherwise it would be easier to work directly with the Function type directly.
				Reconciler: &reconcilers.SyncReconciler{
					Sync: func(ctx context.Context, resource *duckv1alpha1.ImageRef) error {
						// do something with the duckv1alpha1.ImageRef instead of a buildv1alpha1.Function
						return nil
					},
				},
			},
			FunctionChildImageReconciler(c),
		},

		Config: c,
	}
}
```

#### Sequence

A [`Sequence`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Sequence) composes multiple SubReconcilers as a single SubReconciler. Each sub reconciler is called in turn, aggregating the result of each sub reconciler. A reconciler returning an error will interrupt the sequence.

**Example:**

A Sequence is commonly used in a ResourceReconciler, but may be used anywhere a SubReconciler is accepted. 

```go
func FunctionReconciler(c reconcilers.Config) *reconcilers.ResourceReconciler {
	return &reconcilers.ResourceReconciler{
		Name: "Function",
		Type: &buildv1alpha1.Function{},
		Reconciler: reconcilers.Sequence{
			FunctionTargetImageReconciler(c),
			FunctionChildImageReconciler(c),
		},

		Config: c,
	}
}
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/build/function_reconciler.go#L39-L51)

#### WithConfig

[`WithConfig`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#WithConfig) overrides the config that nested reconcilers consume. The config can be retrieved from the context via [`RetrieveConfig`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveConfig). For interactions with the reconciled resource, the config originally used to load that resource should be used, which can be retrieved from the context via [`RetrieveOriginalConfig`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveOriginalConfig).

**Example:**

`WithConfig` can be used to change the REST Config backing the clients. This could be to make requests to the same cluster with a user defined service account, or target an entirely different Kubernetes cluster.

```go
func SwapRESTConfig(rc *rest.Config) *reconcilers.SubReconciler {
	return &reconcilers.WithConfig{
		Reconciler: reconcilers.Sequence{
			LookupReferenceDataReconciler(),
			DoSomethingChildReconciler(),
		},

		Config: func(ctx context.Context, c reconciler.Config) (reconciler.Config, error ) {
			// the rest config could also be stashed from a lookup in a SyncReconciler based on a dynamic value
			cl, err := clusters.New(rc)
			if err != nil {
				return reconciler.Config{}, err
			}
			return c.WithCluster(cl), nil
		}
	}
}
```

#### WithFinalizer

[`WithFinalizer`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#WithFinalizer) allows external state to be allocated and then cleaned up once the resource is deleted. When the resource is not terminating, the finalizer is set on the reconciled resource before the nested reconciler is called. When the resource is terminating, the finalizer is cleared only after the nested reconciler returns without an error.

The [Finalizers](#finalizers) utilities are used to manage the finalizer on the reconciled resource.

> Warning: It is crucial that each WithFinalizer have a unique and stable finalizer name. Two reconcilers that use the same finalizer, or a reconciler that changed the name of its finalizer, may leak the external state when the reconciled resource is deleted, or the resource may never terminate.

**Example:**

`WithFinalizer` can be used to wrap any other [SubReconciler](#subreconciler), which can then safely allocate external state while the resource is not terminating, and then cleanup that state once the resource is terminating.

```go
func SyncExternalState() *reconcilers.SubReconciler {
	return &reconcilers.WithFinalizer{
		Finalizer: "unique.finalizer.name"
		Reconciler: &reconcilers.SyncReconciler{
			Sync: func(ctx context.Context, resource *resources.TestResource) error {
				// allocate external state
				return nil
			},
			Finalize: func(ctx context.Context, resource *resources.TestResource) error {
				// cleanup the external state
				return nil
			},
		},
	}
}
```

### AdmissionWebhookAdapter

[`AdmissionWebhookAdapter`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#AdmissionWebhookAdapter) allows using [SubReconciler](#subreconciler) to process [admission webhook requests](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#webhook-request-and-response). The full suite of sub-reconcilers are available, however, behavior that is [generally not accepted](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#side-effects) within a webhook is discouraged. For example, new requests against the API server are discouraged (reading from an informer is ok), mutation requests against the API Server can cause a loop with the webhook processing its own requests.

All requests are allowed by default unless the [response.Allowed](https://pkg.go.dev/k8s.io/api/admission/v1#AdmissionResponse.Allowed) field is explicitly set, or the reconciler returns an error. The raw admission request and response can be retrieved from the context via the [`RetrieveAdmissionRequest`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveAdmissionRequest) and [`RetrieveAdmissionResponse`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveAdmissionResponse) methods, respectively. The [`Result`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile#Result) typically returned by a reconciler is unused.

The request object is unmarshaled from the request object for most operations, and the old object for delete operations. If the webhhook handles multiple resources or versions of the same resource with different shapes, use of an unstructured type is recommended.

If the resource being reconciled is mutated and the response does not already define a patch, a json patch is computed for the mutation and set on the response.

Testing can be done on the reconciler directly with [SubReconcilerTests](#subreconcilertests), or through the webhook with [AdmissionWebhookTests](#admissionwebhooktests).

**Example**

The Service Binding controller uses a mutating webhook to intercept the creation and updating of workload resources. It projects services into the workload based on ServiceBindings that reference that workload, mutating the resource. If the resource is mutated, a patch is automatically created and added to the webhook response. The webhook allows workloads to be bound at admission time.

```go
func AdmissionProjectorWebhook(c reconcilers.Config) *reconcilers.AdmissionWebhookAdapter {
	return &reconcilers.AdmissionWebhookAdapter{
		Name: "AdmissionProjectorWebhook",
		Type: &unstructured.Unstructured{},
		Reconciler: &reconcilers.SyncReconciler{
			Sync: func(ctx context.Context, workload *unstructured.Unstructured) error {
				c := reconcilers.RetrieveConfigOrDie(ctx)

				// find matching service bindings
				serviceBindings := &servicebindingv1beta1.ServiceBindingList{}
				if err := c.List(ctx, serviceBindings, client.InNamespace(workload.Namespace)); err != nil {
					return err
				}

				// check that bindings are for this specific workload
				activeServiceBindings := ...

				// project active bindings into workload, the workload is mutated by the projector
				projector := projector.New(resolver.New(c))
				for i := range activeServiceBindings {
					sb := activeServiceBindings[i].DeepCopy()
					sb.Default()
					if err := projector.Project(ctx, sb, workload); err != nil {
						return err
					}
				}

				return nil
			},
		},
		Config: c,
	}
}
```
[full source](https://github.com/scothis/servicebinding-runtime/blob/8ae0b1fb8b7a37856fa18171bc34e3462c35348b/controllers/webhook_controller.go#L113-L166)

The webhook adapter can be registered with the controller manager at a path, in this case `/interceptor`. There MutatingWebhookConfiguration resource that intercepts 

```go
mgr.GetWebhookServer().Register("/interceptor", controllers.AdmissionProjectorWebhook(config).Build())
```

## Testing

While `controller-runtime` focuses its testing efforts on integration testing by spinning up a new API Server and etcd, `reconciler-runtime` focuses on unit testing reconcilers. The state for each test case is pure, preventing side effects from one test case impacting the next.

The table test pattern is used to declare each test case in a test suite with the resource being reconciled, other given resources in the cluster, and all expected resource mutations (create, update, delete).

The tests make extensive use of given and mutated resources. It is recommended to use a library like [dies](https://dies.dev) to reduce boilerplate code and to highlight the delta unique to each test.

There are two test suites, one for reconcilers and an optimized harness for testing sub reconcilers.

<a name="reconcilertestsuite" />

### ReconcilerTests

[`ReconcilerTestCase`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#ReconcilerTestCase) run the full reconciler via the controller runtime Reconciler's Reconcile method. There are two ways to compose a ReconcilerTestCase either as an unordered set using [`ReconcilerTests`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#ReconcilerTests), or an order list using [`ReconcilerTestSuite`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#ReconcilerTestSuite). When using `ReconcilerTests` the key for each test case is used as the name for that test case.

**Example:**

```go
testRequest := ... // request for the resource to reconcile
inMemoryGatewayImagesConfigMap := ... // ConfigMap with images
inMemoryGateway := ... // resource to reconcile
gatewayCreate := ... // expected to be created
scheme := ... // scheme registered with all resource types the reconcile interacts with

rts := rtesting.ReconcilerTests{
	"creates gateway": {
		Request:      testRequest,
		GivenObjects: []client.Object{
			inMemoryGateway,
			inMemoryGatewayImagesConfigMap,
		},
		ExpectTracks: []client.Object{
			rtesting.NewTrackRequest(inMemoryGatewayImagesConfigMap, inMemoryGateway, scheme),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(inMemoryGateway, scheme, corev1.EventTypeNormal, "Created",
				`Created Gateway "%s"`, testName),
			rtesting.NewEvent(inMemoryGateway, scheme, corev1.EventTypeNormal, "StatusUpdated",
				`Updated status`),
		},
		ExpectCreates: []client.Object{
			gatewayCreate,
		},
		ExpectStatusUpdates: []client.Object{
			// example using an https://dies.dev style die to mutate the resource
			inMemoryGateway.
				StatusDie(func(d *diestreamingv1alpha1.InMemoryGatewayStatusDie) {
					d.ObservedGeneration(1)
					d.ConditionsDie(
						// the condition will be unknown since the child resource
						// was just created and hasn't been reconciled by its
						// controller yet
						inMemoryGatewayConditionGatewayReady.Unknown(),
						inMemoryGatewayConditionReady.Unknown(),
					)
				}),
		},
	},
	...
}}

rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
	return streaming.InMemoryGatewayReconciler(c, testSystemNamespace)
})
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/streaming/inmemorygateway_reconciler_test.go#L142-L169)

<a name="subreconcilertestsuite" />

### SubReconcilerTests

For more complex reconcilers, the number of moving parts can make it difficult to fully cover all aspects of the reonciler and handle corner cases and sources of error. The [`SubReconcilerTestCase`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#SubReconcilerTestCase) enables testing a single sub reconciler in isolation from the resource. While very similar to ReconcilerTestCase, these are the differences:

- `Request` is replaced with `Resource` since the resource is not lookedup, but handed to the reconciler. `ExpectResource` is the mutated value of the resource after the reconciler runs.
- `GivenStashedValues` is a map of stashed value to seed, `ExpectStashedValues` are individually compared with the actual stashed value after the reconciler runs.
- `ExpectStatusUpdates` is not available

There are two ways to compose a SubReconcilerTestCase either as an unordered set using [`SubReconcilerTests`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#SubReconcilerTests), or an order list using [`SubReconcilerTestSuite`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#SubReconcilerTestSuite). When using `SubReconcilerTests` the key for each test case is used as the name for that test case.

**Example:**

Like with the tracking example, the processor reconciler in projectriff also looks up images from a ConfigMap. The sub reconciler under test is responsible for tracking the ConfigMap, loading and stashing its contents. Sub reconciler tests make it trivial to test this behavior in isolation, including error conditions.

```go
processor := ...
processorImagesConfigMap := ...

rts := rtesting.SubReconcilerTests{
	"missing images configmap": {
		Resource: processor,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(processorImagesConfigMap, processor, scheme),
		},
		ShouldErr: true,
	},
	"stash processor image": {
		Resource: processor,
		GivenObjects: []client.Object{
			processorImagesConfigMap,
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(processorImagesConfigMap, processor, scheme),
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			streaming.ProcessorImagesStashKey: processorImagesConfigMap.Create().Data,
		},
	},
}

rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
	return streaming.ProcessorSyncProcessorImages(c, testSystemNamespace)
})
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/streaming/processor_reconciler_test.go#L279-L305)


### AdmissionWebhookTests

[`AdmissionWebhookTestCase`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#AdmissionWebhookTestCase) runs the full webhook handler via the controller runtime [webhook handler's](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/webhook/admission#Handler) Handle method. There are two ways to compose a AdmissionWebhookTestCase either as an unordered set using [`AdmissionWebhookTests`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#AdmissionWebhookTests), or an order list using [`AdmissionWebhookTestSuite`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#AdmissionWebhookTestSuite). When using `AdmissionWebhookTestSuite` the key for each test case is used as the name for that test case.

**Example**

Service bindings project into workloads with a controller and a mutating webhook. The admission request for the workload resource along with a given ServiceBinding resource is projected mutating the resource, which is treated as a patch in the admission response.

```go
workload := dieappsv1.DeploymentBlank.
	...
serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
	...

request := dieadmissionv1.AdmissionRequestBlank.
	KindDie(func(d *diemetav1.GroupVersionKindDie) {
		d.Group("apps")
		d.Version("v1")
		d.Kind("Deployment")
	}).
	ResourceDie(func(d *diemetav1.GroupVersionResourceDie) {
		d.Group("apps")
		d.Version("v1")
		d.Resource("deployments")
	}).
	UID(requestUID).
	Operation(admissionv1.Create).
	Namespace(namespace).
	Name(name)
response := dieadmissionv1.AdmissionResponseBlank.
	Allowed(true)

wts := rtesting.AdmissionWebhookTests{
	"project binding": {
		GivenObjects: []client.Object{
			serviceBinding,
		},
		Request: &admission.Request{
			AdmissionRequest: request.
				Object(workload.DieReleaseRawExtension()).
				DieRelease(),
		},
		ExpectedResponse: admission.Response{
			AdmissionResponse: response.DieRelease(),
			Patches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/template/spec/containers/0/env",
					Value: []interface{}{
						map[string]interface{}{
							"name":  "SERVICE_BINDING_ROOT",
							"value": "/bindings",
						},
					},
				},
				...
			},
		},
	},
}

wts.Run(t, scheme, func(t *testing.T, wtc *rtesting.AdmissionWebhookTestCase, c reconcilers.Config) *admission.Webhook {
	return controllers.AdmissionProjectorWebhook(c).Build()
})
```
[full source](https://github.com/scothis/servicebinding-runtime/blob/8ae0b1fb8b7a37856fa18171bc34e3462c35348b/controllers/webhook_controller_test.go#L177-L490)

### ExpectConfig

The [`ExpectConfig`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#ExpectConfig) is a testing object that can create a [Config](#config) with given test state that will observe the reconciler's behavior against the config and can assert that the observed behavior matches the expected behavior. When used with the `AdditionalConfigs` field of [ReconcilerTestCase](#reconcilertests) and [SubReconcilerTestCase](#subreconcilertests), the corresponding configs can be obtained with [`RetrieveAdditionalConfigs`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveAdditionalConfigs). Use of `RetrieveAdditionalConfigs` should be limited to a reconciler that is dedicated to work with multiple configs like [WithConfig](#withconfig); reconcilers nested under WithConfig should interact with the default config.

## Utilities

### Config

The [`Config`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config) is a single object that contains the common remote APIs needed by a reconciler. The config object includes:
- [`Client`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config.Client) as the primary interaction with the Kubernetes API Server. Gets and Lists are read from informers when available.
- [`APIReader`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config.APIReader) read-only Kubernetes API Server client that bypasses informers.
- [`Recorder`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config.Recorder) record Kubernetes events for a resource.
- [`Tracker`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config.Tracker) track relationships between resource, and later lookup resources tracking a specific resource.

Root reconcilers like [ResourceReconciler](#resourcereconciler) and [AdmissionWebhookAdapter](#admissionwebhookadapter) accept a Config to use that is then passed to [SubReconciler](#subreconciler) via the context, and retrieved using [`RetrieveConfigOrDie`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveConfigOrDie). The active config may be modified at runtime using [WithConfig](#withconfig).

To setup a Config for a test and make assertions that the expected behavior matches the observed behavior, use [ExpectConfig](#expectconfig).

### Stash

The stash allows passing arbitrary state between sub reconcilers within the scope of a single reconciler request. Values are stored on the context by [`StashValue`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#StashValue) and accessed via [`RetrieveValue`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveValue).

For testing, given stashed values can be defined in a [SubReconcilerTests](#subreconcilertests) with [`GivenStashedValues`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#SubReconcilerTestCase.GivenStashedValues). Newly stashed or mutated values expectations are defined with [`ExpectStashedValues`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#SubReconcilerTestCase.ExpectStashedValues).

**Example:**

```go
const exampleStashKey reconcilers.StashKey = "example"

func StashExampleSubReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "StashExample",
		Sync: func(ctx context.Context, resource *examplev1.MyExample) error {
			value := Example{} // something we want to expose to a sub reconciler later in this chain
			reconcilers.StashValue(ctx, exampleStashKey, value)
			return nil
		},
	}
}


func StashExampleSubReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "StashExample",
		Sync: func(ctx context.Context, resource *examplev1.MyExample) error {
			value, ok := reconcilers.RetrieveValue(ctx, exampleStashKey).(Example)
			if !ok {
				return nil, fmt.Errorf("expected stashed value for key %q", exampleStashKey)
			}
			... // do something with the value
		},
	}
}
```

### Tracker

The [`Tracker`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/tracker#Tracker) provides a means for one resource to watch another resource for mutations, triggering the reconciliation of the resource defining the reference.

It's common to work with a resource that is also tracked. The [Config.TrackAndGet](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config.TrackAndGet) method uses the same signature as client.Get, but additionally tracks the resource.

In the [Setup](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#SyncReconciler) method, a watch is created that will notify the handler every time a resource of that kind is mutated. The [EnqueueTracked](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#EnqueueTracked) helper returns a list of resources that are tracking the given resource, those resources are enqueued for the reconciler.

The tracker will automatically expire a track request if not periodically renewed. By default, the TTL is 2x the resync internal. This ensures all tracked resources will naturally have the tracking relationship refreshed as part of the normal reconciliation resource. There is no need to manually untrack a resource.

**Example:**

The stream gateways in projectriff fetch the image references they use to run from a ConfigMap. When the ConfigMap changes, we want to detect and rollout the updated images.

```go
func InMemoryGatewaySyncConfigReconciler(c reconcilers.Config, namespace string) reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "SyncConfig",
		Sync: func(ctx context.Context, resource *streamingv1alpha1.InMemoryGateway) error {
			log := logr.FromContextOrDiscard(ctx)
			c := reconciler.RetrieveConfig(ctx)

			var config corev1.ConfigMap
			key := types.NamespacedName{Namespace: namespace, Name: inmemoryGatewayImages}
			// track config for new images, get the configmap
			if err := c.TrackAndGet(ctx, key, &config); err != nil {
				return err
			}
			// consume the configmap
			resource.Status.GatewayImage = config.Data[gatewayImageKey]
			resource.Status.ProvisionerImage = config.Data[provisionerImageKey]
			return nil
		},

		Setup: func(ctx context.Context, mgr reconcilers.Manager, bldr *reconcilers.Builder) error {
			// enqueue the tracking resource for reconciliation from changes to
			// tracked ConfigMaps. Internally `EnqueueTracked` handels informer 
			// events to watch for changes of the target resource. When the
			// informer emits an event, the tracking resources are looked up
			// from the tracker and enqueded for reconciliation.
			bldr.Watches(&source.Kind{Type: &corev1.ConfigMap{}}, reconcilers.EnqueueTracked(ctx, &corev1.ConfigMap{}))
			return nil
		},
	}
}
```
[full source](https://github.com/projectriff/system/blob/4c3b75327bf99cc37b57ba14df4c65d21dc79d28/pkg/controllers/streaming/inmemorygateway_reconciler.go#L58-L84)

### Status

The `apis` package provides means for conveniently managing a custom resource's `.status`.

A resource's status subresource is expected to meet the following contract:

```go
type MyStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}
```

**Example:**

Embed `api.Status` into your resource's status and add more fields:

```go
type MyResourceStatus struct {
  apis.Status `json:",inline"`
  UsefulMessage string `json:"usefulMessage,omitempty"`
}

type MyResource struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`
  
  Spec MyResourceSpec `json:"spec"`
  // +optional
  Status MyResourceStatus `json:"status"`
}
```

### Finalizers

[Finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) allow a reconciler to clean up state for a resource that has been deleted by a client, and not yet fully removed. Terminating resources have `.metadata.deletionTimestamp` set. Resources with finalizers will stay in this terminating state until all finalizers are cleared from the resource. While using the [Kubernetes garbage collector](https://kubernetes.io/docs/concepts/architecture/garbage-collection/) is recommended when possible, finalizer are useful for cases when state exists outside of the same cluster, scope, and namespace of the reconciled resource that needs to be cleaned up when no longer used.

Deleting a resource that uses finalizers requires the controller to be running.

> Note: [WithFinalizer](#withfinalizer) can be used in lieu of, or in conjunction with, [ChildReconciler](#childreconciler)#Finalizer. The distinction is the scope within the reconciler tree where a finalizer is applied. While a reconciler can define as many finalizer on the resource as it desires, in practice, it's best to minimize the number of finalizers as setting and clearing each finalizer makes a request to the API Server. 
>
> A single WithFinalizer will always add a finalizer to the reconciled resource. It can then compose multiple ChildReconcilers, as well as other reconcilers that do not natively support managing finalizers (e.g. SyncReconciler). On the other hand, the ChildReconciler will only set the finalizer when it is required potentially reducing the number of finalizers, but only covers that exact sub-reconciler. It's important the external state that needs to be cleaned up be covered by a finalizer, it does not matter which finalizer is used.

The [AddFinalizer](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#AddFinalizer) and [ClearFinalizer](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#ClearFinalizer) functions patch the reconciled resource to update its finalizers. These methods work with [CastResource](#castresource) resources and use the same client the [ResourceReconciler](#resourcereconciler) used to originally load the reconciled resource. They can be called inside [SubReconcilers](#subreconciler) that may use a different client.

When an update is required, only the `.metadata.finalizers` field is patched. The reconciled resource's `.metadata.resourceVersion` is used as an optimistic concurrency lock, and is updated with the value returned from the server. Any error from the server will cause the resource reconciliation to err. When testing with [SubReconcilerTests](#subreconcilertests), the resource version of the resource defaults to `"999"`, the patch bytes include the resource version and the response increments the reonciled resource's version. For a resource with the default version that patches a finalizer, the expected reconciled resource will have a resource version of `"1000"`.

A minimal test case for a sub reconciler that adds a finalizer may look like:

```go
	...
	{
		Name: "add 'test.finalizer' finalizer",
		Resource: resourceDie,
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(resourceDie, scheme, corev1.EventTypeNormal, "FinalizerPatched",
				`Patched finalizer %q`, "test.finalizer"),
		},
		ExpectResource: resourceDie.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Finalizers("test.finalizer")
				d.ResourceVersion("1000")
			}),
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "testing.reconciler.runtime",
				Kind:      "TestResource",
				Namespace: resourceDie.GetNamespace(),
				Name:      resourceDie.GetName(),
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":["test.finalizer"],"resourceVersion":"999"}}`),
			},
		},
	},
	...
```

### ResourceManager

The [`ResourceManager`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#ResourceManager) provides a means to mange a single resource by sychronizing the current and desired state. The resource will be created if it does not exist, deleted if no longer desired and updated when not semantically equivlent. The same resource manager should be reused to manage multiple resource and must be reused when managing the same resource over time in order to take full effect. This utility is used by the [ChildReconciler](#childreconciler) and [AggregateReconciler](#aggregatereconciler).

The `Manage(ctx context.Context, resource, actual, desired client.Object) (client.Object, error)` method take three objects and returns another object:
- `resource` is the reconciled resource, events, tracks and finalizer are against this object. May be an object of any underlaying type.
- `actual` the resource that exists on the API Server. Must be compatible with the `Type`.
- `desired` the resoruce that should exist on the API Server after this call. Must be compatible with the `Type`.
- the returned object is the value as persisted by the API Server.

Internally, a mutations made to the resoruce at admission time (like defaults applied by a mutating webhook) are captured and reapplied to the desired state before checking if an update is needed. This reduces requests that are functionally a no-op but create churn on the API Server. The mutation cache is defensive and fails open to make an API request.

If configured, a [finalizer](#finalizers) can be managed on the resource which will be added before create/udpate and removed after sucessful delete.

If requested, the managed resource will be tracked for the resource.

## Breaking Changes

Known breaking changes are captured in the [release notes](https://github.com/vmware-labs/reconciler-runtime/releases), it is strongly recomened to review the release notes before upgrading to a new version of reconciler-runtime. When possible, breaking changes are first marked as deprecations before full removal in a later release. Patch releases will be issued to fix significant bugs and unintentional breaking changes.

We strive to release reconciler-runtime against the latest Kuberentes and controller-runtime releases. Upstream breaking changes in either dependency may also force changes in reconciler-runtime without a deprecation period.

reconciler-runtime is rapidly evolving. While we strive for API compatability between releases, functionality that is better handled using a different API may be removed. Release version numbers follow semver.

## Contributing

The reconciler-runtime project team welcomes contributions from the community. If you wish to contribute code and you have not signed our contributor license agreement (CLA), our bot will update the issue when you open a Pull Request. For any questions about the CLA process, please refer to our [FAQ](https://cla.vmware.com/faq). For more detailed information, refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## Acknowledgements

`reconciler-runtime` was conceived in [`projectriff/system`](https://github.com/projectriff/system/) and implemented initially by [Scott Andrews](https://github.com/scothis), [Glyn Normington](https://github.com/glyn) and the [riff community](https://github.com/orgs/projectriff/people) at large, drawing inspiration from [Kubebuilder](https://www.kubebuilder.io) and [Knative](https://knative.dev) reconcilers.

## License

Apache License v2.0: see [LICENSE](./LICENSE) for details.

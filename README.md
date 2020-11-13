# reconciler-runtime <!-- omit in toc -->

![CI](https://github.com/vmware-labs/reconciler-runtime/workflows/CI/badge.svg?branch=main)
[![GoDoc](https://godoc.org/github.com/vmware-labs/reconciler-runtime?status.svg)](https://godoc.org/github.com/vmware-labs/reconciler-runtime)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmware-labs/reconciler-runtime)](https://goreportcard.com/report/github.com/vmware-labs/reconciler-runtime)
[![codecov](https://codecov.io/gh/vmware-labs/reconciler-runtime/branch/main/graph/badge.svg)](https://codecov.io/gh/vmware-labs/reconciler-runtime)

`reconciler-runtime` is an opinionated framework for authoring and testing Kubernetes reconcilers using [`controller-runtime`](https://github.com/kubernetes-sigs/controller-runtime) project. `controller-runtime` provides infrastructure for creating and operating controllers, but provides little support for the business logic of implementing a reconciler within the controller. The [`Reconciler` interface](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler) provided by `controller-runtime` is the handoff point with `reconciler-runtime`.

<!-- ToC managed by https://marketplace.visualstudio.com/items?itemName=yzhang.markdown-all-in-one -->
- [Reconcilers](#reconcilers)
	- [ParentReconciler](#parentreconciler)
	- [SubReconciler](#subreconciler)
		- [SyncReconciler](#syncreconciler)
		- [ChildReconciler](#childreconciler)
- [Testing](#testing)
	- [ReconcilerTestSuite](#reconcilertestsuite)
	- [SubReconcilerTestSuite](#subreconcilertestsuite)
- [Utilities](#utilities)
	- [Config](#config)
	- [Stash](#stash)
	- [Tracker](#tracker)
- [Contributing](#contributing)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Reconcilers

### ParentReconciler

A [`ParentReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#ParentReconciler) is responsible for orchestrating the reconciliation of a single resource. The reconciler delegates the manipulation of other resources to SubReconcilers.

The parent is responsible for:
- fetching the resource being reconciled
- creating a stash to pass state between sub reconcilers
- passing the resource to each sub reconciler in turn
- reflects the observed generation on the status
- updates the resource status if it was modified
- logging the reconcilers activities
- records events for mutations and errors

The implementor is responsible for:
- defining the set of sub reconcilers

**Example:**

Parent reconcilers tend to be quite simple, as they delegate their work to sub reconcilers. We'll use an example from projectriff of the Function resource, which uses Kpack to build images from a git repo. In this case the FunctionTargetImageReconciler resolves the target image for the function, and FunctionChildImageReconciler creates a child Kpack Image resource based on the resolve value. 

```go
func FunctionReconciler(c reconcilers.Config) *reconcilers.ParentReconciler {
	c.Log = c.Log.WithName("Function")

	return &reconcilers.ParentReconciler{
		Type: &buildv1alpha1.Function{},
		SubReconcilers: []reconcilers.SubReconciler{
			FunctionTargetImageReconciler(c),
			FunctionChildImageReconciler(c),
		},

		Config: c,
	}
}
```
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/build/function_reconciler.go#L39-L51)

### SubReconciler

The [`SubReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#SubReconciler) interface defines the contract between the parent and sub reconcilers.

There are two types of sub reconcilers provided by `reconciler-runtime`:

#### SyncReconciler

The [`SyncReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#SyncReconciler) is the minimal type-aware sub reconciler. It is used to manage a portion of the parent's reconciliation that is custom, or whose behavior is not covered by another sub reconciler type. Common uses include looking up reference data for the reconciliation, or controlling resources that are not kubernetes resources.

**Example:**

While sync reconcilers have the ability to do anything a reconciler can do, it's best to keep them focused on a single goal, letting the parent reconciler structure multiple sub reconcilers together. In this case, we use the parent resource and the client to resolve the target image and stash the value on the parent's status. The status is a good place to stash simple values that can be made public. More [advanced forms of stashing](#stash) are also available.

```go
func FunctionTargetImageReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("TargetImage")

	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, parent *buildv1alpha1.Function) error {
			targetImage, err := resolveTargetImage(ctx, c.Client, parent)
			if err != nil {
				return err
			}
			parent.Status.MarkImageResolved()
			parent.Status.TargetImage = targetImage
			return nil
		},

		Config: c,
	}
}
```
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/build/function_reconciler.go#L53-L74)

#### ChildReconciler

The [`ChildReconciler`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#ChildReconciler) is a sub reconciler that is responsible for managing a single controlled resource. A developer defines their desired state for the child resource (if any), and the reconciler creates/updates/deletes the resource to match the desired state. The child resource is also used to update the parent's status. Mutations and errors are recorded for the parent.

The ChildReconciler is responsible for:
- looking up an existing child
- creating/updating/deleting the child resource based on the desired state
- setting the owner reference on the child resource
- logging the reconcilers activities
- recording child mutations and errors for the parent resource
- adapting to child resource changes applied by mutating webhooks

The implementor is responsible for:
- defining the desired resource
- indicating if two resources are semantically equal
- merging the actual resource with the desired state (often as simple as copying the spec and labels)
- updating the parent's status from the child

**Example:**

Now it's time to create the child Image resource that will do the work of building our Function. This reconciler looks more more complex than what we have seen so far, each function on the reconciler provides a focused hook into the lifecycle being orchestrated by the ChildReconciler.

```go
func FunctionChildImageReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("ChildImage")

	return &reconcilers.ChildReconciler{
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
		
		Config:     c,
		IndexField: ".metadata.functionController",
	}
}
```
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/build/function_reconciler.go#L76-L151)

## Testing

While `controller-runtime` focuses its testing efforts on integration testing by spinning up a new API Server and etcd, `reconciler-runtime` focuses on unit testing reconcilers. The state for each test case is pure, preventing side effects from one test case impacting the next.

The table test pattern is used to declare each test case in a test suite with the resource being reconciled, other given resources in the cluster, and all expected resource mutations (create, update, delete).

The tests make extensive use of factories to reduce boilerplate code and to highlight the delta unique to each test. Factories are themselves an immutable fluent API that returns a new factory with mutated the underlying state. This makes it safe to take an existing factory and extend it for use in a new test case without impacting the original use. Changes to the original object before the extension will cascade to you.

```go
deploymentCreate := factories.Deployment().
	ObjectMeta(func(om factories.ObjectMeta) {
		om.Namespace(testNamespace)
		om.GenerateName("%s-gateway-", testName)
		om.AddLabel(streamingv1alpha1.GatewayLabelKey, testName)
		om.ControlledBy(gateway, scheme)
	}).
	AddSelectorLabel(streamingv1alpha1.GatewayLabelKey, testName).
	PodTemplateSpec(func(pts factories.PodTemplateSpec) {
		pts.ContainerNamed("test", func(c *corev1.Container) {
			c.Image = "scratch"
		})
	})
deploymentGiven := deploymentCreate.
	ObjectMeta(func(om factories.ObjectMeta) {
		om.Name("%s-gateway-000", testName)
		om.Created(1)
	})
```
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/streaming/gateway_reconciler_test.go#L84-L101)

Factories are provided for some common k8s types like Deployment, ConfigMap, ServiceAccount (contributions for more are welcome). Resources that don't have a factory can be wrapped.

```go
factory := rtesting.Wrapper(fullyDefinedResource)
```

There are two test suites, one for reconcilers and an optimized harness for testing sub reconcilers.

### ReconcilerTestSuite

[`ReconcilerTestCase`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#ReconcilerTestCase) run the full reconciler via the controller runtime Reconciler's Reconcile method.

```go
testKey := ... // NamesapcedName of the resource to reconcile
inMemoryGatewayImagesConfigMap := ... // factory holding ConfigMap with images
inMemoryGateway := ... // factory holding resource to reconcile
gatewayCreate := ... // factory holding gateway expected to be created
scheme := ... // scheme registered with all resource types the reconcile interacts with

rts := rtesting.ReconcilerTestSuite{{
	...
}, {
	Name: "creates gateway",
	Key:  testKey,
	GivenObjects: []rtesting.Factory{
		inMemoryGatewayMinimal,
		inMemoryGatewayImagesConfigMap,
	},
	ExpectTracks: []rtesting.TrackRequest{
		rtesting.NewTrackRequest(inMemoryGatewayImagesConfigMap, inMemoryGateway, scheme),
	},
	ExpectEvents: []rtesting.Event{
		rtesting.NewEvent(inMemoryGateway, scheme, corev1.EventTypeNormal, "Created",
			`Created Gateway "%s"`, testName),
		rtesting.NewEvent(inMemoryGateway, scheme, corev1.EventTypeNormal, "StatusUpdated",
			`Updated status`),
	},
	ExpectCreates: []rtesting.Factory{
		gatewayCreate,
	},
	ExpectStatusUpdates: []rtesting.Factory{
		inMemoryGateway.
			StatusObservedGeneration(1).
			StatusConditions(
				// the condition will be unknown since the child resource
				// was just created and hasn't been reconciled by its
				// controller yet
				inMemoryGatewayConditionGatewayReady.Unknown(),
				inMemoryGatewayConditionReady.Unknown(),
			),
	},
}, {
	...
}}

rts.Test(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
	return streaming.InMemoryGatewayReconciler(c, testSystemNamespace)
})
```
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/streaming/inmemorygateway_reconciler_test.go#L142-L169)

### SubReconcilerTestSuite

For more complex reconcilers, the number of moving parts can make it difficult to fully cover all aspects of the reonciler and handle corner cases and sources of error. The [`SubReconcilerTestCase`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/testing#SubReconcilerTestCase) enables testing a single sub reconciler in isolation from the parent. While very similar to ReconcilerTestCase, these are the differences:

- `Key` is replaced with `Parent` since the parent resource is not lookedup, but handed to the reconciler. `ExpectParent` is the mutated value of the parent resource after the reconciler runs.
- `GivenStashedValues` is a map of stashed value to seed, `ExpectStashedValues` are individually compared with the actual stashed value after the reconciler runs.
- `ExpectStatusUpdates` is not available

**Example:**

Like with the tracking example, the processor reconciler in projectriff also looks up images from a ConfigMap. The sub reconciler under test is responsible for tracking the ConfigMap, loading and stashing its contents. Sub reconciler tests make it trivial to test this behavior in isolation, including error conditions.

```go
processor := ...
processorImagesConfigMap := ...

rts := rtesting.SubReconcilerTestSuite{
	{
		Name:   "missing images configmap",
		Parent: processor,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(processorImagesConfigMap, processor, scheme),
		},
		ShouldErr: true,
	},
	{
		Name:   "stash processor image",
		Parent: processor,
		GivenObjects: []rtesting.Factory{
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
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/streaming/processor_reconciler_test.go#L279-L305)

## Utilities

### Config

The [`Config`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#Config) is a single object that contains the key APIs needed by a reconciler. The config object is provided to the reconciler when initialized and is preconfigured for the reconciler.

### Stash

The stash allows passing arbitrary state between sub reconcilers within the scope of a single reconciler request. Values are stored on the context by [`StashValue`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#StashValue) and accessed via [`RetrieveValue`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/reconcilers#RetrieveValue).

**Example:**

```go
const exampleStashKey reconcilers.StashKey = "example"

func StashExampleSubReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("StashExample")

	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, resource *examplev1.MyExample) error {
			value := Example{} // something we want to expose to a sub reconciler later in this chain
			reconcilers.StashValue(ctx, exampleStashKey, *value)
			return nil
		},

		Config: c,
	}
}


func StashExampleSubReconciler(c reconcilers.Config) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("StashExample")

	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, resource *examplev1.MyExample) error {
			value, ok := reconcilers.RetrieveValue(ctx, exampleStashKey).(Example)
			if !ok {
				return nil, fmt.Errorf("expected stashed value for key %q", exampleStashKey)
			}
			... // do something with the value
		},

		Config: c,
	}
}
```

### Tracker

The [`Tracker`](https://pkg.go.dev/github.com/vmware-labs/reconciler-runtime/tracker#Tracker) provides a means for one resource to watch another resource for mutations, triggering the reconciliation of the resource defining the reference.

**Example:**

The stream gateways in projectriff fetch the image references they use to run from a ConfigMap, when the values change, we want to detect and rollout the updated images.

```go
func InMemoryGatewaySyncConfigReconciler(c reconcilers.Config, namespace string) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("SyncConfig")

	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, parent *streamingv1alpha1.InMemoryGateway) error {
			var config corev1.ConfigMap
			key := types.NamespacedName{Namespace: namespace, Name: inmemoryGatewayImages}
			// track config for new images
			c.Tracker.Track(
				// the resource to track, GVK and NamespacedName
				tracker.NewKey(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, key),
				// the resource to enqueue, NamespacedName only
				types.NamespacedName{Namespace: parent.Namespace, Name: parent.Name},
			)
			// get the configmap
			if err := c.Get(ctx, key, &config); err != nil {
				return err
			}
			// consume the configmap
			parent.Status.GatewayImage = config.Data[gatewayImageKey]
			parent.Status.ProvisionerImage = config.Data[provisionerImageKey]
			return nil
		},

		Config: c,
		Setup: func(mgr reconcilers.Manager, bldr *reconcilers.Builder) error {
			// enqueue the tracking resource for reconciliation from changes to
			// tracked ConfigMaps. Internally `EnqueueTracked` sets up an 
			// Informer to watch to changes of the target resource. When the
			// informer emits an event, the tracking resources are looked up
			// from the tracker and enqueded for reconciliation.
			bldr.Watches(&source.Kind{Type: &corev1.ConfigMap{}}, reconcilers.EnqueueTracked(&corev1.ConfigMap{}, c.Tracker, c.Scheme))
			return nil
		},
	}
}
```
[full source](https://github.com/projectriff/system/blob/1fcdb7a090565d6750f9284a176eb00a3fe14663/pkg/controllers/streaming/inmemorygateway_reconciler.go#L58-L84)

## Contributing

The reconciler-runtime project team welcomes contributions from the community. If you wish to contribute code and you have not signed our contributor license agreement (CLA), our bot will update the issue when you open a Pull Request. For any questions about the CLA process, please refer to our [FAQ](https://cla.vmware.com/faq). For more detailed information, refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## Acknowledgements

`reconciler-runtime` was conceived in [`projectriff/system`](https://github.com/projectriff/system/) and implemented initially by [Scott Andrews](https://github.com/scothis), [Glyn Normington](https://github.com/glyn) and the [riff community](https://github.com/orgs/projectriff/people) at large, drawing inspiration from [Kubebuilder](https://www.kubebuilder.io) and [Knative](https://knative.dev) reconcilers.

## License

Apache License v2.0: see [LICENSE](./LICENSE) for details.

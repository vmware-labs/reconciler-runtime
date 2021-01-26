/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

// Package informers enables informer watches to be stopped when they are no
// longer needed, which is not currently supported by controller runtime.
//
// Why do watches need to be stopped? When reconcilers are dealing with
// duck-typed resources, the set of watches is not fixed at compile time -
// watches need to be started as new duck typed resources come into play.
// Similarly, over time, those resources may go out of play and, unless watches
// are stopped, there would be a resource leak.
//
// The current implementation of this package is layered on top of controller
// runtime and could be simplified considerably if moved upstream into
// controller runtime. See below for some of the current limitations.
//
// The basic approach to stopping watches is to use multiple controller managers
// to manage watches because a controller manager (which has its own informer
// cache) can be stopped and this results in the watches associated with the
// controller manager being stopped.
//
// The use of multiple controller managers is artificial and probably has
// downsides since the metrics and health servers and leader election are all
// disabled. But enabling the servers would consume two ports per reconciler,
// which isn't scalable.
//
// The informers struct contains a map of informer structs indexed by group-kind
// kind being watched.
//
// Within each informer struct, a controller (with the informer's
// group-version-kind as the object being controlled) is created per reconciler
// and watches are added to this controller. When the controller is driven for
// reconciliation, this is delegated to the corresponding reconciler.
//
// This is a proof of concept only, with at least the following limitations:
//
// * Since a watch is added when a track request is issued during reconciliation,
//   watches will be added repeatedly which may result in many more reconciliation
//   requests being presented to reconcilers than is necessary. It could also be a
//   resource leak depending on how watches are handled by controller runtime.
//
// * The watches should watch metadata only, for efficiency's sake. Note that
//   builder.OnlyMetadata (used elsewhere for metadata-only watches) relies on a
//   manager's scheme to get the group-version-kind from an incoming object, which
//   isn't appropriate since duck-typed objects won't necessarily be registered
//   with any scheme. See a TODO in informers.go.
//
// * Subsidiary controller managers are configured without metric and health
//   endpoints and with leader election disabled. See the TODO in supermanager.go.
//
// * Superfluous controllers may be created when certain races occur and there is
//   no way to tidy them up. See a TODO in informers.go.
package informers

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
// controller runtime.
//
// The basic approach to stopping watches is to use multiple controller managers
// to manage watches because a controller manager (which has its own informer
// cache) can be stopped and this results in the watches associated with the
// controller manager being stopped.
package informers

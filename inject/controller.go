/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package inject

import "sigs.k8s.io/controller-runtime/pkg/controller"

type Controller interface {
	InjectController(controller controller.Controller) error
}

// ControllerInto will set controller on i and return the result if it implements Controller.
// Returns false if i does not implement Controller.
func ControllerInto(c controller.Controller, i interface{}) (bool, error) {
	if s, ok := i.(Controller); ok {
		return true, s.InjectController(c)
	}
	return false, nil
}

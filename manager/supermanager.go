/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package manager

import (
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// New returns a controller manager which can subquently create other managers
// with the given REST configuration and controller options. If the REST
// configuration or anything it refers to is mutated or if anything the
// controller options refers to is subsequently mutated, unpredictable behaviour
// may occur.
func New(config *rest.Config, options ctrl.Options) (SuperManager, error) {
	mainManager, err := ctrl.NewManager(config, options)
	if err != nil {
		return nil, err
	}

	return &superManager{
		Manager: mainManager,
		config:  config,
		options: options,
	}, nil
}

// SuperManager is a controller manager which can create other managers in order
// to be able to stop watches by stopping the corresponding caches.
type SuperManager interface {
	ctrl.Manager

	// NewManager returns a new controller manager which is configured the same
	// as this super manager.
	NewManager() (ctrl.Manager, error)
}

type superManager struct {
	ctrl.Manager
	config  *rest.Config // or should we use GetConfig() to get the initialised config from the main manager?
	options ctrl.Options
}

func (sm *superManager) NewManager() (ctrl.Manager, error) {
	return ctrl.NewManager(sm.config, sm.options)
}

/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package manager

import (
	"context"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// New returns a controller manager which can subquently create other managers
// with the given REST configuration and controller options. If the REST
// configuration or anything it refers to is mutated or if anything the
// controller options refers to is subsequently mutated, unpredictable behaviour
// may occur.
func New(config *rest.Config, options ctrl.Options) (SuperManager, error) {
	return new(func() (ctrl.Manager, error) {
		return ctrl.NewManager(config, options)
	})
}

type newManagerFunc func() (ctrl.Manager, error)

func new(newManager newManagerFunc) (*superManager, error) {
	mainManager, err := newManager()
	if err != nil {
		return nil, err
	}

	return &superManager{
		Manager:        mainManager,
		newManagerFunc: newManager,
		failures:       make(chan error),
		stop:           make(chan struct{}),
	}, nil
}

// SuperManager is a controller manager which can create other managers in order
// to be able to stop watches by stopping the corresponding caches.
type SuperManager interface {
	ctrl.Manager

	// NewManager returns a new controller manager which is configured the same
	// as this super manager. The new manager should be started using the
	// AddCancelable method on this interface so that the
	NewManager() (ctrl.Manager, error)

	// AddCancelable starts the given runnable asynchronously and returns a
	// function which can be used to cancel the start. If the started runnable
	// returns an error, this causes the super manager Start method to return
	// the error, unless the super manager Start returns for another reason.
	AddCancelable(r manager.Runnable) (context.CancelFunc, error)
}

type superManager struct {
	ctrl.Manager
	newManagerFunc newManagerFunc
	failures       chan error
	stop           chan struct{}
}

var _ manager.Runnable = &superManager{}

// NewManager implements SuperManager.
func (sm *superManager) NewManager() (ctrl.Manager, error) {
	return sm.newManagerFunc()
}

// Start wraps the main manager to report other errors early.
func (sm *superManager) Start(ctx context.Context) error {
	defer close(sm.stop)

	go func() {
		sm.failures <- sm.Manager.Start(ctx)
	}()

	return <-sm.failures
}

// AddWithContext implements SuperManager.
func (sm *superManager) AddCancelable(r manager.Runnable) (context.CancelFunc, error) {
	ctx, cancel := sm.contextWithCancel()
	go func() {
		if err := r.Start(ctx); err != nil {
			sm.failures <- err
		}
	}()
	return cancel, nil
}

func (sm *superManager) contextWithCancel() (context.Context, context.CancelFunc) {
	ctx, c := context.WithCancel(context.Background())

	go func() {
		done := ctx.Done()
		select {
		case <-done:
		case <-sm.stop:
			c()
		}
	}()

	return ctx, c
}

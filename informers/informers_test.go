/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package informers_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/informers"
	supermanager "github.com/vmware-labs/reconciler-runtime/manager"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestInformers_GetSameInformer(t *testing.T) {
	errChan := make(chan error)
	mockSuperManager := &mockSuperManager{
		start: func(ctx context.Context) error {
			return <-errChan
		},
		newManager: func() (ctrl.Manager, error) {
			return &mockManager{}, nil
		},
		addCancelable: func(r manager.Runnable) (context.CancelFunc, error) {
			return nil, nil
		},
	}

	is := informers.New(time.Hour, mockSuperManager, logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())

	outChan := make(chan error)
	go func() {
		outChan <- is.Start(ctx)
	}()

	gvk := schema.GroupVersionKind{
		Group:   "group",
		Version: "version",
		Kind:    "kind",
	}

	i1, _, err := is.GetInformer(gvk)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	i2, _, err := is.GetInformer(gvk)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if i1 != i2 {
		t.Errorf("unexpected informer %v, should have been %v", i2, i1)
	}

	cancel()
	close(errChan)
	err = <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestInformers_GetDistinctInformer(t *testing.T) {
	errChan := make(chan error)
	mockSuperManager := &mockSuperManager{
		start: func(ctx context.Context) error {
			return <-errChan
		},
		newManager: func() (ctrl.Manager, error) {
			return &mockManager{}, nil
		},
		addCancelable: func(r manager.Runnable) (context.CancelFunc, error) {
			return func() {}, nil
		},
	}

	is := informers.New(time.Hour, mockSuperManager, logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())

	outChan := make(chan error)
	go func() {
		outChan <- is.Start(ctx)
	}()

	gvk := schema.GroupVersionKind{
		Group:   "group",
		Version: "version",
		Kind:    "kind",
	}

	i1, c, err := is.GetInformer(gvk)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	c()

	i2, _, err := is.GetInformer(gvk)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if i1 == i2 {
		t.Errorf("unexpected informer %v", i2)
	}

	cancel()
	close(errChan)
	err = <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestInformers_GetInformerRace(t *testing.T) {
	first := true
	errChan := make(chan error)
	mockSuperManager := &mockSuperManager{
		start: func(ctx context.Context) error {
			return <-errChan
		},
		// set newManager later
		addCancelable: func(r manager.Runnable) (context.CancelFunc, error) {
			return func() {}, nil
		},
	}

	is := informers.New(time.Hour, mockSuperManager, logr.Discard())

	gvk := schema.GroupVersionKind{
		Group:   "group",
		Version: "version",
		Kind:    "kind",
	}

	var i1 informers.Informer = nil
	var i1Err error = errors.New("unexpected")
	mockSuperManager.newManager = func() (ctrl.Manager, error) {
		if first {
			first = false
			i1, _, i1Err = is.GetInformer(gvk)
		}
		return &mockManager{}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	outChan := make(chan error)
	go func() {
		outChan <- is.Start(ctx)
	}()

	i2, _, i2Err := is.GetInformer(gvk)
	if i1Err != nil {
		t.Errorf("unexpected error %v", i1Err)
	}
	if i2Err != nil {
		t.Errorf("unexpected error %v", i2Err)
	} else if i1 != i2 {
		t.Errorf("unexpected informer %v, should have been %v", i2, i1)
	}

	cancel()
	close(errChan)
	err := <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestInformers_AddEventHandler(t *testing.T) {
	errChan := make(chan error)
	mockSuperManager := &mockSuperManager{
		start: func(ctx context.Context) error {
			return <-errChan
		},
		newManager: func() (ctrl.Manager, error) {
			return &mockManager{
				getConfig: func() *rest.Config {
					return &rest.Config{}
				},
			}, nil
		},
		addCancelable: func(r manager.Runnable) (context.CancelFunc, error) {
			return nil, nil
		},
	}

	is := informers.New(time.Hour, mockSuperManager, logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())

	outChan := make(chan error)
	go func() {
		outChan <- is.Start(ctx)
	}()

	gvk := schema.GroupVersionKind{
		Group:   "group",
		Version: "version",
		Kind:    "kind",
	}

	i1, _, err := is.GetInformer(gvk)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	err = i1.AddEventHandler(nil, &mockReconciler{})
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	// repeat
	err = i1.AddEventHandler(nil, &mockReconciler{})
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	cancel()
	close(errChan)
	err = <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestInformers_AddEventHandlerRace(t *testing.T) {
	first := true
	simulateRace := func() {
		fmt.Println("debug")
	}
	reconciler := &mockReconciler{}
	errChan := make(chan error)
	mockSuperManager := &mockSuperManager{
		start: func(ctx context.Context) error {
			return <-errChan
		},
		newManager: func() (ctrl.Manager, error) {
			return &mockManager{
				getConfig: func() *rest.Config {
					if first {
						first = false
						simulateRace()
					}
					return &rest.Config{}
				},
			}, nil
		},
		addCancelable: func(r manager.Runnable) (context.CancelFunc, error) {
			return nil, nil
		},
	}

	is := informers.New(time.Hour, mockSuperManager, logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())

	outChan := make(chan error)
	go func() {
		outChan <- is.Start(ctx)
	}()

	gvk := schema.GroupVersionKind{
		Group:   "group",
		Version: "version",
		Kind:    "kind",
	}

	i1, _, err := is.GetInformer(gvk)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	simulateRace = func() {
		err = i1.AddEventHandler(nil, reconciler)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
	}

	err = i1.AddEventHandler(nil, reconciler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	cancel()
	close(errChan)
	err = <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

var _ supermanager.SuperManager = &mockSuperManager{}

type mockSuperManager struct {
	ctrl.Manager
	start         func(context.Context) error
	newManager    func() (ctrl.Manager, error)
	addCancelable func(manager.Runnable) (context.CancelFunc, error)
}

func (sm *mockSuperManager) Start(ctx context.Context) error {
	return sm.start(ctx)
}

func (sm *mockSuperManager) NewManager() (ctrl.Manager, error) {
	return sm.newManager()
}

func (sm *mockSuperManager) AddCancelable(r manager.Runnable) (context.CancelFunc, error) {
	return sm.addCancelable(r)
}

var _ manager.Manager = &mockManager{}

type mockManager struct {
	ctrl.Manager
	start     func(ctx context.Context) error
	getConfig func() *rest.Config
}

func (m *mockManager) Start(ctx context.Context) error {
	return m.start(ctx)
}

func (m *mockManager) GetConfig() *rest.Config {
	return m.getConfig()
}

func (m *mockManager) GetScheme() *runtime.Scheme {
	return &runtime.Scheme{}
}

func (m *mockManager) GetLogger() logr.Logger {
	return logr.Discard()
}

func (m *mockManager) SetFields(interface{}) error {
	return nil
}

func (m *mockManager) Add(manager.Runnable) error {
	return nil
}

type mockReconciler struct{}

func (*mockReconciler) Reconcile(context.Context, reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

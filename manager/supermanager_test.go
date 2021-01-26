/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package manager

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"golang.org/x/net/context"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSuperManager_MainManagerNewError(t *testing.T) {
	_, err := new(func() (ctrl.Manager, error) {
		return nil, errors.New("failed to create manager")
	}, logr.Discard())
	if err == nil || err.Error() != "failed to create manager" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestSuperManager_MainManagerStartError(t *testing.T) {
	sm, err := new(func() (ctrl.Manager, error) {
		return &mockManager{
			start: func(ctx context.Context) error {
				return errors.New("start failed")
			},
		}, nil
	}, logr.Discard())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	err = sm.Start(nil)
	if err == nil || err.Error() != "start failed" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestSuperManager_SubmanagerNewError(t *testing.T) {
	first := true
	sm, err := new(func() (ctrl.Manager, error) {
		if first {
			first = false
			return nil, nil
		}
		return nil, errors.New("failed to create manager")
	}, logr.Discard())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	_, err = sm.NewManager()
	if err == nil || err.Error() != "failed to create manager" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestSuperManager_SubmanagerStopsOk(t *testing.T) {
	errChan := make(chan error)
	sm, err := new(func() (ctrl.Manager, error) {
		return &mockManager{
			start: func(ctx context.Context) error {
				return <-errChan
			},
		}, nil
	}, logr.Discard())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	testManager := &mockManager{
		start: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}

	cancel, err := sm.AddCancelable(testManager)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	ctx, _ := context.WithCancel(context.Background())
	outChan := make(chan error)
	go func() {
		outChan <- sm.Start(ctx)
	}()

	cancel()

	errChan <- errors.New("start error")

	err = <-outChan
	if err == nil || err.Error() != "start error" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestSuperManager_SubmanagerStopsWithError(t *testing.T) {
	sm, err := new(func() (ctrl.Manager, error) {
		return &mockManager{
			start: func(ctx context.Context) error {
				select {} // block indefinitely
			},
		}, nil
	}, logr.Discard())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	testManager := &mockManager{
		start: func(ctx context.Context) error {
			<-ctx.Done()
			return errors.New("start error")
		},
	}

	cancel, err := sm.AddCancelable(testManager)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	ctx, _ := context.WithCancel(context.Background())
	outChan := make(chan error)
	go func() {
		outChan <- sm.Start(ctx)
	}()

	cancel()

	err = <-outChan
	if err == nil || err.Error() != "start error" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestSuperManager_CancelingSuperManagerCancelsSubmanager(t *testing.T) {
	sm, err := new(func() (ctrl.Manager, error) {
		return &mockManager{
			start: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
		}, nil
	}, logr.Discard())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	subManagerComplete := make(chan struct{})
	testManager := &mockManager{
		start: func(ctx context.Context) error {
			<-ctx.Done()
			subManagerComplete <- struct{}{}
			return nil
		},
	}

	_, err = sm.AddCancelable(testManager)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	ctx, superCancel := context.WithCancel(context.Background())
	outChan := make(chan error)
	go func() {
		outChan <- sm.Start(ctx)
	}()

	superCancel()

	<-subManagerComplete
	err = <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestSuperManager_AddSubmanagerAfterCancelingSuperManager(t *testing.T) {
	sm, err := new(func() (ctrl.Manager, error) {
		return &mockManager{
			start: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
		}, nil
	}, logr.Discard())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	ctx, superCancel := context.WithCancel(context.Background())
	outChan := make(chan error)
	go func() {
		outChan <- sm.Start(ctx)
	}()

	superCancel()
	err = <-outChan
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	subManagerComplete := make(chan struct{})
	testManager := &mockManager{
		start: func(ctx context.Context) error {
			<-ctx.Done()
			subManagerComplete <- struct{}{}
			return nil
		},
	}

	_, err = sm.AddCancelable(testManager)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	<-subManagerComplete
}

type mockManager struct {
	ctrl.Manager
	start func(ctx context.Context) error
}

func (m *mockManager) Start(ctx context.Context) error {
	return m.start(ctx)
}

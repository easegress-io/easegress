/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package worker provides the worker for FaaSController.
package worker

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/function/provider"
	"github.com/megaease/easegress/v2/pkg/object/function/spec"
	"github.com/megaease/easegress/v2/pkg/object/function/storage"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

type (
	// Worker stores the worker information
	Worker struct {
		mutex     sync.RWMutex
		superSpec *supervisor.Spec

		name string

		ingress  *ingressServer
		store    storage.Storage
		provider provider.FaaSProvider

		// for storing manually starting functions' names
		starFunctions sync.Map

		syncInterval string
		done         chan (struct{})
	}
)

// NewWorker return a worker
func NewWorker(superSpec *supervisor.Spec) *Worker {
	store := storage.NewStorage(superSpec.Name(), superSpec.Super().Cluster())
	faasProvider := provider.NewProvider(superSpec)
	ingress := newIngressServer(superSpec, superSpec.Name())
	adm := superSpec.ObjectSpec().(*spec.Admin)

	w := &Worker{
		superSpec:    superSpec,
		store:        store,
		name:         superSpec.Name(),
		provider:     faasProvider,
		ingress:      ingress,
		syncInterval: adm.SyncInterval,

		done:          make(chan struct{}),
		starFunctions: sync.Map{},
		mutex:         sync.RWMutex{},
	}

	go w.run()
	return w
}

func (worker *Worker) run() {
	syncInterval, err := time.ParseDuration(worker.syncInterval)
	if err != nil {
		logger.Errorf("BUG: parse default sync interval: %s failed: %v",
			syncInterval, err)
		return
	}

	if err = worker.ingress.Init(); err != nil {
		logger.Errorf("worker ingress init failed: %v", err)
		return
	}

	if err = worker.provider.Init(); err != nil {
		logger.Errorf("worker's faas provider init failed: %v", err)
		return
	}

	worker.registerAPIs()

	go worker.syncStatus(syncInterval)
}

// updateStatus rebase functions' status by comparing FaaSProvider's function
// status and local store's function status.
func (worker *Worker) updateStatus() {
	// get all function
	functionList, err := worker.listFunctions()
	if err != nil {
		logger.Errorf("list function failed: %v", err)
		return
	}

	allFunctionMap := map[string]*spec.Function{}
	needUpdateFunction := []*spec.Status{}
	for _, function := range functionList {
		allFunctionMap[function.Spec.Name] = function

		if function.Status.State == spec.InactiveState {
			if _, exist := worker.starFunctions.Load(function.Spec.Name); !exist {
				continue
			}
			// function is inactive and user haven't start it manually
			// ignore event from FaaSProvider
		}

		// get function provision status inside faas provider
		providerStatus, err := worker.provider.GetStatus(function.Spec.Name)
		if err != nil {
			continue
		}

		if stateUpdated, err := function.Next(providerStatus.Event); err != nil {
			// not need to update
		} else {
			if stateUpdated {
				function.Status.ExtData = providerStatus.ExtData
				logger.Debugf("need update function: %s, spec:%#v status:%#v ",
					function.Spec.Name, function.Spec, function.Status)
				needUpdateFunction = append(needUpdateFunction, function.Status)
			}
		}
	}

	// update function status if needed and then
	for _, v := range needUpdateFunction {
		if err := worker.updateFunctionStatus(v); err != nil {
			logger.Errorf("update function: %s status failed: %v", v.Name, err)
			continue
		}
	}

	// check inactive and manually started functions' new state
	// if it is changed successfully, should remove it from worker.startFunctions.
	// not matter its' state is active, initial, or failed.
	for _, v := range allFunctionMap {
		if _, exist := worker.starFunctions.Load(v.Spec.Name); exist {
			if v.Status.State != spec.InactiveState {
				worker.starFunctions.Delete(v.Spec.Name)
			}
		}
	}

	// call ingress server reconciling all function pipeline state
	worker.ingress.Update(allFunctionMap)
}

// syncStatus sync function's status with
func (worker *Worker) syncStatus(syncInterval time.Duration) {
	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					worker.superSpec.Name(), err, debug.Stack())
			}
		}()

		worker.updateStatus()
	}
	for {
		select {
		case <-worker.done:
			return
		case <-time.After(syncInterval):
			routine()
		}
	}
}

// Close closes the Egress HTTPServer and Pipelines
func (worker *Worker) Close() {
	worker.mutex.Lock()
	defer worker.mutex.Unlock()

	close(worker.done)
	worker.ingress.Close()
}

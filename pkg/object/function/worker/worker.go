/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://wwww.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package worker

import (
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/function/provider"
	"github.com/megaease/easegress/pkg/object/function/spec"
	"github.com/megaease/easegress/pkg/object/function/storage"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	defaultSyncInterval = "5s"
)

type (
	Worker struct {
		mutex     sync.RWMutex
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec

		name string

		ingress  *ingressServer
		store    storage.Storage
		provider provider.FaaSProvider

		syncInterval time.Duration
		done         chan (struct{})
	}
)

// newWorker return a worker
func NewWorker(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	store := storage.NewStorage(superSpec.Name(), super.Cluster())
	faasProvider := provider.NewProvider(superSpec)
	ingress := newIngressServer(superSpec, super, superSpec.Name())

	w := &Worker{
		super:     super,
		superSpec: superSpec,
		store:     store,
		name:      superSpec.Name(),
		provider:  faasProvider,
		ingress:   ingress,

		done:  make(chan struct{}),
		mutex: sync.RWMutex{},
	}

	w.registerAPIs()
	go w.run()
	return w
}

func (worker *Worker) run() {
	var err error
	worker.syncInterval, err = time.ParseDuration(defaultSyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse default sync interval: %s failed: %v",
			defaultSyncInterval, err)
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

	go worker.syncStatus()
}

// updateStatus rebase functions' status by comparing FaaSProvider's function
// status and local store's function status,e.g.,
// 1) if FaaS provider provision function successfully, local "pending"/"failed" status's function
//    will be turn into "active"
// 2) if FaaS provider provision function failed, not matter function status is, they will
//    be turn into "failed" and the corresponding ingress pipeline will be stopped.
// 3) if the local function is in "inactive" status, even provision successfully won't trigger
//    function be turned into another status.
func (worker *Worker) updateStatus() {
	// get all function
	functionList, err := worker.listFunctions()
	if err != nil {
		logger.Errorf("list function failed: %v", err)
		return
	}

	allFunctionMap := map[string]*spec.Status{}
	needUpdateFunction := []*spec.Status{}
	for _, function := range functionList {
		allFunctionMap[function.Spec.Name] = function.Status
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
				logger.Infof("need update function %#v", function)
				needUpdateFunction = append(needUpdateFunction, function.Status)
			}
		}
	}

	// update function status if needed and then
	for _, v := range needUpdateFunction {
		if err := worker.updateFunctionStatus(v); err != nil {
			continue
		}
	}

	// call ingress server reconciling all function pipeline state
	worker.ingress.Update(allFunctionMap)
}

// syncStatus sync function's status with
func (worker *Worker) syncStatus() {
	for {
		select {
		case <-worker.done:
			return
		case <-time.After(worker.syncInterval):
			worker.updateStatus()
		}
	}
}

// Close closes the Egress HTTPServer and Pipelines
func (worker *Worker) Close() {
	worker.mutex.Lock()
	defer worker.mutex.Unlock()

	worker.ingress.Close()
}

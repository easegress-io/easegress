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

package worker

import (
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/function/spec"
	"github.com/megaease/easegress/v2/pkg/object/function/storage"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

// put puts a function spec and status into store.
func (worker *Worker) put(funcSpec *spec.Spec) error {
	buf, err := codectool.MarshalJSON(funcSpec)
	if err != nil {
		return err
	}
	key := storage.GetFunctionSpecPrefix(worker.name, funcSpec.Name)
	if err = worker.store.Put(key, string(buf)); err != nil {
		logger.Errorf("put function:%s spec failed: %v", funcSpec.Name, err)
		return err
	}

	status := &spec.Status{
		Name:  funcSpec.Name,
		State: spec.InitState(),
		Event: spec.CreateEvent,
	}
	buf, err = codectool.MarshalJSON(status)
	if err != nil {
		return err
	}
	key = storage.GetFunctionStatusPrefix(worker.name, funcSpec.Name)
	if err = worker.store.Put(key, string(buf)); err != nil {
		logger.Errorf("put function:%s status failed: %v", funcSpec.Name, err)
		return err
	}
	return nil
}

func (worker *Worker) listFunctions() ([]*spec.Function, error) {
	functions := []*spec.Function{}
	specs, err := worker.listAllFunctionSpecs()
	if err != nil {
		logger.Errorf("worker list all function specs failed: %v", err)
		return functions, nil
	}

	statuses, err := worker.listAllFunctionStatus()
	if err != nil {
		logger.Errorf("worker list all function status failed: %v", err)
		return functions, nil
	}
	for _, funcSpec := range specs {
		var status *spec.Status
		for _, s := range statuses {
			if funcSpec.Name == s.Name {
				status = s
			}
		}
		if status != nil {
			fsm, err := spec.InitFSM(status.State)
			if err != nil {
				logger.Errorf("List all functions, in function: %s, failed: %v", funcSpec.Name, err)
				continue
			}

			functions = append(functions, &spec.Function{
				Spec:   funcSpec,
				Status: status,
				Fsm:    fsm,
			})
		}
	}
	return functions, nil
}

func (worker *Worker) get(functionName string) (*spec.Function, error) {
	funcSpec, err := worker.getFunctionSpec(functionName)
	if err != nil {
		logger.Errorf("get function: %s's spec failed: %v", functionName, err)
		return nil, err
	}
	status, err := worker.getFunctionStatus(functionName)
	if err != nil {
		logger.Errorf("get function: %s's status failed: %v", functionName, err)
		return nil, err
	}
	fsm, err := spec.InitFSM(status.State)
	if err != nil {
		logger.Errorf("init function: %s's fsm failed: %v ", functionName, err)
		return nil, err
	}

	return &spec.Function{
		Spec:   funcSpec,
		Status: status,
		Fsm:    fsm,
	}, nil
}

func (worker *Worker) updateFunctionStatus(status *spec.Status) error {
	buff, err := codectool.MarshalJSON(status)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", status, err)
		return err
	}

	key := storage.GetFunctionStatusPrefix(worker.name, status.Name)
	err = worker.store.Put(key, string(buff))
	if err != nil {
		logger.Errorf("update function: %s's status failed: %v", status.Name, err)
		return err
	}
	return nil
}

func (worker *Worker) updateFunctionSpec(funcSpec *spec.Spec) error {
	buff, err := codectool.MarshalJSON(funcSpec)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", funcSpec, err)
		return err
	}

	key := storage.GetFunctionSpecPrefix(worker.name, funcSpec.Name)
	err = worker.store.Put(key, string(buff))
	if err != nil {
		logger.Errorf("update function: %s's spec failed: %v", funcSpec.Name, err)
		return err
	}
	return nil
}

func (worker *Worker) listAllFunctionStatus() ([]*spec.Status, error) {
	return worker.listFunctionStatus(true, "")
}

func (worker *Worker) getFunctionStatus(functionName string) (*spec.Status, error) {
	status, err := worker.listFunctionStatus(false, functionName)
	if err != nil {
		return nil, err
	}
	if len(status) == 0 {
		return nil, errFunctionNotFound
	}

	return status[0], nil
}

func (worker *Worker) listFunctionStatus(all bool, functionName string) ([]*spec.Status, error) {
	status := []*spec.Status{}
	var prefix string
	if all {
		prefix = storage.GetAllFunctionStatusPrefix(worker.name)
	} else {
		prefix = storage.GetFunctionStatusPrefix(worker.name, functionName)
	}

	kvs, err := worker.store.GetPrefix(prefix)
	if err != nil {
		return status, err
	}

	for _, v := range kvs {
		_status := &spec.Status{}
		if err = codectool.Unmarshal([]byte(v), _status); err != nil {
			logger.Errorf("BUG: unmarshal %s to json failed: %v", v, err)
			continue
		}

		status = append(status, _status)
	}

	return status, nil
}

func (worker *Worker) listAllFunctionSpecs() ([]*spec.Spec, error) {
	return worker.listFunctionSpecs(true, "")
}

func (worker *Worker) getFunctionSpec(functionName string) (*spec.Spec, error) {
	specs, err := worker.listFunctionSpecs(false, functionName)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, errFunctionNotFound
	}

	return specs[0], nil
}

func (worker *Worker) listFunctionSpecs(all bool, functionName string) ([]*spec.Spec, error) {
	specs := []*spec.Spec{}
	var prefix string
	if all {
		prefix = storage.GetAllFunctionSpecPrefix(worker.name)
	} else {
		prefix = storage.GetFunctionSpecPrefix(worker.name, functionName)
	}

	kvs, err := worker.store.GetPrefix(prefix)
	if err != nil {
		return specs, err
	}

	for _, v := range kvs {
		_spec := &spec.Spec{}
		if err = codectool.Unmarshal([]byte(v), _spec); err != nil {
			logger.Errorf("BUG: unmarshal %s to json failed: %v", v, err)
			continue
		}

		specs = append(specs, _spec)
	}

	return specs, nil
}

// Lock locks the cluster store.
func (worker *Worker) Lock() error {
	return worker.store.Lock()
}

// Unlock unlocks the cluster store.
func (worker *Worker) Unlock() {
	worker.store.Unlock()
}

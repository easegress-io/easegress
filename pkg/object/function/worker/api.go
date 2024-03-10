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
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/function/spec"
	"github.com/megaease/easegress/v2/pkg/object/function/storage"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/v"
)

var (
	errFunctionNotFound     = fmt.Errorf("can't find function")
	errFunctionAlreadyExist = fmt.Errorf("function already exist")
	startMsg                = []byte("function has been started, please wait for system turning it into active status")
)

func (worker *Worker) faasAPIPrefix() string {
	return fmt.Sprintf("/faas/%s", worker.name)
}

const apiGroupName = "faas_admin"

func (worker *Worker) registerAPIs() {
	group := &api.Group{
		Group: apiGroupName,
		Entries: []*api.Entry{
			{Path: worker.faasAPIPrefix(), Method: "POST", Handler: worker.Create},
			{Path: worker.faasAPIPrefix(), Method: "GET", Handler: worker.List},
			{Path: worker.faasAPIPrefix() + "/{name}", Method: "GET", Handler: worker.Get},
			{Path: worker.faasAPIPrefix() + "/{name}/start", Method: "PUT", Handler: worker.Start},
			{Path: worker.faasAPIPrefix() + "/{name}/stop", Method: "PUT", Handler: worker.Stop},
			{Path: worker.faasAPIPrefix() + "/{name}", Method: "DELETE", Handler: worker.Delete},
			{Path: worker.faasAPIPrefix() + "/{name}", Method: "PUT", Handler: worker.Update},
		},
	}

	api.RegisterAPIs(group)
}

// UnregisterAPIs unregister APIs
func (worker *Worker) UnregisterAPIs() {
	api.UnregisterAPIs(apiGroupName)
}

func (worker *Worker) readFunctionName(w http.ResponseWriter, r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "name")
	if serviceName == "" {
		return "", fmt.Errorf("empty service name")
	}

	return serviceName, nil
}

// Create deals with HTTP POST method
func (worker *Worker) Create(w http.ResponseWriter, r *http.Request) {
	spec := &spec.Spec{}
	err := worker.readAPISpec(w, r, spec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		logger.Errorf("create function with bad request: %v", err)
		return
	}
	worker.store.Lock()
	defer worker.store.Unlock()

	_, err = worker.getFunctionSpec(spec.Name)
	if err != nil && err != errFunctionNotFound {
		logger.Errorf("create function: %s by getting faas spec failed: %v", spec.Name, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	// already created
	if err == nil {
		api.HandleAPIError(w, r, http.StatusConflict, errFunctionAlreadyExist)
		return
	}

	if err = worker.provider.Create(spec); err != nil {
		logger.Errorf("create function: %s by calling faas provider failed: %v", spec.Name, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	if err := worker.put(spec); err != nil {
		logger.Errorf("create function: %s by setting store failed: %v", spec.Name, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if err = worker.ingress.Put(spec); err != nil {
		logger.Errorf("[BUG] create function: %s by add ingress failed: %v", spec.Name, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Location", r.URL.Path+"/"+spec.Name)
	w.WriteHeader(http.StatusCreated)
}

func (worker *Worker) readAPISpec(w http.ResponseWriter, r *http.Request, spec interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}
	err = codectool.Unmarshal(body, spec)
	if err != nil {
		return fmt.Errorf("unmarshal to json failed: %v", err)
	}

	vr := v.Validate(spec)
	if !vr.Valid() {
		return fmt.Errorf("validate failed: \n%s", vr.Error())
	}

	return nil
}

func (worker *Worker) updateState(w http.ResponseWriter, r *http.Request, event spec.Event) (name string, err error) {
	name, err = worker.readFunctionName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	function, err := worker.get(name)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	stateUpdated := false
	if stateUpdated, err = function.Next(event); err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	// not need to update function's status.
	if !stateUpdated {
		return
	}
	err = worker.store.Lock()
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	defer worker.store.Unlock()
	err = worker.updateFunctionStatus(function.Status)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	return
}

// Stop is a stop API which deals with HTTP PUT method
func (worker *Worker) Stop(w http.ResponseWriter, r *http.Request) {
	name, err := worker.updateState(w, r, spec.StopEvent)
	if err != nil {
		logger.Errorf("worker stop function failed, %v", err)
		return
	}
	worker.ingress.Stop(name)
}

// Start is a start API which deals with HTTP PUT method
func (worker *Worker) Start(w http.ResponseWriter, r *http.Request) {
	name, err := worker.updateState(w, r, spec.StartEvent)
	if err != nil {
		return
	}

	worker.starFunctions.Store(name, struct{}{})
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Write(startMsg)
}

// Delete deals with HTTP DELETE
func (worker *Worker) Delete(w http.ResponseWriter, r *http.Request) {
	name, err := worker.readFunctionName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	function, err := worker.get(name)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if _, err = function.Next(spec.DeleteEvent); err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = worker.Lock()
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	defer worker.Unlock()

	// delete function in FaaS Provider
	if err = worker.provider.Delete(name); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	// remove route in Ingress
	worker.ingress.Delete(name)
	if err = worker.store.Delete(storage.GetFunctionSpecPrefix(worker.name, name)); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if err = worker.store.Delete(storage.GetFunctionStatusPrefix(worker.name, name)); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
}

// List deals with HTTP GET method
func (worker *Worker) List(w http.ResponseWriter, r *http.Request) {
	functions, err := worker.listFunctions()
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	buff, err := codectool.MarshalJSON(functions)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("marshal %#v to json failed: %v", functions, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

// Update deals with HTTP PUT method
func (worker *Worker) Update(w http.ResponseWriter, r *http.Request) {
	funcSpec := &spec.Spec{}
	err := worker.readAPISpec(w, r, funcSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		logger.Errorf("update function with bad request: %v")
		return
	}
	name, err := worker.readFunctionName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		logger.Errorf("update function with bad request: %v")
		return
	}
	if name != funcSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("update URL name %s, specname :%s, mismatch", name, funcSpec.Name))
		return
	}

	function, err := worker.get(funcSpec.Name)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	stateUpdated := false
	if stateUpdated, err = function.Next(spec.UpdateEvent); err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	worker.Lock()
	defer worker.Unlock()
	// update the FaaS provider related filed
	if err := worker.provider.Update(funcSpec); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		logger.Errorf("update function: %s failed: %v", funcSpec.Name, err)
		return
	}

	if err = worker.updateFunctionSpec(funcSpec); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	if stateUpdated {
		if err = worker.updateFunctionStatus(function.Status); err != nil {
			api.HandleAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
	}
}

// Get deals with HTTP GET method request
func (worker *Worker) Get(w http.ResponseWriter, r *http.Request) {
	name, err := worker.readFunctionName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	function, err := worker.get(name)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		logger.Errorf("create function with bad request: %v", err)
		return
	}

	// no display
	function.Fsm = nil

	buff, err := codectool.MarshalJSON(function)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("marshal %#v to json failed: %v", function, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

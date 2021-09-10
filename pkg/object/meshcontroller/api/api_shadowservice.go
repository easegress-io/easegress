/*
 * Copyright (c) 2017, MegaEase
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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easemesh-api/v1alpha1"
)

func (a *API) readShadowServiceName(r *http.Request) (string, error) {
	name := chi.URLParam(r, "shadowServiceName")
	if name == "" {
		return "", fmt.Errorf("empty shadow service name")
	}

	return name, nil
}

type shadowServicesByOrder []*spec.ShadowService

func (s shadowServicesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s shadowServicesByOrder) Len() int           { return len(s) }
func (s shadowServicesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (a *API) listShadowServices(w http.ResponseWriter, r *http.Request) {
	specs := a.shadowService.ListSpecs()
	svcSpecs := a.service.ListServiceSpecs()
	for _, spec := range specs {
		for _, svcSpec := range svcSpecs {
			if spec.ServiceName == svcSpec.Name {
				spec.Service = svcSpec
				break
			}
		}
	}

	sort.Sort(shadowServicesByOrder(specs))
	var pbSpecs []*v1alpha1.ShadowService
	for _, v := range specs {
		pbSpec := &v1alpha1.ShadowService{}
		err := a.convertSpecToPB(v, pbSpec)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbSpecs = append(pbSpecs, pbSpec)
	}

	buff, err := json.Marshal(pbSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) createShadowService(w http.ResponseWriter, r *http.Request) {
	name, err := a.readShadowServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	spec := &spec.ShadowService{}
	err = a.readAPISpec(w, r, &v1alpha1.ShadowService{}, spec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if name != spec.Name {
		msg := fmt.Errorf("name conflict: %s %s", name, spec.Name)
		api.HandleAPIError(w, r, http.StatusBadRequest, msg)
		return
	}
	spec.Service = nil

	a.Lock()
	defer a.Unlock()

	if svc := a.service.GetServiceSpec(spec.ServiceName); svc == nil {
		msg := fmt.Errorf("service %s does not existed", spec.ServiceName)
		api.HandleAPIError(w, r, http.StatusNotFound, msg)
		return
	}

	if ssvc := a.shadowService.GetSpec(spec.Name); ssvc != nil {
		msg := fmt.Errorf("shadow service %s already existed", spec.Name)
		api.HandleAPIError(w, r, http.StatusConflict, msg)
		return
	}

	a.shadowService.PutSpec(spec)

	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
}

func (a *API) getShadowService(w http.ResponseWriter, r *http.Request) {
	name, err := a.readShadowServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	spec := a.shadowService.GetSpec(name)
	if spec == nil {
		msg := fmt.Errorf("shadow service %s does not existed", name)
		api.HandleAPIError(w, r, http.StatusNotFound, msg)
		return
	}

	if svcSpec := a.service.GetServiceSpec(spec.ServiceName); svcSpec != nil {
		spec.Service = svcSpec
	}

	pbSpec := &v1alpha1.ShadowService{}
	err = a.convertSpecToPB(spec, pbSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", spec, err))
	}

	buff, err := json.Marshal(pbSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", spec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) updateShadowService(w http.ResponseWriter, r *http.Request) {
	name, err := a.readShadowServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	spec := &spec.ShadowService{}
	err = a.readAPISpec(w, r, &v1alpha1.ShadowService{}, spec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if name != spec.Name {
		msg := fmt.Errorf("name conflict: %s %s", name, spec.Name)
		api.HandleAPIError(w, r, http.StatusBadRequest, msg)
		return
	}
	spec.Service = nil

	a.Lock()
	defer a.Unlock()

	if oldSpec := a.shadowService.GetSpec(name); oldSpec == nil {
		msg := fmt.Errorf("shadow service %s does not existed", name)
		api.HandleAPIError(w, r, http.StatusNotFound, msg)
		return
	}

	if svc := a.service.GetServiceSpec(spec.ServiceName); svc == nil {
		msg := fmt.Errorf("service %s does not existed", spec.ServiceName)
		api.HandleAPIError(w, r, http.StatusNotFound, msg)
		return
	}

	a.shadowService.PutSpec(spec)
}

func (a *API) deleteShadowService(w http.ResponseWriter, r *http.Request) {
	name, err := a.readShadowServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.Lock()
	defer a.Unlock()

	if spec := a.shadowService.GetSpec(name); spec == nil {
		msg := fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, msg)
		return
	}

	a.shadowService.DeleteSpec(name)
}

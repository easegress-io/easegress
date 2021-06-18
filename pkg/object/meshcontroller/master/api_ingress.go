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

package master

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	v1alpha1 "github.com/megaease/easemesh-api/v1alpha1"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

type ingressesByOrder []*spec.Ingress

func (s ingressesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s ingressesByOrder) Len() int           { return len(s) }
func (s ingressesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *Master) readIngressName(w http.ResponseWriter, r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "ingressName")
	if serviceName == "" {
		return "", fmt.Errorf("empty ingress name")
	}

	return serviceName, nil
}

func (m *Master) listIngresses(w http.ResponseWriter, r *http.Request) {
	specs := m.service.ListIngressSpecs()

	sort.Sort(ingressesByOrder(specs))
	var apiSpecs []*v1alpha1.Ingress
	for _, v := range specs {
		ingress := &v1alpha1.Ingress{}
		err := m.convertSpecToPB(v, ingress)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, ingress)
	}
	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (m *Master) createIngress(w http.ResponseWriter, r *http.Request) {
	pbIngressSpec := &v1alpha1.Ingress{}
	ingressSpec := &spec.Ingress{}

	ingressName, err := m.readIngressName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(w, r, pbIngressSpec, ingressSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if ingressName != ingressSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", ingressName, ingressSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetIngressSpec(ingressName)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", ingressName))
		return
	}

	m.service.PutIngressSpec(ingressSpec)

	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
}

func (m *Master) getIngress(w http.ResponseWriter, r *http.Request) {
	ingressName, err := m.readIngressName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	ingressSpec := m.service.GetIngressSpec(ingressName)
	if ingressSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}
	pbIngressSpec := &v1alpha1.Ingress{}
	err = m.convertSpecToPB(ingressSpec, pbIngressSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", ingressSpec, err))
	}

	buff, err := json.Marshal(pbIngressSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbIngressSpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (m *Master) updateIngress(w http.ResponseWriter, r *http.Request) {
	pbIngressSpec := &v1alpha1.Ingress{}
	ingressSpec := &spec.Ingress{}

	ingressName, err := m.readIngressName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(w, r, pbIngressSpec, ingressSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if ingressName != ingressSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", ingressName, ingressSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetIngressSpec(ingressName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}

	m.service.PutIngressSpec(ingressSpec)
}

func (m *Master) deleteIngress(w http.ResponseWriter, r *http.Request) {
	ingressName, err := m.readIngressName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetIngressSpec(ingressName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}

	m.service.DeleteIngressSpec(ingressName)
}

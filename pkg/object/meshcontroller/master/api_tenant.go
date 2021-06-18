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
	"time"

	"github.com/go-chi/chi/v5"
	v1alpha1 "github.com/megaease/easemesh-api/v1alpha1"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

type tenantsByOrder []*spec.Tenant

func (s tenantsByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s tenantsByOrder) Len() int           { return len(s) }
func (s tenantsByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *Master) readTenantName(w http.ResponseWriter, r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "tenantName")
	if serviceName == "" {
		return "", fmt.Errorf("empty tenant name")
	}

	return serviceName, nil
}

func (m *Master) listTenants(w http.ResponseWriter, r *http.Request) {
	specs := m.service.ListTenantSpecs()

	sort.Sort(tenantsByOrder(specs))

	var apiSpecs []*v1alpha1.Tenant
	for _, v := range specs {
		tenant := &v1alpha1.Tenant{}
		err := m.convertSpecToPB(v, &tenant)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, tenant)
	}

	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (m *Master) createTenant(w http.ResponseWriter, r *http.Request) {
	pbTenantSpec := &v1alpha1.Tenant{}
	tenantSpec := &spec.Tenant{}

	tenantName, err := m.readTenantName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(w, r, pbTenantSpec, tenantSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if tenantName != tenantSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", tenantName, tenantSpec.Name))
		return
	}

	if len(tenantSpec.Services) > 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("services are not empty"))
		return
	}
	tenantSpec.CreatedAt = time.Now().Format(time.RFC3339)

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetTenantSpec(tenantName)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", tenantName))
		return
	}

	m.service.PutTenantSpec(tenantSpec)

	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
}

func (m *Master) getTenant(w http.ResponseWriter, r *http.Request) {
	tenantName, err := m.readTenantName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	tenantSpec := m.service.GetTenantSpec(tenantName)
	if tenantSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	pbTenantSpec := &v1alpha1.Tenant{}
	err = m.convertSpecToPB(tenantSpec, pbTenantSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", tenantSpec, err))
	}

	buff, err := json.Marshal(pbTenantSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbTenantSpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (m *Master) updateTenant(w http.ResponseWriter, r *http.Request) {
	pbTenantSpec := &v1alpha1.Tenant{}
	tenantSpec := &spec.Tenant{}

	tenantName, err := m.readTenantName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(w, r, pbTenantSpec, tenantSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if tenantName != tenantSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", tenantName, tenantSpec.Name))
		return
	}

	if len(tenantSpec.Services) > 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("services are not empty"))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetTenantSpec(tenantName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	// NOTE: The fields below can't be updated.
	tenantSpec.Services, tenantSpec.CreatedAt = oldSpec.Services, oldSpec.CreatedAt

	m.service.PutTenantSpec(tenantSpec)
}

func (m *Master) deleteTenant(w http.ResponseWriter, r *http.Request) {
	tenantName, err := m.readTenantName(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetTenantSpec(tenantName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	if len(oldSpec.Services) != 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("%s got services: %v", tenantName, oldSpec.Services))
		return
	}

	m.service.DeleteTenantSpec(tenantName)
}

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

package api

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// ObjectPrefix is the object prefix.
	ObjectPrefix = "/objects"

	// ObjectKindsPrefix is the object-kinds prefix.
	ObjectKindsPrefix = "/object-kinds"

	// ObjectTemplatePrefix is the object-template prefix.
	ObjectTemplatePrefix = "/objects-yaml"

	// StatusObjectPrefix is the prefix of object status.
	StatusObjectPrefix = "/status/objects"

	// ObjectAPIResourcesPrefix is the prefix of object api resources.
	ObjectAPIResourcesPrefix = "/object-api-resources"
)

func RegisterValidateHook() {}

func (s *Server) objectAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    ObjectKindsPrefix,
			Method:  "GET",
			Handler: s.listObjectKinds,
		},
		{
			Path:    ObjectAPIResourcesPrefix,
			Method:  "GET",
			Handler: s.listObjectAPIResources,
		},
		{
			Path:    ObjectPrefix,
			Method:  "POST",
			Handler: s.createObject,
		},
		{
			Path:    ObjectPrefix,
			Method:  "GET",
			Handler: s.listObjects,
		},
		{
			Path:    ObjectPrefix + "/{name}",
			Method:  "GET",
			Handler: s.getObject,
		},
		{
			Path:    ObjectTemplatePrefix + "/{kind}/{name}",
			Method:  "GET",
			Handler: s.getObjectTemplate,
		},
		{
			Path:    ObjectPrefix + "/{name}",
			Method:  "PUT",
			Handler: s.updateObject,
		},
		{
			Path:    ObjectPrefix + "/{name}",
			Method:  "DELETE",
			Handler: s.deleteObject,
		},
		{
			Path:    ObjectPrefix,
			Method:  "DELETE",
			Handler: s.deleteObjects,
		},
		{
			Path:    StatusObjectPrefix,
			Method:  "GET",
			Handler: s.listStatusObjects,
		},
		{
			Path:    StatusObjectPrefix + "/{name}",
			Method:  "GET",
			Handler: s.getStatusObject,
		},
	}
}

func (s *Server) readObjectSpec(w http.ResponseWriter, r *http.Request) (*supervisor.Spec, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %v", err)
	}

	spec, err := s.super.CreateSpec(string(body))
	if err != nil {
		return nil, err
	}
	name := chi.URLParam(r, "name")

	if name != "" && name != spec.Name() {
		return nil, fmt.Errorf("inconsistent name in url and spec ")
	}

	return spec, err
}

func (s *Server) upgradeConfigVersion(w http.ResponseWriter, r *http.Request) {
	version := s._plusOneVersion()
	w.Header().Set(ConfigVersionKey, fmt.Sprintf("%d", version))
}

func (s *Server) createObject(w http.ResponseWriter, r *http.Request) {
	spec, err := s.readObjectSpec(w, r)
	if err != nil {
		HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	if spec.Categroy() == supervisor.CategorySystemController {
		HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("can't create system controller object"))
	}

	name := spec.Name()

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getObject(name)
	if existedSpec != nil {
		HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("conflict name: %s", name))
		return
	}

	// Validate hooks.
	for _, hook := range objectValidateHooks {
		err := hook(OperationTypeCreate, spec)
		if err != nil {
			HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("validate failed: %v", err))
			return
		}
	}

	s._putObject(spec)
	s.upgradeConfigVersion(w, r)

	w.WriteHeader(http.StatusCreated)
	location := fmt.Sprintf("%s/%s", r.URL.Path, name)
	w.Header().Set("Location", location)
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	s.Lock()
	defer s.Unlock()

	spec := s._getObject(name)

	if spec.Categroy() == supervisor.CategorySystemController {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("can't delete system controller object"))
		return
	}

	if spec == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	// Validate hooks.
	for _, hook := range objectValidateHooks {
		err := hook(OperationTypeDelete, spec)
		if err != nil {
			HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("validate failed: %v", err))
			return
		}
	}

	s._deleteObject(name)
	s.upgradeConfigVersion(w, r)
}

func (s *Server) deleteObjects(w http.ResponseWriter, r *http.Request) {
	allFlag := r.URL.Query().Get("all")
	if allFlag == "true" {
		s.Lock()
		defer s.Unlock()

		specs := s._listObjects()
		for _, spec := range specs {
			if spec.Categroy() == supervisor.CategorySystemController {
				continue
			}

			// Validate hooks.
			for _, hook := range objectValidateHooks {
				err := hook(OperationTypeDelete, spec)
				if err != nil {
					HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("validate failed: %v", err))
					return
				}
			}

			s._deleteObject(spec.Name())
		}

		s.upgradeConfigVersion(w, r)
	}
}

// getObjectTemplate returns the template of the object in yaml format.
// The body is in yaml format to keep the order of fields.
func (s *Server) getObjectTemplate(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	name := chi.URLParam(r, "name")

	allKinds := supervisor.ObjectKinds()
	for _, k := range allKinds {
		if strings.EqualFold(k, kind) {
			kind = k
		}
	}

	obj := supervisor.GetObject(kind)
	if obj == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	specByte, err := codectool.MarshalYAML(obj.DefaultSpec())
	if err != nil {
		HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	metaByte, err := codectool.MarshalYAML(supervisor.NewMeta(kind, name))
	if err != nil {
		HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	spec := fmt.Sprintf("%s\n%s", metaByte, specByte)

	w.Header().Set("Content-Type", "text/x-yaml")
	w.Write([]byte(spec))
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_, namespace := parseNamespaces(r)
	if namespace != "" && namespace != DefaultNamespace {
		spec := s._getObjectByNamespace(namespace, name)
		if spec == nil {
			HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
			return
		}

		WriteBody(w, r, spec)
		return
	}

	// No need to lock.
	spec := s._getObject(name)
	if spec == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}
	WriteBody(w, r, spec)
}

func (s *Server) updateObject(w http.ResponseWriter, r *http.Request) {
	spec, err := s.readObjectSpec(w, r)
	if err != nil {
		HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	name := spec.Name()

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getObject(name)
	if existedSpec == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	if existedSpec.Kind() != spec.Kind() {
		HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("different kinds: %s, %s",
				existedSpec.Kind(), spec.Kind()))
		return
	}

	// Validate hooks.
	for _, hook := range objectValidateHooks {
		err := hook(OperationTypeUpdate, spec)
		if err != nil {
			HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("validate failed: %v", err))
			return
		}
	}

	s._putObject(spec)
	s.upgradeConfigVersion(w, r)
}

func parseNamespaces(r *http.Request) (bool, string) {
	allNamespaces := strings.TrimSpace(r.URL.Query().Get("all-namespaces"))
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	flag, err := strconv.ParseBool(allNamespaces)
	if err != nil {
		return false, namespace
	}
	return flag, namespace
}

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	allNamespaces, namespace := parseNamespaces(r)
	if allNamespaces && namespace != "" {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("conflict query params, can't set all-namespaces and namespace at the same time"))
		return
	}
	if allNamespaces {
		allSpecs := s._listAllNamespaces()
		allSpecs[DefaultNamespace] = s._listObjects()
		WriteBody(w, r, allSpecs)
		return
	}
	if namespace != "" && namespace != DefaultNamespace {
		specs := s._listNamespaces(namespace)
		WriteBody(w, r, specs)
		return
	}

	// allNamespaces == false && namespace == ""
	// No need to lock.
	specs := specList(s._listObjects())
	// NOTE: Keep it consistent.
	sort.Sort(specs)

	WriteBody(w, r, specs)
}

func (s *Server) getStatusObject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_, namespace := parseNamespaces(r)

	var spec *supervisor.Spec
	if namespace == "" || namespace == DefaultNamespace {
		spec = s._getObject(name)
	} else {
		spec = s._getObjectByNamespace(namespace, name)
	}
	if spec == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	_, isTraffic := supervisor.TrafficObjectKinds[spec.Kind()]
	status := s._getStatusObject(namespace, name, isTraffic)
	WriteBody(w, r, status)
}

func (s *Server) listStatusObjects(w http.ResponseWriter, r *http.Request) {
	// No need to lock.

	status := s._listStatusObjects()

	WriteBody(w, r, status)
}

type specList []*supervisor.Spec

func (s specList) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s specList) Len() int           { return len(s) }
func (s specList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s specList) Marshal() ([]byte, error) {
	specs := []map[string]interface{}{}
	for _, spec := range s {
		var m map[string]interface{}
		err := codectool.Unmarshal([]byte(spec.JSONConfig()), &m)
		if err != nil {
			return nil, fmt.Errorf("unmarshal %s to json failed: %v",
				spec.JSONConfig(), err)
		}
		specs = append(specs, m)
	}

	buff, err := codectool.MarshalJSON(specs)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to json failed: %v", specs, err)
	}

	return buff, nil
}

func (s *Server) listObjectKinds(w http.ResponseWriter, r *http.Request) {
	kinds := supervisor.ObjectKinds()

	WriteBody(w, r, kinds)
}

func (s *Server) listObjectAPIResources(w http.ResponseWriter, r *http.Request) {
	res := ObjectAPIResources()
	WriteBody(w, r, res)
}

func getTrafficController(super *supervisor.Supervisor) *trafficcontroller.TrafficController {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		return nil
	}
	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		return nil
	}
	return tc
}

func (s *Server) _listAllNamespaces() map[string][]*supervisor.Spec {
	tc := getTrafficController(s.super)
	if tc == nil {
		return nil
	}
	res := make(map[string][]*supervisor.Spec)
	allObjects := tc.ListAllNamespace()
	for namespace, objects := range allObjects {
		specs := make([]*supervisor.Spec, 0, len(objects))
		for _, o := range objects {
			specs = append(specs, o.Spec())
		}
		res[namespace] = specs
	}
	return res
}

func (s *Server) _listNamespaces(ns string) []*supervisor.Spec {
	tc := getTrafficController(s.super)
	if tc == nil {
		return nil
	}
	traffics := tc.ListTrafficGates(ns)
	pipelines := tc.ListPipelines(ns)
	specs := make([]*supervisor.Spec, 0, len(traffics)+len(pipelines))
	for _, t := range traffics {
		specs = append(specs, t.Spec())
	}
	for _, p := range pipelines {
		specs = append(specs, p.Spec())
	}
	return specs
}

func (s *Server) _getObjectByNamespace(ns string, name string) *supervisor.Spec {
	tc := getTrafficController(s.super)
	if tc == nil {
		return nil
	}
	object, ok := tc.GetPipeline(ns, name)
	if ok {
		return object.Spec()
	}

	object, ok = tc.GetTrafficGate(ns, name)
	if ok {
		return object.Spec()
	}
	return nil
}

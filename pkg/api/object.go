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
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/codectool"
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
			Handler: s.listObjectApiResources,
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

	name := spec.Name()

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getObject(name)
	if existedSpec != nil {
		HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("conflict name: %s", name))
		return
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
	if spec == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
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

	s._putObject(spec)
	s.upgradeConfigVersion(w, r)
}

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	// No need to lock.
	specs := specList(s._listObjects())
	// NOTE: Keep it consistent.
	sort.Sort(specs)

	WriteBody(w, r, specs)
}

func (s *Server) getStatusObject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	spec := s._getObject(name)

	if spec == nil {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	status := s._getStatusObject(name)

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

func (s *Server) listObjectApiResources(w http.ResponseWriter, r *http.Request) {
	res := ObjectApiResources()
	WriteBody(w, r, res)
}

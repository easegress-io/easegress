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
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/cluster/customdata"
	"github.com/megaease/easegress/pkg/util/spectool"
)

const (
	// CustomDataKindPrefix is the URL prefix of APIs for custom data kind
	CustomDataKindPrefix = "/customdatakinds"
	// CustomDataPrefix is the URL prefix of APIs for custom data
	CustomDataPrefix = "/customdata/{kind}"
)

// ChangeRequest represents a change request to custom data
type ChangeRequest struct {
	Rebuild bool              `json:"rebuild"`
	Delete  []string          `json:"delete"`
	List    []customdata.Data `json:"list"`
}

func (s *Server) customDataAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    CustomDataKindPrefix,
			Method:  http.MethodGet,
			Handler: s.listCustomDataKind,
		},
		{
			Path:    CustomDataKindPrefix + "/{name}",
			Method:  http.MethodGet,
			Handler: s.getCustomDataKind,
		},
		{
			Path:    CustomDataKindPrefix,
			Method:  http.MethodPost,
			Handler: s.createCustomDataKind,
		},
		{
			Path:    CustomDataKindPrefix,
			Method:  http.MethodPut,
			Handler: s.updateCustomDataKind,
		},
		{
			Path:    CustomDataKindPrefix + "/{name}",
			Method:  http.MethodDelete,
			Handler: s.deleteCustomDataKind,
		},

		{
			Path:    CustomDataPrefix,
			Method:  http.MethodGet,
			Handler: s.listCustomData,
		},
		{
			Path:    CustomDataPrefix + "/{id}",
			Method:  http.MethodGet,
			Handler: s.getCustomData,
		},
		{
			Path:    CustomDataPrefix,
			Method:  http.MethodPost,
			Handler: s.createCustomData,
		},
		{
			Path:    CustomDataPrefix,
			Method:  http.MethodPut,
			Handler: s.updateCustomData,
		},
		{
			Path:    CustomDataPrefix + "/{id}",
			Method:  http.MethodDelete,
			Handler: s.deleteCustomData,
		},

		{
			Path:    CustomDataPrefix + "/items",
			Method:  http.MethodPost,
			Handler: s.batchUpdateCustomData,
		},
	}
}

func (s *Server) listCustomDataKind(w http.ResponseWriter, r *http.Request) {
	result, err := s.cds.ListKinds()
	if err != nil {
		ClusterPanic(err)
	}

	buff := spectool.MustMarshalJSON(result)
	s.writeJSONBody(w, buff)
}

func (s *Server) getCustomDataKind(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	k, err := s.cds.GetKind(name)
	if err != nil {
		ClusterPanic(err)
	}

	buff := spectool.MustMarshalJSON(k)
	s.writeJSONBody(w, buff)
}

func (s *Server) createCustomDataKind(w http.ResponseWriter, r *http.Request) {
	k := customdata.Kind{}
	spectool.MustDecode(r.Body, &k)

	err := s.cds.PutKind(&k, false)
	if err != nil {
		ClusterPanic(err)
	}

	w.WriteHeader(http.StatusCreated)
	location := fmt.Sprintf("%s/%s", r.URL.Path, k.Name)
	w.Header().Set("Location", location)
}

func (s *Server) updateCustomDataKind(w http.ResponseWriter, r *http.Request) {
	k := customdata.Kind{}
	spectool.MustDecode(r.Body, &k)

	err := s.cds.PutKind(&k, true)
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) deleteCustomDataKind(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := s.cds.DeleteKind(name)
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) listCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	result, err := s.cds.ListData(kind)
	if err != nil {
		ClusterPanic(err)
	}

	buff, err := spectool.MarshalJSON(result)
	if err != nil {
		panic(err)
	}

	s.writeJSONBody(w, buff)
}

func (s *Server) getCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	id := chi.URLParam(r, "id")

	data, err := s.cds.GetData(kind, id)
	if err != nil {
		ClusterPanic(err)
	}

	buff, err := spectool.MarshalJSON(data)
	if err != nil {
		panic(err)
	}

	s.writeJSONBody(w, buff)
}

func (s *Server) createCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	data := customdata.Data{}
	spectool.MustDecode(r.Body, &data)

	id, err := s.cds.PutData(kind, data, false)
	if err != nil {
		ClusterPanic(err)
	}

	w.WriteHeader(http.StatusCreated)
	location := fmt.Sprintf("%s/%s", r.URL.Path, id)
	w.Header().Set("Location", location)
}

func (s *Server) updateCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	data := customdata.Data{}
	spectool.MustDecode(r.Body, &data)

	_, err := s.cds.PutData(kind, data, true)
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) deleteCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	id := chi.URLParam(r, "id")
	err := s.cds.DeleteData(kind, id)
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) batchUpdateCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	var cr ChangeRequest
	spectool.MustDecode(r.Body, &cr)

	if cr.Rebuild {
		err := s.cds.DeleteAllData(kind)
		if err != nil {
			ClusterPanic(err)
		}
	}

	err := s.cds.BatchUpdateData(kind, cr.Delete, cr.List)
	if err != nil {
		ClusterPanic(err)
	}
}

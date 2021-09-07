//go:build wasmhost
// +build wasmhost

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
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/filter/wasmhost"
	"go.etcd.io/etcd/client/v3/concurrency"
	"gopkg.in/yaml.v2"
)

func (s *Server) isFilterExist(pipeline, filter, kind string) bool {
	spec := s._getObject(pipeline)
	if spec == nil {
		return false
	}

	rawSpec := spec.RawSpec()
	var filters []interface{}
	if f := rawSpec["filters"]; f != nil {
		filters, _ = f.([]interface{})
	}
	if filters == nil {
		return false
	}

	for i := range filters {
		f, _ := filters[i].(map[interface{}]interface{})
		if f == nil {
			continue
		}

		if n := f["name"]; n == nil || n != filter {
			continue
		}

		if k := f["kind"]; k == nil || k != kind {
			continue
		}

		return true
	}

	return false
}

func (s *Server) wasmReloadCode(w http.ResponseWriter, r *http.Request) {
	key := s.cluster.Layout().WasmCodeEvent()
	value := time.Now().Format(time.RFC3339Nano)
	if e := s.cluster.Put(key, value); e != nil {
		ClusterPanic(e)
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "wasm code reload event posted at: %s\n", value)
}

func (s *Server) wasmListData(w http.ResponseWriter, r *http.Request) {
	pipeline := chi.URLParam(r, "pipeline")
	filter := chi.URLParam(r, "filter")
	if !s.isFilterExist(pipeline, filter, wasmhost.Kind) {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	c := s.cluster
	prefix := c.Layout().WasmDataPrefix(pipeline, filter)
	rawData, e := c.GetPrefix(prefix)
	if e != nil {
		HandleAPIError(w, r, http.StatusInternalServerError, e)
		return
	}

	// remove prefix from key
	data := map[string]string{}
	for k, v := range rawData {
		k = k[len(prefix):]
		data[k] = v
	}

	buf, e := yaml.Marshal(data)
	if e != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", data, e))
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buf)
}

func (s *Server) wasmApplyData(w http.ResponseWriter, r *http.Request) {
	pipeline := chi.URLParam(r, "pipeline")
	filter := chi.URLParam(r, "filter")
	if !s.isFilterExist(pipeline, filter, wasmhost.Kind) {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	body, e := ioutil.ReadAll(r.Body)
	if e != nil {
		HandleAPIError(w, r, http.StatusBadRequest, e)
		return
	}

	data := make(map[string]string)
	if e = yaml.Unmarshal(body, data); e != nil {
		HandleAPIError(w, r, http.StatusBadRequest, e)
		return
	}

	c := s.cluster
	prefix := c.Layout().WasmDataPrefix(pipeline, filter)
	c.STM(func(stm concurrency.STM) error {
		for k, v := range data {
			stm.Put(prefix+k, v)
		}
		return nil
	})
}

func (s *Server) wasmDeleteData(w http.ResponseWriter, r *http.Request) {
	pipeline := chi.URLParam(r, "pipeline")
	filter := chi.URLParam(r, "filter")
	if !s.isFilterExist(pipeline, filter, wasmhost.Kind) {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	c := s.cluster
	prefix := c.Layout().WasmDataPrefix(pipeline, filter)
	c.DeletePrefix(prefix)
}

func appendWasmAPI(s *Server, group *Group) {
	entry := &Entry{
		Path:    "/wasm/code",
		Method:  http.MethodPost,
		Handler: s.wasmReloadCode,
	}
	group.Entries = append(group.Entries, entry)

	entry = &Entry{
		Path:    "/wasm/data/{pipeline}/{filter}",
		Method:  http.MethodGet,
		Handler: s.wasmListData,
	}
	group.Entries = append(group.Entries, entry)

	entry = &Entry{
		Path:    "/wasm/data/{pipeline}/{filter}",
		Method:  http.MethodPut,
		Handler: s.wasmApplyData,
	}
	group.Entries = append(group.Entries, entry)

	entry = &Entry{
		Path:    "/wasm/data/{pipeline}/{filter}",
		Method:  http.MethodDelete,
		Handler: s.wasmDeleteData,
	}
	group.Entries = append(group.Entries, entry)
}

func init() {
	appendAddonAPIs = append(appendAddonAPIs, appendWasmAPI)
}

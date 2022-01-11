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
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.etcd.io/etcd/client/v3/concurrency"
	"gopkg.in/yaml.v2"
)

const (
	// CustomDataPrefix is the object prefix.
	CustomDataPrefix = "/customdata/{kind}"
)

type (
	// CustomData defines the custom data type
	CustomData map[string]interface{}

	// ChangeRequest represents a change request to custom data
	ChangeRequest struct {
		Rebuild bool         `yaml:"rebuild"`
		Delete  []string     `yaml:"delete"`
		List    []CustomData `yaml:"list"`
	}
)

// Key returns the 'key' field of the custom data
func (cd CustomData) Key() string {
	if v, ok := cd["key"].(string); ok {
		return v
	}
	return ""
}

func (s *Server) customDataAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    CustomDataPrefix,
			Method:  http.MethodGet,
			Handler: s.listCustomData,
		},
		{
			Path:    CustomDataPrefix,
			Method:  http.MethodPost,
			Handler: s.updateCustomData,
		},
		{
			Path:    CustomDataPrefix + "/{key}",
			Method:  http.MethodGet,
			Handler: s.getCustomData,
		},
	}
}

func (s *Server) listCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	prefix := s.cluster.Layout().CustomDataPrefix(kind)
	kvs, err := s.cluster.GetRawPrefix(prefix)
	if err != nil {
		ClusterPanic(err)
	}

	result := make([]CustomData, 0, len(kvs))
	for key, kv := range kvs {
		key = key[len(prefix):]
		cd := CustomData{}
		err = yaml.Unmarshal(kv.Value, &cd)
		if err != nil {
			panic(err)
		}
		result = append(result, cd)
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	err = yaml.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

func (s *Server) updateCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	var cr ChangeRequest
	err := yaml.NewDecoder(r.Body).Decode(&cr)
	if err != nil {
		panic(err)
	}

	prefix := s.cluster.Layout().CustomDataPrefix(kind)
	if cr.Rebuild {
		err = s.cluster.DeletePrefix(prefix)
		if err != nil {
			ClusterPanic(err)
		}
	}

	err = s.cluster.STM(func(stm concurrency.STM) error {
		if !cr.Rebuild {
			for _, key := range cr.Delete {
				key = s.cluster.Layout().CustomDataItem(kind, key)
				stm.Del(key)
			}
		}
		for _, v := range cr.List {
			key := v.Key()
			key = s.cluster.Layout().CustomDataItem(kind, key)
			data, err := yaml.Marshal(v)
			if err != nil {
				return err
			}
			stm.Put(key, string(data))
		}
		return nil
	})
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) getCustomData(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	key := chi.URLParam(r, "key")

	key = s.cluster.Layout().CustomDataItem(kind, key)
	kv, err := s.cluster.GetRaw(key)
	if err != nil {
		ClusterPanic(err)
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(kv.Value)
}

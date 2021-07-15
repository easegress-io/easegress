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
	"net/http"
	"time"
)

func (s *Server) postWasmCodeUpdateEvent() string {
	key := s.cluster.Layout().WasmCodeEvent()
	value := time.Now().Format(time.RFC3339Nano)
	if e := s.cluster.Put(key, value); e != nil {
		ClusterPanic(e)
	}
	return value
}

func appendWasmAPI(s *Server, group *APIGroup) {
	entry := &APIEntry{
		Path:   "/wasm/code",
		Method: "POST",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			v := s.postWasmCodeUpdateEvent()
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "wasm code update event posted at: %s\n", v)
		},
	}

	group.Entries = append(group.Entries, entry)
}

func init() {
	appendAddonAPIs = append(appendAddonAPIs, appendWasmAPI)
}

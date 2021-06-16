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

	yaml "gopkg.in/yaml.v2"
)

func aboutText() string {
	return fmt.Sprintf(`Copyright © 2017 - %d MegaEase(https://megaease.com). All rights reserved.
Powered by open-source software: Etcd(https://etcd.io), Apache License 2.0.
`, time.Now().Year())
}

const (
	// APIPrefix is the prefix of api.
	APIPrefix = "/apis/v1"

	lockKey = "/config/lock"

	// ConfigVersionKey is the key of header for config version.
	ConfigVersionKey = "X-Config-Version"
)

func (s *Server) setupAPIs() {
	s.setupListAPIs()
	s.setupMemberAPIs()
	s.setupObjectAPIs()
	s.setupMetadaAPIs()
	s.setupHealthAPIs()
	s.setupAboutAPIs()
}

func (s *Server) setupListAPIs() {
	listAPIs := []*APIEntry{
		{
			Path:    "",
			Method:  "GET",
			Handler: s.listAPIs,
		},
	}

	s.RegisterAPIs(listAPIs)
}

// RegisterAPIs registers APIs.
func (s *Server) RegisterAPIs(apis []*APIEntry) {
	s.apisMutex.Lock()
	defer s.apisMutex.Unlock()

	s.apis = append(s.apis, apis...)

	for _, api := range apis {
		api.Path = APIPrefix + api.Path
		switch api.Method {
		case "GET":
			s.router.Get(api.Path, api.Handler)
		case "HEAD":
			s.router.Head(api.Path, api.Handler)
		case "PUT":
			s.router.Put(api.Path, api.Handler)
		case "POST":
			s.router.Post(api.Path, api.Handler)
		case "PATCH":
			s.router.Patch(api.Path, api.Handler)
		case "DELETE":
			s.router.Delete(api.Path, api.Handler)
		case "CONNECT":
			s.router.Connect(api.Path, api.Handler)
		case "OPTIONS":
			s.router.Options(api.Path, api.Handler)
		case "TRACE":
			s.router.Trace(api.Path, api.Handler)
		}
	}
}

func (s *Server) setupHealthAPIs() {
	healthAPIs := []*APIEntry{
		{
			// https://stackoverflow.com/a/43381061/1705845
			Path:   "/healthz",
			Method: "GET",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("ok"))
			},
		},
	}

	s.RegisterAPIs(healthAPIs)
}

func (s *Server) setupAboutAPIs() {
	aboutAPIs := []*APIEntry{
		{
			Path:   "/about",
			Method: "GET",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte(aboutText()))
			},
		},
	}

	s.RegisterAPIs(aboutAPIs)
}

func (s *Server) listAPIs(w http.ResponseWriter, r *http.Request) {
	s.apisMutex.RLock()
	defer s.apisMutex.RUnlock()

	buff, err := yaml.Marshal(s.apis)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", s.apis, err))
	}
	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buff)
	return
}

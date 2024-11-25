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
	"net/http"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/megaease/easegress/v2/pkg/logger"
)

type (
	dynamicMux struct {
		server *Server
		router atomic.Value

		done      chan struct{}
		closeOnce sync.Once
	}
)

func newDynamicMux(server *Server) *dynamicMux {
	m := &dynamicMux{
		server: server,
		done:   make(chan struct{}),
	}

	m.router.Store(chi.NewRouter())

	go m.run()

	return m
}

func (m *dynamicMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.Load().(*chi.Mux).ServeHTTP(w, r)
}

func (m *dynamicMux) run() {
	for {
		select {
		case <-m.done:
			return
		case <-apisChangeChan:
			m.reloadAPIs()
		}
	}
}

func (m *dynamicMux) reloadAPIs() {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	apiGroups := make([]*Group, 0, len(apis))
	for _, group := range apis {
		apiGroups = append(apiGroups, group)
	}

	sort.Sort(apisByOrder(apiGroups))

	router := chi.NewMux()
	router.Use(middleware.StripSlashes)
	router.Use(m.newAPILogger)
	router.Use(m.newConfigVersionAttacher)
	router.Use(m.newRecoverer)
	if len(m.server.opt.BasicAuth) > 0 {
		router.Use(m.basicAuth("easegress-basic-auth", m.server.opt.BasicAuth))
	}

	// For access from browser.
	cors := cors.New(cors.Options{
		// By default, not allow delete method.
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
	})
	router.Use(cors.Handler)

	for _, apiGroup := range apiGroups {
		for _, api := range apiGroup.Entries {
			pathV1 := APIPrefixV1 + api.Path
			pathV2 := APIPrefixV2 + api.Path

			switch api.Method {
			case "GET":
				router.Get(pathV1, api.Handler)
				router.Get(pathV2, api.Handler)
			case "HEAD":
				router.Head(pathV1, api.Handler)
				router.Head(pathV2, api.Handler)
			case "PUT":
				router.Put(pathV1, api.Handler)
				router.Put(pathV2, api.Handler)
			case "POST":
				router.Post(pathV1, api.Handler)
				router.Post(pathV2, api.Handler)
			case "PATCH":
				router.Patch(pathV1, api.Handler)
				router.Patch(pathV2, api.Handler)
			case "DELETE":
				router.Delete(pathV1, api.Handler)
				router.Delete(pathV2, api.Handler)
			case "CONNECT":
				router.Connect(pathV1, api.Handler)
				router.Connect(pathV2, api.Handler)
			case "OPTIONS":
				router.Options(pathV1, api.Handler)
				router.Options(pathV2, api.Handler)
			case "TRACE":
				router.Trace(pathV1, api.Handler)
				router.Trace(pathV2, api.Handler)
			default:
				logger.Errorf("BUG: group %s unsupported method: %s",
					apiGroup.Group, api.Method)
			}
		}
	}

	m.router.Store(router)
}

func (m *dynamicMux) close() {
	// make sure to close channel only once.
	// when use "signal-upgrade", easegress will start a new process gracefully,
	// which may cause the old process be closed twice.
	// here we use sync.Once to make sure the channel is closed only once.
	// more discussion here: https://github.com/megaease/easegress/issues/1170
	m.closeOnce.Do(func() {
		close(m.done)
	})
}

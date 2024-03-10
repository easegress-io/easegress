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

package worker

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	defaultServerIP = "127.0.0.1"
)

type (
	apiServer struct {
		srv       http.Server
		router    *chi.Mux
		apisMutex sync.RWMutex
		apis      []*apiEntry
	}

	apiEntry struct {
		Path    string           `json:"path"`
		Method  string           `json:"method"`
		Handler http.HandlerFunc `json:"-"`
	}

	apiErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

// newAPIServer creates an initialed API server.
func newAPIServer(port int) *apiServer {
	r := chi.NewRouter()
	addr := fmt.Sprintf("%s:%d", defaultServerIP, port)

	s := &apiServer{
		srv:    http.Server{Addr: addr, Handler: r},
		router: r,
	}

	r.Use(newRecoverer)

	s.addListAPI()

	go func(s *apiServer) {
		logger.Infof("api server running in %d", port)
		s.srv.ListenAndServe()
	}(s)

	return s
}

// Close closes Server.
func (s *apiServer) Close() {
	// Give the server a bit to close connections
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		logger.Errorf("Could not gracefully shutdown the server", zap.Error(err))
	}

	logger.Infof("Server stopped")
}

func (s *apiServer) addListAPI() {
	listAPIs := []*apiEntry{
		{
			Path:    "/",
			Method:  "GET",
			Handler: s.listAPIs,
		},
	}

	s.registerAPIs(listAPIs)
}

func (s *apiServer) listAPIs(w http.ResponseWriter, r *http.Request) {
	s.apisMutex.RLock()
	defer s.apisMutex.RUnlock()

	buff := codectool.MustMarshalJSON(s.apis)

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (s *apiServer) registerAPIs(apis []*apiEntry) {
	s.apisMutex.Lock()
	defer s.apisMutex.Unlock()

	s.apis = append(s.apis, apis...)

	for _, api := range apis {
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

func handleAPIError(w http.ResponseWriter, r *http.Request, code int, err error) {
	body := apiErr{
		Code:    code,
		Message: err.Error(),
	}
	buff := codectool.MustMarshalJSON(body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(buff)
}

func newRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {
				logger.Errorf("recover from %s, err: %v, stack trace:\n%s\n",
					r.URL.Path, rvr, debug.Stack())
				handleAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("%v", rvr))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

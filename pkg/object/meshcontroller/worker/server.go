/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://wwwrk.apache.org/licenses/LICENSE-2.0
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
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
	"go.uber.org/zap"

	"gopkg.in/yaml.v2"
)

const (
	defaultServerIP = "127.0.0.1"
)

type (
	apiServer struct {
		opt       option.Options
		srv       http.Server
		router    *chi.Mux
		apisMutex sync.RWMutex
		apis      []*apiEntry
		port      int
	}

	apiEntry struct {
		Path    string           `yaml:"path"`
		Method  string           `yaml:"method"`
		Handler http.HandlerFunc `yaml:"-"`
	}

	apiErr struct {
		Code    int    `yaml:"code"`
		Message string `yaml:"message"`
	}
)

// NewAPIServer creates a initialed API server.
func NewAPIServer(port int) *apiServer {
	r := chi.NewRouter()
	addr := fmt.Sprintf("%s:%d", defaultServerIP, port)

	s := &apiServer{
		srv:    http.Server{Addr: addr},
		router: r,
	}

	r.Use(newRecoverer)
	s.addListAPI()

	return s
}

// Start runs ListenAndServe on the http.Server with graceful shutdown
func (s *apiServer) Start() {
	logger.Infof("api server running in %s", s.opt.APIAddr)
	defer logger.Sync()

	go func() {
		if err := http.ListenAndServe(s.opt.APIAddr, s.router); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Could not listen on", zap.String("addr", s.opt.APIAddr), zap.Error(err))
		}
	}()

	logger.Infof("Server is ready to handle requests", zap.String("addr", s.opt.APIAddr))
	s.Close()
}

// Close closes Server.
func (s *apiServer) Close() {
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, os.Interrupt)
	sig := <-quit
	logger.Infof("Server is shutting down", zap.String("reason", sig.String()))

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

	buff, err := yaml.Marshal(s.apis)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", s.apis, err))
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buff)
}

func (s *apiServer) registerAPIs(apis []*apiEntry) {
	s.apisMutex.Lock()
	defer s.apisMutex.Unlock()

	s.apis = append(s.apis, apis...)

	for _, api := range apis {
		logger.Infof("api method: %s, path: %s, handler %#v", api.Method, api.Path, api.Handler)
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
	w.WriteHeader(code)
	buff, err := yaml.Marshal(apiErr{
		Code:    code,
		Message: err.Error(),
	})
	if err != nil {
		panic(err)
	}
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

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
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
)

type (
	// Server is the api server.
	Server struct {
		opt       option.Options
		srv       http.Server
		router    *chi.Mux
		cluster   cluster.Cluster
		apisMutex sync.RWMutex
		apis      []*APIEntry

		mutex      cluster.Mutex
		mutexMutex sync.Mutex
	}

	// APIEntry is the entry of API.
	APIEntry struct {
		Path    string           `yaml:"path"`
		Method  string           `yaml:"method"`
		Handler http.HandlerFunc `yaml:"-"`
	}
)

// GlobalServer is the global api server.
var GlobalServer *Server

// MustNewServer creates an api server.
func MustNewServer(opt *option.Options, cluster cluster.Cluster) *Server {
	r := chi.NewRouter()

	s := &Server{
		srv:     http.Server{Addr: opt.APIAddr, Handler: r},
		router:  r,
		cluster: cluster,
	}

	r.Use(middleware.StripSlashes)
	r.Use(s.newAPILogger)
	r.Use(s.newConfigVersionAttacher)
	r.Use(s.newRecoverer)

	_, err := s.getMutex()
	if err != nil {
		logger.Errorf("get cluster mutex %s failed: %v", lockKey, err)
	}

	s.setupAPIs()

	go func() {
		logger.Infof("api server running in %s", opt.APIAddr)
		s.srv.ListenAndServe()
	}()

	GlobalServer = s

	return s
}

// Close closes Server.
func (s *Server) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		logger.Errorf("gracefully shutdown the server failed: %v", err)
	}

	logger.Infof("server stopped")
}

func (s *Server) getMutex() (cluster.Mutex, error) {
	s.mutexMutex.Lock()
	defer s.mutexMutex.Unlock()

	if s.mutex != nil {
		return s.mutex, nil
	}

	mutex, err := s.cluster.Mutex(lockKey)
	if err != nil {
		return nil, err
	}

	s.mutex = mutex

	return s.mutex, nil
}

// Lock locks cluster operations.
func (s *Server) Lock() {
	mutex, err := s.getMutex()
	if err != nil {
		ClusterPanic(err)
	}

	err = mutex.Lock()
	if err != nil {
		ClusterPanic(err)
	}
}

// Unlock unlocks cluster operations.
func (s *Server) Unlock() {
	mutex, err := s.getMutex()
	if err != nil {
		ClusterPanic(err)
	}

	err = mutex.Unlock()
	if err != nil {
		ClusterPanic(err)
	}
}

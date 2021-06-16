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
	"os"
	"os/signal"
	"sync"
	"time"

	chi "github.com/go-chi/chi/v5"
	"go.uber.org/zap"

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

var (
	// GlobalServer is the global api server.
	GlobalServer *Server
)

// MustNewServer creates an api server.
func MustNewServer(opt *option.Options, cluster cluster.Cluster) *Server {
	r := chi.NewRouter()

	s := &Server{
		opt:     *opt,
		srv:     http.Server{Addr: opt.APIAddr},
		router:  r,
		cluster: cluster,
	}

	r.Use(s.newConfigVersionAttacher)
	r.Use(s.newRecoverer)
	r.Use(s.newAPILogger)

	_, err := s.getMutex()
	if err != nil {
		logger.Errorf("get cluster mutex %s failed: %v", lockKey, err)
	}

	s.setupAPIs()

	GlobalServer = s

	s.Start()

	return s
}

// Start runs ListenAndServe on the http.Server with graceful shutdown
func (s *Server) Start() {
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
func (s *Server) Close() {
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

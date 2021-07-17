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

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	// Server is the api server.
	Server struct {
		opt     *option.Options
		server  http.Server
		router  *dynamicMux
		cluster cluster.Cluster
		super   *supervisor.Supervisor

		mutex      cluster.Mutex
		mutexMutex sync.Mutex
	}

	// Group is the API group
	Group struct {
		Group   string
		Entries []*Entry
	}

	// Entry is the entry of API.
	Entry struct {
		Path    string           `yaml:"path"`
		Method  string           `yaml:"method"`
		Handler http.HandlerFunc `yaml:"-"`
	}
)

// MustNewServer creates an api server.
func MustNewServer(opt *option.Options, cluster cluster.Cluster, super *supervisor.Supervisor) *Server {
	s := &Server{
		opt:     opt,
		cluster: cluster,
		super:   super,
	}
	s.router = newDynamicMux(s)
	s.server = http.Server{Addr: opt.APIAddr, Handler: s.router}

	_, err := s.getMutex()
	if err != nil {
		logger.Errorf("get cluster mutex %s failed: %v", lockKey, err)
	}

	s.initMetadata()
	s.registerAPIs()

	go func() {
		logger.Infof("api server running in %s", opt.APIAddr)
		s.server.ListenAndServe()
	}()

	return s
}

// Close closes Server.
func (s *Server) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		logger.Errorf("gracefully shutdown the server failed: %v", err)
	}

	s.router.close()

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

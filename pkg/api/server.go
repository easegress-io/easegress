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
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/cluster/customdata"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	pprof "github.com/megaease/easegress/v2/pkg/profile"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

type (
	// Server is the api server.
	Server struct {
		opt     *option.Options
		server  http.Server
		router  *dynamicMux
		cluster cluster.Cluster
		super   *supervisor.Supervisor
		cds     *customdata.Store
		profile pprof.Profile

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
		Path    string           `json:"path"`
		Method  string           `json:"method"`
		Handler http.HandlerFunc `json:"-"`
	}
)

// MustNewServer creates an api server.
func MustNewServer(opt *option.Options, cls cluster.Cluster, super *supervisor.Supervisor, profile pprof.Profile) *Server {
	s := &Server{
		opt:     opt,
		cluster: cls,
		super:   super,
		profile: profile,
	}
	s.router = newDynamicMux(s)
	s.server = http.Server{Addr: opt.APIAddr, Handler: s.router}

	if opt.ClientCAFile != "" {
		caCert, err := os.ReadFile(opt.ClientCAFile)
		if err != nil {
			logger.Errorf("read client CA file %s failed: %v", opt.ClientCAFile, err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			logger.Errorf("Failed to append CA certificate to pool")
		}
		s.server.TLSConfig = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  caCertPool,
		}
	}

	_, err := s.getMutex()
	if err != nil {
		logger.Errorf("get cluster mutex %s failed: %v", lockKey, err)
	}

	kindPrefix := cls.Layout().CustomDataKindPrefix()
	dataPrefix := cls.Layout().CustomDataPrefix()
	s.cds = customdata.NewStore(cls, kindPrefix, dataPrefix)

	s.registerAPIs()

	go func() {
		var err error
		if s.opt.TLS {
			logger.Infof("api server (https) running in %s", opt.APIAddr)
			err = s.server.ListenAndServeTLS(s.opt.CertFile, s.opt.KeyFile)
		} else {
			logger.Infof("api server running in %s", opt.APIAddr)
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Errorf("start api server failed: %v", err)
		}
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

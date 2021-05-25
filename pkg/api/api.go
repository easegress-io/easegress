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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	"github.com/kataras/iris"
	yaml "gopkg.in/yaml.v2"
)

func aboutText() string {
	return fmt.Sprintf(`Copyright © 2017 - %d MegaEase(https://megaease.com). All rights reserved.
Powered by open-source software: Etcd(https://etcd.io), Apache License 2.0.
`, time.Now().Year())
}

const (
	// APIPrefix is the prefix of api.
	APIPrefix = "/apis/v3"

	lockKey = "/config/lock"

	// ConfigVersionKey is the key of header for config version.
	ConfigVersionKey = "X-Config-Version"
)

type (
	// Server is the api server.
	Server struct {
		app       *iris.Application
		cluster   cluster.Cluster
		apisMutex sync.RWMutex
		apis      []*APIEntry

		mutex      cluster.Mutex
		mutexMutex sync.Mutex
	}

	// APIEntry is the entry of API.
	APIEntry struct {
		Path    string       `yaml:"path"`
		Method  string       `yaml:"method"`
		Handler iris.Handler `yaml:"-"`
	}
)

var (
	// GlobalServer is the global api server.
	GlobalServer *Server
)

// MustNewServer creates an api server.
func MustNewServer(opt *option.Options, cluster cluster.Cluster) *Server {
	app := iris.New()

	s := &Server{
		app:     app,
		cluster: cluster,
	}

	// NOTE: Fix trailing slash problem.
	// Reference: https://github.com/kataras/iris/issues/820#issuecomment-383131098
	app.WrapRouter(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		path := r.URL.Path
		if len(path) > 1 && path[len(path)-1] == '/' && path[len(path)-2] != '/' {
			path = path[:len(path)-1]
			r.RequestURI = path
			r.URL.Path = path
		}
		next(w, r)
	})

	app.Use(newConfigVersionAttacher(s))
	app.Use(newRecoverer())
	app.Use(newAPILogger())

	app.Logger().SetOutput(ioutil.Discard)

	_, err := s.getMutex()
	if err != nil {
		logger.Errorf("get cluster mutex %s failed: %v", lockKey, err)
	}

	s.setupAPIs()

	go func() {
		logger.Infof("api server running in %s", opt.APIAddr)

		err := app.Run(iris.Addr(opt.APIAddr))
		if err == iris.ErrServerClosed {
			return
		}
		if err != nil {
			logger.Errorf("run api app failed: %v", err)
			os.Exit(1)
		}
	}()

	GlobalServer = s

	return s
}

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
			s.app.Get(api.Path, api.Handler)
		case "HEAD":
			s.app.Head(api.Path, api.Handler)
		case "PUT":
			s.app.Put(api.Path, api.Handler)
		case "POST":
			s.app.Post(api.Path, api.Handler)
		case "PATCH":
			s.app.Patch(api.Path, api.Handler)
		case "DELETE":
			s.app.Delete(api.Path, api.Handler)
		case "CONNECT":
			s.app.Connect(api.Path, api.Handler)
		case "OPTIONS":
			s.app.Options(api.Path, api.Handler)
		case "TRACE":
			s.app.Trace(api.Path, api.Handler)
		}

	}

	s.app.RefreshRouter()
}

func (s *Server) setupHealthAPIs() {
	healthAPIs := []*APIEntry{
		{
			// https://stackoverflow.com/a/43381061/1705845
			Path:    "/healthz",
			Method:  "GET",
			Handler: func(iris.Context) { /* 200 by default */ },
		},
	}

	s.RegisterAPIs(healthAPIs)
}

func (s *Server) setupAboutAPIs() {
	aboutAPIs := []*APIEntry{
		{
			Path:   "/about",
			Method: "GET",
			Handler: func(ctx iris.Context) {
				ctx.Header("Content-Type", "text/plain")
				ctx.WriteString(aboutText())
			},
		},
	}

	s.RegisterAPIs(aboutAPIs)
}

func (s *Server) listAPIs(ctx iris.Context) {
	s.apisMutex.RLock()
	defer s.apisMutex.RUnlock()

	buff, err := yaml.Marshal(s.apis)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", s.apis, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

// Close closes Server.
func (s *Server) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	s.app.Shutdown(context.Background())
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

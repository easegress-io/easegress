package api

import (
	"context"
	"fmt"
	"io/ioutil"
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

type apiEntry struct {
	Path    string       `yaml:"path"`
	Method  string       `yaml:"method"`
	Handler iris.Handler `yaml:"-"`
}

const (
	// APIPrefix is the prefix of api.
	APIPrefix = "/apis/v3"

	lockKey = "/config/lock"
)

// Server is the api server.
type Server struct {
	app     *iris.Application
	cluster cluster.Cluster
	apis    []*apiEntry

	mutex      cluster.Mutex
	mutexMutex sync.Mutex
}

// MustNewServer creates an api server.
func MustNewServer(opt *option.Options, cluster cluster.Cluster) *Server {
	app := iris.New()
	app.Use(newRecoverer())
	app.Use(newAPILogger())

	app.Logger().SetOutput(ioutil.Discard)

	s := &Server{
		app:     app,
		cluster: cluster,
	}

	_, err := s.getMutex()
	if err != nil {
		logger.Errorf("get cluster mutex %s failed: %v", lockKey, err)
	}

	s.setupAPIs()

	go func() {
		err := app.Run(iris.Addr(opt.APIAddr))
		if err == iris.ErrServerClosed {
			return
		}
		if err != nil {
			logger.Errorf("run api app failed: %v", err)
			os.Exit(1)
		}
	}()

	return s
}

func (s *Server) setupAPIs() {
	listAPIsEntry := &apiEntry{
		Path:    "",
		Method:  "GET",
		Handler: s.listAPIs,
	}

	s.apis = append(s.apis, listAPIsEntry)
	s.setupMemberAPIs()
	s.setupObjectAPIs()
	s.setupMetadaAPIs()
	s.setupHealthAPIs()
	s.setupAboutAPIs()

	for _, api := range s.apis {
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
}

func (s *Server) setupHealthAPIs() {
	s.apis = append(s.apis, &apiEntry{
		// https://stackoverflow.com/a/43381061/1705845
		Path:    "/healthz",
		Method:  "GET",
		Handler: func(iris.Context) { /* 200 by default */ },
	})
}

func (s *Server) setupAboutAPIs() {
	s.apis = append(s.apis, &apiEntry{
		Path:   "/about",
		Method: "GET",
		Handler: func(ctx iris.Context) {
			ctx.Header("Content-Type", "text/plain")
			ctx.WriteString(aboutText())
		},
	})
}

func (s *Server) listAPIs(ctx iris.Context) {
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
		clusterPanic(err)
	}

	err = mutex.Lock()
	if err != nil {
		clusterPanic(err)
	}
}

// Unlock unlocks cluster operations.
func (s *Server) Unlock() {
	mutex, err := s.getMutex()
	if err != nil {
		clusterPanic(err)
	}

	err = mutex.Unlock()
	if err != nil {
		clusterPanic(err)
	}
}

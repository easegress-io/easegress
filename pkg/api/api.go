package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	"github.com/kataras/iris"
)

type apiEntry struct {
	Path    string       `json:"path"`
	Method  string       `json:"method"`
	Handler iris.Handler `json:"-"`
}

const (
	APIPrefix = "/apis/v2"

	lockKey     = "/config/lock"
	lockTimeout = 10 * time.Second
)

type APIServer struct {
	app     *iris.Application
	cluster cluster.Cluster
	mutex   cluster.Mutex
	apis    []*apiEntry
}

func MustNewAPIServer(cluster cluster.Cluster) *APIServer {
	app := iris.New()
	app.Use(newJSONContentType())
	app.Use(newRecoverer())
	app.Use(newAPILogger())

	app.Logger().SetOutput(ioutil.Discard)

	s := &APIServer{
		app:     app,
		cluster: cluster,
		mutex:   cluster.Mutex(lockKey, lockTimeout),
	}

	s.setupAPIs()

	go func() {
		err := app.Run(iris.Addr(option.Global.APIAddr))
		if err == iris.ErrServerClosed {
			return
		}
		if err != nil {
			logger.Errorf("[run api app failed: %v]", err)
			os.Exit(1)
		}
	}()

	return s
}

func (s *APIServer) setupAPIs() {
	listAPIsEntry := &apiEntry{
		Path:    "",
		Method:  "GET",
		Handler: s.listAPIs,
	}
	s.apis = append(s.apis, listAPIsEntry)
	s.setupMemberAPIs()
	s.setupPluginAPIs()
	s.setupPipelineAPIs()
	s.setupStatAPIs()

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

func (s *APIServer) listAPIs(ctx iris.Context) {
	buff, err := json.Marshal(s.apis)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", s.apis, err))
	}

	ctx.Write(buff)
}

func (s *APIServer) Close() {
	s.app.Shutdown(context.Background())
}

func (s *APIServer) Lock() {
	err := s.mutex.Lock()
	if err != nil {
		clusterPanic(err)
	}
}

func (s *APIServer) Unlock() {
	err := s.mutex.Unlock()
	if err != nil {
		clusterPanic(err)
	}
}

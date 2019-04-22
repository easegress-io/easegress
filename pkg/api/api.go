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

type apiEntry struct {
	Path    string       `yaml:"path"`
	Method  string       `yaml:"method"`
	Handler iris.Handler `yaml:"-"`
}

const (
	// APIPrefix is the prefix of api.
	APIPrefix = "/apis/v3"

	lockKey     = "/config/lock"
	lockTimeout = 10 * time.Second
)

// Server is the api server.
type Server struct {
	app     *iris.Application
	cluster cluster.Cluster
	mutex   cluster.Mutex
	apis    []*apiEntry
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
		mutex:   cluster.Mutex(lockKey, lockTimeout),
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

func (s *Server) listAPIs(ctx iris.Context) {
	buff, err := yaml.Marshal(s.apis)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", s.apis, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (s *Server) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	s.app.Shutdown(context.Background())
}

func (s *Server) Lock() {
	err := s.mutex.Lock()
	if err != nil {
		clusterPanic(err)
	}
}

func (s *Server) Unlock() {
	err := s.mutex.Unlock()
	if err != nil {
		clusterPanic(err)
	}
}

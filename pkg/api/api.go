package api

import (
	"github.com/megaease/easegateway/pkg/store"

	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/recover"
)

type apiEntry struct {
	path    string
	method  string
	handler iris.Handler
}

const (
	APIPrefix = "/apis/v2/groups/{group:string}"
)

type APIServer struct {
	app   *iris.Application
	store store.Store
	apis  []apiEntry
}

func NewAPIServer(store store.Store) *APIServer {
	app := iris.New()
	app.Use(recover.New())
	app.Logger()
	app.Use(newAPILogger())
	// app.Use(newRedirector(cluster))

	s := &APIServer{
		app:   app,
		store: store,
	}
	s.apis = append(s.apis, pluginAPIs(s)...)
	s.apis = append(s.apis, pipelineAPIs(s)...)

	return s
}

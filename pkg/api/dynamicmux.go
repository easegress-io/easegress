package api

import (
	"net/http"
	"sort"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/megaease/easegress/pkg/logger"
)

type (
	dynamicMux struct {
		server *Server
		done   chan struct{}
		router atomic.Value
	}
)

func newDynamicMux(server *Server) *dynamicMux {
	m := &dynamicMux{
		server: server,
		done:   make(chan struct{}),
	}

	m.router.Store(chi.NewRouter())

	go m.run()

	return m
}

func (m *dynamicMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.Load().(*chi.Mux).ServeHTTP(w, r)
}

func (m *dynamicMux) run() {
	for {
		select {
		case <-m.done:
			return
		case <-apisChangeChan:
			m.reloadAPIs()
		}
	}
}

func (m *dynamicMux) reloadAPIs() {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	apiGroups := []*Group{}

	for _, group := range apis {
		apiGroups = append(apiGroups, group)
	}

	sort.Sort(apisbyOrder(apiGroups))

	router := chi.NewMux()
	router.Use(middleware.StripSlashes)
	router.Use(m.newAPILogger)
	router.Use(m.newConfigVersionAttacher)
	router.Use(m.newRecoverer)

	for _, apiGroup := range apiGroups {
		for _, api := range apiGroup.Entries {
			path := APIPrefix + api.Path

			switch api.Method {
			case "GET":
				router.Get(path, api.Handler)
			case "HEAD":
				router.Head(path, api.Handler)
			case "PUT":
				router.Put(path, api.Handler)
			case "POST":
				router.Post(path, api.Handler)
			case "PATCH":
				router.Patch(path, api.Handler)
			case "DELETE":
				router.Delete(path, api.Handler)
			case "CONNECT":
				router.Connect(path, api.Handler)
			case "OPTIONS":
				router.Options(path, api.Handler)
			case "TRACE":
				router.Trace(path, api.Handler)
			default:
				logger.Errorf("BUG: group %s unsupported method: %s",
					apiGroup.Group, api.Method)
			}
		}
	}

	m.router.Store(router)
}

func (m *dynamicMux) close() {
	close(m.done)
}

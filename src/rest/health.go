package rest

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"

	"common"
	"engine"
	"logger"
)

type healthCheckServer struct {
	gateway *engine.Gateway
}

func newHealthCheckServer(gateway *engine.Gateway) (*healthCheckServer, error) {
	return &healthCheckServer{
		gateway: gateway,
	}, nil
}

func (s *healthCheckServer) Api() (*rest.Api, error) {
	router, err := rest.MakeRouter(
		rest.Get(common.PrefixAPIVersion("/health"), s.existing), // keep backward-compatibility
	)

	if err != nil {
		logger.Errorf("[make router for healthcheck server failed: %s]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(rest.DefaultCommonStack...)
	api.SetApp(router)

	return api, nil
}

func (s *healthCheckServer) existing(w rest.ResponseWriter, req *rest.Request) {
	logger.Debugf("[check health]")
	w.WriteHeader(http.StatusOK)
	logger.Debugf("[healthy status returned]")
}

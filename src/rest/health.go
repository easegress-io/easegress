package rest

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"

	"common"
	"engine"
	"logger"
	"version"
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
		rest.Get(common.PrefixAPIVersion("/check"), s.existing),
		rest.Get(common.PrefixAPIVersion("/info"), s.info),
	)

	if err != nil {
		logger.Errorf("[make router for healthcheck server failed: %v]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(restStack...)
	api.SetApp(router)

	return api, nil
}

func (s *healthCheckServer) existing(w rest.ResponseWriter, req *rest.Request) {
	logger.Debugf("[check existing]")
	w.WriteHeader(http.StatusOK)
	logger.Debugf("[existing status returned]")
}

func (s *healthCheckServer) info(w rest.ResponseWriter, req *rest.Request) {
	logger.Debugf("[get health info]")

	w.WriteJson(healthInfoResponse{
		Build: buildInfo{
			Name:       "Ease Gateway",
			Release:    version.RELEASE,
			Build:      version.COMMIT,
			Repository: version.REPO,
		},
	})

	logger.Debugf("[get health info returned]")
}

package rest

import (
	"fmt"
	"net/http"

	"engine"
	"logger"
)

type Rest struct {
	gateway *engine.Gateway
	done    chan error
}

func NewReset(gateway *engine.Gateway) (*Rest, error) {
	if gateway == nil {
		return nil, fmt.Errorf("gateway engine is nil")
	}

	return &Rest{
		gateway: gateway,
		done:    make(chan error, 1),
	}, nil
}

func (s *Rest) Start() (<-chan error, string, error) {
	adminServer, err := newAdminServer(s.gateway)
	if err != nil {
		logger.Errorf("[create admin rest server failed: %v", err)
		return nil, "", err
	}
	statisticsServer, err := newStatisticsServer(s.gateway)
	if err != nil {
		logger.Errorf("[create statistics rest server failed: %v", err)
		return nil, "", err
	}
	healthCheckServer, err := newHealthCheckServer(s.gateway)
	if err != nil {
		logger.Errorf("[create healthcheck rest server failed: %v", err)
		return nil, "", err
	}

	adminApi, err := adminServer.Api()
	if err != nil {
		logger.Errorf("[create admin api failed: %v", err)
		return nil, "", err
	} else {
		logger.Debugf("[admin api created]")
	}
	statisticsApi, err := statisticsServer.Api()
	if err != nil {
		logger.Errorf("[create statistics api failed: %v", err)
		return nil, "", err
	} else {
		logger.Debugf("[statistics api created]")
	}
	healthCheckApi, err := healthCheckServer.Api()
	if err != nil {
		logger.Errorf("[create healthcheck api failed: %v", err)
		return nil, "", err
	} else {
		logger.Debugf("[healthcheck api created]")
	}

	http.Handle("/admin/", http.StripPrefix("/admin", adminApi.MakeHandler()))
	http.Handle("/statistics/", http.StripPrefix("/statistics", statisticsApi.MakeHandler()))
	http.Handle("/", healthCheckApi.MakeHandler()) // keep backward-compatibility

	listenAddr := fmt.Sprintf("0.0.0.0:9090")

	go func() {
		err := http.ListenAndServe(listenAddr, nil)
		if err != nil {
			s.done <- err
		}
	}()

	return s.done, listenAddr, nil
}

func (s *Rest) Close() {
	close(s.done)
}

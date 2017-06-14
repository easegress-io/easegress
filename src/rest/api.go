package rest

import (
	"common"
	"fmt"
	"net/http"

	"cluster/gateway"
	"engine"
	"logger"
)

type Rest struct {
	gateway *engine.Gateway
	gc      *gateway.GatewayCluster
	done    chan error
}

func NewRest(gateway *engine.Gateway) (*Rest, error) {
	if gateway == nil {
		return nil, fmt.Errorf("gateway engine is nil")
	}

	return &Rest{
		gateway: gateway,
		gc:      gateway.GatewayCluster(),
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
	clusterAdminServer, err := newClusterAdminServer(s.gateway, s.gc)
	if err != nil {
		logger.Errorf("[create cluster admin rest server failed: %v", err)
		return nil, "", err
	}
	clusterStatisticsServer, err := newClusterStatisticsServer(s.gateway, s.gc)
	if err != nil {
		logger.Errorf("[create cluster statistics rest server failed: %v", err)
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
	clusterAdminApi, err := clusterAdminServer.Api()
	if err != nil {
		logger.Errorf("[create cluster admin api failed: %v", err)
		return nil, "", err
	} else {
		logger.Debugf("[cluster admin api created]")
	}
	clusterStatisticsApi, err := clusterStatisticsServer.Api()
	if err != nil {
		logger.Errorf("[create cluster statistics api failed: %v", err)
		return nil, "", err
	} else {
		logger.Debugf("[cluster statistics api created]")
	}

	http.Handle("/admin/", http.StripPrefix("/admin", adminApi.MakeHandler()))
	http.Handle("/statistics/", http.StripPrefix("/statistics", statisticsApi.MakeHandler()))
	http.Handle("/health/", http.StripPrefix("/health", healthCheckApi.MakeHandler()))
	http.Handle("/cluster/admin/", http.StripPrefix("/cluster/admin", clusterAdminApi.MakeHandler()))
	http.Handle("/cluster/statistics/", http.StripPrefix("/cluster/statistics", clusterStatisticsApi.MakeHandler()))

	listenAddr := fmt.Sprintf("%s:9090", common.Host)

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

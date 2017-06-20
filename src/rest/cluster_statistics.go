package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cluster/gateway"
	"common"
	"engine"
	"logger"

	"github.com/ant0ine/go-json-rest/rest"
)

const (
	STAT_TIMEOUT_DECAY_RATE = 0.8
)

type clusterStatisticsServer struct {
	gateway *engine.Gateway
	gc      *gateway.GatewayCluster
}

func newClusterStatisticsServer(gateway *engine.Gateway, gc *gateway.GatewayCluster) (*clusterStatisticsServer, error) {
	return &clusterStatisticsServer{
		gateway: gateway,
		gc:      gc,
	}, nil
}

func (s *clusterStatisticsServer) Api() (*rest.Api, error) {
	pav := common.PrefixAPIVersion
	router, err := rest.MakeRouter(
		rest.Get(pav("/#group/pipelines/#pipelineName/plugins/#pluginName/indicators"),
			s.retrievePluginIndicatorNames),
		rest.Get(pav("/#group/pipelines/#pipelineName/plugins/#pluginName/indicators/#indicatorName/value"),
			s.retrievePluginIndicatorValue),
		rest.Get(pav("/#group/pipelines/#pipelineName/plugins/#pluginName/indicators/#indicatorName/desc"),
			s.retrievePluginIndicatorDesc),

		rest.Get(pav("/#group/pipelines/#pipelineName/indicators"),
			s.retrievePipelineIndicatorNames),
		rest.Get(pav("/#group/pipelines/#pipelineName/indicators/#indicatorName/value"),
			s.retrievePipelineIndicatorValue),
		rest.Get(pav("/#group/pipelines/#pipelineName/indicators/#indicatorName/desc"),
			s.retrievePipelineIndicatorDesc),

		rest.Get(pav("/#group/pipelines/#pipelineName/task/indicators"),
			s.retrievePipelineTaskIndicatorNames),
		rest.Get(pav("/#group/pipelines/#pipelineName/task/indicators/#indicatorName/value"),
			s.retrievePipelineTaskIndicatorValue),
		rest.Get(pav("/#group/pipelines/#pipelineName/task/indicators/#indicatorName/desc"),
			s.retrievePipelineTaskIndicatorDesc),

		rest.Get(pav("/#group/gateway/uptime"), s.retrieveGatewayUpTime),
		rest.Get(pav("/#group/gateway/rusage"), s.retrieveGatewaySysResUsage),
		rest.Get(pav("/#group/gateway/loadavg"), s.retrieveGatewaySysAverageLoad),
	)

	if err != nil {
		logger.Errorf("[make router for cluster staticstics server failed: %v]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(rest.DefaultCommonStack...)
	api.SetApp(router)

	return api, nil
}

func (s *clusterStatisticsServer) retrievePluginIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator names from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil || len(pluginName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid plugin name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	req := new(clusterStat)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = 30
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout %d should greater than or equal to 10 senconds", req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPluginIndicatorNames(group, time.Duration(req.TimeoutSec)*time.Second, pipelineName, pluginName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[retrive pipeline %s plugin %s indicator names succeed]", pipelineName, pluginName)
}

func (s *clusterStatisticsServer) retrievePluginIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator value from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil || len(pluginName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid plugin name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid indicator name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	req := new(clusterStat)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = 30
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout %d should greater than or equal to 10 senconds", req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPluginIndicatorValue(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, pluginName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[retrive pipeline %s plugin %s indicator %s value succeed]", pipelineName, pluginName, indicatorName)
}

func (s *clusterStatisticsServer) retrievePluginIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator desc from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil || len(pluginName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid plugin name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid indicator name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	req := new(clusterStat)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = 30
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout %d should greater than or equal to 10 senconds", req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPluginIndicatorDesc(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, pluginName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[retrive pipeline %s plugin %s indicator %s desc succeed]", pipelineName, pluginName, indicatorName)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
}

func (s *clusterStatisticsServer) retrieveGatewayUpTime(w rest.ResponseWriter, r *rest.Request) {
	// TODO
}

func (s *clusterStatisticsServer) retrieveGatewaySysResUsage(w rest.ResponseWriter, r *rest.Request) {
	// TODO
}

func (s *clusterStatisticsServer) retrieveGatewaySysAverageLoad(w rest.ResponseWriter, r *rest.Request) {
	// TODO
}

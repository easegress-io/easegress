package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ant0ine/go-json-rest/rest"

	"cluster/gateway"
	"common"
	"engine"
	"logger"
	"option"
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
		rest.Get(pav("/#group/pipelines/#pipelineName/indicators/value"),
			s.retrievePipelineIndicatorsValue),

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
		logger.Errorf("[make router for cluster statistics server failed: %v]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(append(RestStack, &clusterAvailabilityMiddleware{gc: s.gc})...)
	api.SetApp(router)

	return api, nil
}

func (s *clusterStatisticsServer) retrievePluginIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator names from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil || len(pluginName) == 0 {
		msg := "invalid plugin name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPluginIndicatorNames(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, pluginName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator names of plugin %s in pipeline %s returned from cluster]", pluginName, pipelineName)
}

func (s *clusterStatisticsServer) retrievePluginIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator value from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil || len(pluginName) == 0 {
		msg := "invalid plugin name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := "invalid indicator name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPluginIndicatorValue(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, pluginName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator %s value of plugin %s in pipeline %s returned from cluster]",
		indicatorName, pluginName, pipelineName)
}

func (s *clusterStatisticsServer) retrievePluginIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator desc from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil || len(pluginName) == 0 {
		msg := "invalid plugin name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := "invalid indicator name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPluginIndicatorDesc(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, pluginName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator %s description of plugin %s in pipeline %s returned from cluster]",
		indicatorName, pluginName, pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline indicator names from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPipelineIndicatorNames(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator names of pipeline %s returned from cluster]", pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline indicator value from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := "invalid indicator name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPipelineIndicatorValue(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator %s value of pipeline %s returned from cluster]", indicatorName, pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline indicator desc from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", err)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", err)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := "invalid indicator name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", err)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPipelineIndicatorDesc(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator %s description of pipeline %s returned from cluster]", indicatorName, pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorsValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline statistics values from multiple indicators from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(indicatorsValueRetrieveClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatPipelineIndicatorsValue(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, req.IndicatorNames)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[statistics values of multiple indicators of pipeline %s returned from cluster]", pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline task indicator names from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatTaskIndicatorNames(group, time.Duration(req.TimeoutSec)*time.Second, pipelineName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator names of task in pipeline %s returned from cluster]", pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline task indicator value from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := "invalid indicator name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatTaskIndicatorValue(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator %s value of task in pipeline %s returned]", indicatorName, pipelineName)
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline task indicator desc from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := "invalid pipeline name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := "invalid indicator name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	req := new(statisticsClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		msg := fmt.Sprintf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	ret, clusterErr := s.gc.StatTaskIndicatorDesc(group, time.Duration(req.TimeoutSec)*time.Second,
		pipelineName, indicatorName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Warnf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret)

	logger.Debugf("[indicator %s description of task in pipeline %s returned]", indicatorName, pipelineName)
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

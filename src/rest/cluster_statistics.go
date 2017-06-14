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
		// parameters: timeout(seconds, min: 10s, default:30s),
		// e.g. /cluster/statistics/v1/group_NY/plugins/plugin_ex/indicators?timeout=30s
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
	logger.Debugf("[retrieve plugin indicator names]")

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

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatPluginIndicatorNames(group, timeout, pipelineName, pluginName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s plugin %s indicator names succeed: %s]", pipelineName, pluginName, resp)
}

func (s *clusterStatisticsServer) retrievePluginIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator value]")

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

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatPluginIndicatorValue(group, timeout, pipelineName, pluginName, indicatorName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s plugin %s indicator %s value succeed: %s]", pipelineName, pluginName, indicatorName, resp)
}

func (s *clusterStatisticsServer) retrievePluginIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin indicator desc]")

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

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatPluginIndicatorDesc(group, timeout, pipelineName, pluginName, indicatorName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s plugin %s indicator %s desc succeed: %s]", pipelineName, pluginName, indicatorName, resp)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline indicator names]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatPipelineIndicatorNames(group, timeout, pipelineName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s indicator names succeed: %s]", pipelineName, resp)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline indicator value]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid indicator name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatPipelineIndicatorValue(group, timeout, pipelineName, indicatorName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s indicator %s value succeed: %s]", pipelineName, indicatorName, resp)
}

func (s *clusterStatisticsServer) retrievePipelineIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline indicator desc]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid indicator name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatPipelineIndicatorDesc(group, timeout, pipelineName, indicatorName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s indicator %s desc succeed: %s]", pipelineName, indicatorName, resp)
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorNames(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve task indicator names]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatTaskIndicatorNames(group, timeout, pipelineName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s task indicator names succeed: %s]", pipelineName, resp)
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorValue(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve task indicator value]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid indicator name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatTaskIndicatorValue(group, timeout, pipelineName, indicatorName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s task indicator %s value succeed: %s]", pipelineName, indicatorName, resp)
}

func (s *clusterStatisticsServer) retrievePipelineTaskIndicatorDesc(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve task indicator desc]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	indicatorName, err := url.QueryUnescape(r.PathParam("indicatorName"))
	if err != nil || len(indicatorName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid indicator name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, _, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(STAT_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.StatTaskIndicatorDesc(group, timeout, pipelineName, indicatorName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s task indicator %s desc succeed: %s]", pipelineName, indicatorName, resp)
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

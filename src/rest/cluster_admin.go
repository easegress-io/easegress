package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ant0ine/go-json-rest/rest"

	"cluster/gateway"
	"common"
	"engine"
	"logger"
)

type clusterAdminServer struct {
	gateway *engine.Gateway
	gc      *gateway.GatewayCluster
}

func newClusterAdminServer(gateway *engine.Gateway, gc *gateway.GatewayCluster) (*clusterAdminServer, error) {
	return &clusterAdminServer{
		gateway: gateway,
		gc:      gc,
	}, nil
}

func (s *clusterAdminServer) Api() (*rest.Api, error) {
	pav := common.PrefixAPIVersion
	router, err := rest.MakeRouter(
		rest.Post(pav("/#group/plugins"), s.createPlugin),
		rest.Get(pav("/#group/plugins"), s.retrievePlugins),
		rest.Get(pav("/#group/plugins/#pluginName"), s.retrievePlugin),
		rest.Put(pav("/#group/plugins"), s.updatePlugin),
		rest.Delete(pav("/#group/plugins/#pluginName"), s.deletePlugin),

		rest.Post(pav("/#group/pipelines"), s.createPipeline),
		rest.Get(pav("/#group/pipelines"), s.retrievePipelines),
		rest.Get(pav("/#group/pipelines/#pipelineName"), s.retrievePipeline),
		rest.Put(pav("/#group/pipelines"), s.updatePipeline),
		rest.Delete(pav("/#group/pipelines/#pipelineName"), s.deletePipeline),

		rest.Get(pav("/#group/plugin-types"), s.retrievePluginTypes),
		rest.Get(pav("/#group/pipeline-types"), s.retrievePipelineTypes),
	)

	if err != nil {
		logger.Errorf("[make router for cluster admin server failed: %v]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(rest.DefaultCommonStack...)
	api.SetApp(router)

	return api, nil
}

func (s *clusterAdminServer) createPlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[create plugin in cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	req := new(pluginCreationClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req == nil || req.Config == nil {
		msg := "invalid request"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	conf, err := json.Marshal(req.Config)
	if err != nil {
		msg := "plugin config is invalid"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
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

	clusterErr := s.gc.CreatePlugin(group, req.TimeoutSec*time.Second, req.OperationSeqSnapshot,
		req.Consistent, req.Type, conf)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("plugin created in cluster")
}

func (s *clusterAdminServer) retrievePlugins(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugins from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	req := new(pluginsRetrieveClusterRequest)
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

	ret, clusterErr := s.gc.RetrievePlugins(group, req.TimeoutSec*time.Second, req.Consistent,
		req.NamePattern, req.Types)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret.Plugins)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugins returned from cluster]")
}

func (s *clusterAdminServer) retrievePlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pluginRetrieveClusterRequest)
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

	ret, clusterErr := s.gc.RetrievePlugin(group, req.TimeoutSec*time.Second, req.Consistent, pluginName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	if ret.Plugin == nil {
		msg := fmt.Sprintf("plugin %s not found", pluginName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	w.WriteJson(ret.Plugin)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin %s returned from cluster]", pluginName)
}

func (s *clusterAdminServer) updatePlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[update plugin in cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pluginUpdateClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	conf, err := json.Marshal(req.Config)
	if err != nil {
		msg := "plugin config is invalid"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
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

	clusterErr := s.gc.UpdatePlugin(group, req.TimeoutSec*time.Second, req.OperationSeqSnapshot, req.Consistent,
		req.Type, conf)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("plugin updated in cluster")
}

func (s *clusterAdminServer) deletePlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[delete plugin from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}
	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pluginDeletionClusterRequest)
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

	clusterErr := s.gc.DeletePlugin(group, req.TimeoutSec*time.Second, req.OperationSeqSnapshot, req.Consistent,
		pluginName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin %s deleted from cluster]", pluginName)
}

func (s *clusterAdminServer) createPipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[create pipeline]")

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	req := new(pipelineCreationRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	if len(req.Type) == 0 || req.Config == nil {
		msg := fmt.Sprintf("invalid request: need both type and config")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	conf, err := json.Marshal(req.Config)
	if err != nil {
		msg := fmt.Sprintf("invalid request: bad config: %v", err)
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	httpErr := s.gc.CreatePipeline(group, syncAll, timeout, req.Type, conf)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("create pipeline succeed: %s: %s", req.Type, conf)
}

func (s *clusterAdminServer) retrievePipelines(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipelines]")

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	req := new(pipelinesRetrieveRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.RetrievePipelines(group, syncAll, timeout, req.NamePattern, req.Types)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipelines name-pattern(%s) types(%s) succeed: %s]", req.NamePattern, req.Types, resp)
}

func (s *clusterAdminServer) retrievePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.RetrievePipelines(group, syncAll, timeout, pipelineName, nil)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline %s succeed: %s]", pipelineName, resp)
}

func (s *clusterAdminServer) updatePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[update pipeline]")

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	req := new(pipelineUpdateRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	if len(req.Type) == 0 || req.Config == nil {
		msg := fmt.Sprintf("invalid request: need both type and config")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	conf, err := json.Marshal(req.Config)
	if err != nil {
		msg := fmt.Sprintf("invalid request: bad config: %v", err)
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	httpErr := s.gc.UpdatePipeline(group, syncAll, timeout, req.Type, conf)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("update pipeline succeed: %s: %s", req.Type, conf)
}

func (s *clusterAdminServer) deletePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[delete pipeline]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil || len(pipelineName) == 0 {
		msg := fmt.Sprintf("invalid request: invalid pipeline name")
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	httpErr := s.gc.DeletePipeline(group, syncAll, timeout, pipelineName)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[delete pipeline %s succeed]", pipelineName)
}

func (s *clusterAdminServer) retrievePluginTypes(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin types]")

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.RetrievePluginTypes(group, syncAll, timeout)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve plugin types succeed: %s]", resp)
}

func (s *clusterAdminServer) retrievePipelineTypes(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline types]")

	group, syncAll, timeout, err := parseClusterParam(r)
	if err != nil {
		msg := fmt.Sprintf("invalid request: %s", err.Error())
		rest.Error(w, msg, http.StatusBadRequest)
		return
	}

	timeout = time.Duration(ADMIN_TIMEOUT_DECAY_RATE * float64(timeout))
	resp, httpErr := s.gc.RetrievePipelineTypes(group, syncAll, timeout)
	if httpErr != nil {
		w.WriteHeader(httpErr.StatusCode)
		rest.Error(w, httpErr.Msg, httpErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.ResponseWriter).Write(resp)

	logger.Debugf("[retrieve pipeline types succeed: %s]", resp)
}

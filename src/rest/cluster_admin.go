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
		rest.Get(pav("/#group/sequence"), s.retrieveOperationSequence),

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

func (s *clusterAdminServer) retrieveOperationSequence(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve operation sequence from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(clusterOperationSeqRequest)
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

	seq, clusterErr := s.gc.QueryGroupMaxSeq(group, req.TimeoutSec*time.Second)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(clusterOperationSeqResponse{
		Group:             group,
		OperationSequence: seq,
	})
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[operation sequence %d returned from cluster]", seq)
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
		// defensive programming
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
	logger.Debugf("[create pipeline in cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	req := new(pipelineCreationClusterRequest)
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
		msg := "pipeline config is invalid"
		rest.Error(w, msg, http.StatusBadRequest)
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

	clusterErr := s.gc.CreatePipeline(group, req.TimeoutSec*time.Second, req.OperationSeqSnapshot,
		req.Consistent, req.Type, conf)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("pipeline created in cluster")
}

func (s *clusterAdminServer) retrievePipelines(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipelines from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	req := new(pipelinesRetrieveClusterRequest)
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

	ret, clusterErr := s.gc.RetrievePipelines(group, req.TimeoutSec*time.Second, req.Consistent,
		req.NamePattern, req.Types)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret.Pipelines)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[retrieve pipelines name-pattern(%s) types(%s) succeed: %s]", req.NamePattern, req.Types, ret)
}

func (s *clusterAdminServer) retrievePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pipelineRetrieveClusterRequest)
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

	ret, clusterErr := s.gc.RetrievePipeline(group, req.TimeoutSec*time.Second, req.Consistent, pipelineName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	if ret.Pipeline == nil {
		// defensive programming
		msg := fmt.Sprintf("pipeline %s not found", pipelineName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	w.WriteJson(ret.Pipeline)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[retrieve pipeline %s succeed: %s]", pipelineName, ret)
}

func (s *clusterAdminServer) updatePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[update pipeline in cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pipelineUpdateClusterRequest)
	err = r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	conf, err := json.Marshal(req.Config)
	if err != nil {
		msg := "pipeline config is invalid"
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

	clusterErr := s.gc.UpdatePipeline(group, req.TimeoutSec*time.Second, req.OperationSeqSnapshot,
		req.Consistent, req.Type, conf)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("pipeline updated in cluster")
}

func (s *clusterAdminServer) deletePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[delete pipeline from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pipelineDeletionClusterRequest)
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

	clusterErr := s.gc.DeletePipeline(group, req.TimeoutSec*time.Second, req.OperationSeqSnapshot,
		req.Consistent, pipelineName)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[pipeline %s deleted from cluster]", pipelineName)
}

func (s *clusterAdminServer) retrievePluginTypes(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin types from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pluginTypesRetrieveClusterRequest)
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

	ret, clusterErr := s.gc.RetrievePluginTypes(group, req.TimeoutSec*time.Second, req.Consistent)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret.PluginTypes)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin types returned from cluster]")
}

func (s *clusterAdminServer) retrievePipelineTypes(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline types from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%s]", err.Error())
		return
	}

	req := new(pipelineTypesRetrieveClusterRequest)
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

	ret, clusterErr := s.gc.RetrievePipelineTypes(group, req.TimeoutSec*time.Second, req.Consistent)
	if clusterErr != nil {
		rest.Error(w, clusterErr.Error(), clusterErr.Type.HTTPStatusCode())
		logger.Errorf("[%s]", clusterErr.Error())
		return
	}

	w.WriteJson(ret.PipelineTypes)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[retrieve pipeline types succeed: %s]", ret)
}

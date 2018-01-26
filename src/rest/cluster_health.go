package rest

import (
	"cluster/gateway"
	"common"
	"engine"
	"fmt"
	"logger"
	"net/http"
	"net/url"
	"time"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/go-openapi/spec"

	"version"
)

type clusterHealthServer struct {
	container *restful.Container
	gateway   *engine.Gateway
	gc        *gateway.GatewayCluster
}

func newClusterHealthServer(gateway *engine.Gateway, gc *gateway.GatewayCluster) (*clusterHealthServer, error) {
	container := restful.NewContainer()
	container.DoNotRecover(false)
	container.EnableContentEncoding(true)

	return &clusterHealthServer{
		container: container,
		gateway:   gateway,
		gc:        gc,
	}, nil
}

func (s *clusterHealthServer) GetHandler() http.Handler {
	healthTags := []string{"cluster health check information"}
	groupInfoTags := []string{"cluster group information"}
	memberInfoTags := []string{"cluster member information"}

	accessLogFilter := &accessLogMiddleware{}
	timerFilter := &timerFilter{}
	poweredByFilter := &poweredByFilter{
		XPoweredBy: fmt.Sprintf("EaseGateway/rest-api/%s-%s", version.RELEASE, version.COMMIT),
	}
	clusterAvailabilityMiddleware := &clusterAvailabilityMiddleware{s.gc}
	ws := new(restful.WebService)
	ws.Path("/" + common.APIVersion)
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)
	ws.Filter(accessLogFilter.Process).Filter(timerFilter.Process).Filter(poweredByFilter.Process).
		Filter(clusterAvailabilityMiddleware.Process)

	// groups api
	ws.Route(ws.GET("/info/groups").To(s.retrieveGroupsList).
		// docs
		Doc("Retrieves groups list in a cluster").
		Notes("The cluster groups retrieve endpoint in the cluster returns the "+
			"list of all groups").
		Metadata(restfulspec.KeyOpenAPITags, groupInfoTags).
		Reads(clusterRequest{}).
		Writes(clusterRetrieveGroupsResponse{}).
		Returns(200, "The groups list in a cluster.", clusterRetrieveGroupsResponse{}).
		Returns(404, "The group or member not found by given name.", errorResponse{}).
		Returns(500, "Handle the request failed by internal error.", errorResponse{}).
		Returns(503, "All operations are disallowed when running in standalone mode.", errorResponse{}))

	ws.Route(ws.GET("/info/groups/{group-name}").To(s.retrieveGroup).
		// docs
		Doc("Retrieves specific group information in a cluster").
		Notes("The cluster specific group retrieve endpoint in the cluster returns the "+
			"detail information of the specified group").
		Metadata(restfulspec.KeyOpenAPITags, groupInfoTags).
		Param(ws.PathParameter("group-name", "Group name of cluster to query.").DataType("string")).
		Reads(clusterRequest{}).
		Writes(gateway.RespQueryGroupPayload{}).
		Returns(200, "The group detailed information in a cluster.", gateway.RespQueryGroupPayload{}).
		Returns(206, "The group detailed information in a cluster. But only partial group members respond the query", gateway.RespQueryGroupPayload{}).
		Returns(400, "Invalid group name.", errorResponse{}).
		Returns(404, "The group not found by given name.", errorResponse{}).
		Returns(408, "Request timeout.", errorResponse{}).
		Returns(500, "Handle the request failed by internal error.", errorResponse{}).
		Returns(503, "All operations are disallowed when running in standalone mode.", errorResponse{}))

	// members api
	ws.Route(ws.GET("/info/groups/{group-name}/members").To(s.retrieveMembersInGroup).
		// docs
		Doc("Retrieves members list of a group in a cluster").
		Notes("The cluster members retrieve endpoint in the cluster returns the "+
			"list of members in this group.").
		Metadata(restfulspec.KeyOpenAPITags, memberInfoTags).
		Param(ws.PathParameter("group-name", "Group name of cluster to query").DataType("string")).
		Reads(clusterRequest{}).
		Writes(gateway.RespQueryMembersList{}).
		Returns(200, "The members list of a specific group in a cluster", gateway.RespQueryMembersList{}).
		Returns(404, "The group not found by given name.", errorResponse{}).
		Returns(408, "Request timeout.", errorResponse{}).
		Returns(500, "Handle the request failed by internal error.", errorResponse{}).
		Returns(503, "All operations are disallowed when running in standalone mode.", errorResponse{}))

	ws.Route(ws.GET("/info/groups/{group-name}/members/{member-name}").To(s.retrieveMember).
		// docs
		Doc("Retrieves specific member information in a cluster").
		Notes("The cluster members retrieve endpoint in the cluster returns the "+
			"detail information of the specified member").
		Metadata(restfulspec.KeyOpenAPITags, memberInfoTags).
		Param(ws.PathParameter("member-name", "Member name of cluster to query").DataType("string")).
		Param(ws.PathParameter("group-name", "group name of cluster to query").DataType("string")).
		Reads(clusterRequest{}).
		Writes(gateway.RespQueryMember{}).
		Returns(200, "The member detailed information in a cluster.", gateway.RespQueryMember{}).
		Returns(404, "The member not found by given name.", errorResponse{}).
		Returns(408, "Request timeout.", errorResponse{}).
		Returns(500, "Handle the request failed by internal error.", errorResponse{}).
		Returns(503, "All operations are disallowed when running in standalone mode.", errorResponse{}))

	// health api
	ws.Route(ws.GET("/check/groups").To(s.healthCheckGroups).
		// docs
		Doc("Retrieves health check status for groups in a cluster").
		Notes("The cluster group health check retrieve endpoint in the cluster returns the "+
			"health check status of specific group").
		Metadata(restfulspec.KeyOpenAPITags, healthTags).
		Reads(clusterRequest{}).
		Writes(gateway.RespQueryGroupHealthPayload{}).
		Returns(200, "The specific group's health check status in a cluster.", gateway.RespQueryGroupHealthPayload{}).
		Returns(408, "Request timeout.", errorResponse{}).
		Returns(500, "Handle the request failed by internal error.", errorResponse{}).
		Returns(503, "All operations are disallowed when running in standalone mode.", errorResponse{}))

	ws.Route(ws.GET("/check/groups/{group-name}").To(s.healthCheckGroup).
		// docs
		Doc("Retrieves specific group health check status in a cluster").
		Notes("The cluster group health check retrieve endpoint in the cluster returns the "+
			"health check status of specific group").
		Metadata(restfulspec.KeyOpenAPITags, healthTags).
		Param(ws.PathParameter("group-name", "Group name of cluster to query.").DataType("string")).
		Reads(clusterRequest{}).
		Writes(gateway.RespQueryGroupHealthPayload{}).
		Returns(200, "The specific group's health check status in a cluster.", gateway.RespQueryGroupHealthPayload{}).
		Returns(404, "The group not found by given name.", errorResponse{}).
		Returns(500, "Handle the request failed by internal error.", errorResponse{}).
		Returns(503, "All operations are disallowed when running in standalone mode.", errorResponse{}))

	s.container.Add(ws)

	config := restfulspec.Config{
		APIPath:                       common.PrefixAPIVersion("/apidocs.json"),
		WebServices:                   s.container.RegisteredWebServices(),
		WebServicesURL:                "http://localhost:9090/cluster/info",
		PostBuildSwaggerObjectHandler: s.enrichSwaggerObject}

	s.container.Add(restfulspec.NewOpenAPIService(config))

	return s.container
}

func (s *clusterHealthServer) healthCheckGroups(request *restful.Request, response *restful.Response) {
	logger.Debugf("[health check groups in cluster]")

	req, err := getRequest(request)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: err.Error()}, restful.MIME_JSON)
		logger.Errorf("[healthCheckGroups failed: %s]", err)
		return
	}

	health, clusterErr := s.gc.RetrieveClusterHealthStatus(time.Duration(req.TimeoutSec) * time.Second)
	if clusterErr != nil {
		response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), errorResponse{ErrorMsg: clusterErr.Message}, restful.MIME_JSON)
		logger.Errorf("[healthCheckGroups failed: %s]", clusterErr.Message)
	} else {
		response.WriteAsJson(health)
	}

	logger.Debugf("[healthCheckGroups from cluster returned]")
}

func (s *clusterHealthServer) healthCheckGroup(request *restful.Request, response *restful.Response) {
	logger.Debugf("[health check group in cluster]")
	req, err := getRequest(request)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: err.Error()}, restful.MIME_JSON)
		logger.Errorf("[healthCheckGroup failed: %s]", err)
		return
	}

	group, err := url.QueryUnescape(request.PathParameter("group-name"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: msg}, restful.MIME_JSON)
		logger.Errorf("[healthCheckGroup failed: %s]", msg)
		return
	}

	health, clusterErr := s.gc.RetrieveGroupHealthStatus(group, time.Duration(req.TimeoutSec)*time.Second)
	if clusterErr != nil {
		response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), errorResponse{ErrorMsg: clusterErr.Message}, restful.MIME_JSON)
		logger.Errorf("[retrieveGroup failed: %s]", clusterErr.Message)
	} else {
		response.WriteAsJson(health)
	}

	logger.Debugf("[healthCheckGroup from cluster returned]")
}

func (s *clusterHealthServer) enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "Cluster Health Service",
			Description: "Expose cluster health status for managing or debuging Cluster",
			Contact:     &contact,
			Version:     version1,
		},
	}
	swo.SwaggerProps.BasePath = "/cluster/health"
}

func (s *clusterHealthServer) retrieveGroupsList(request *restful.Request, response *restful.Response) {
	logger.Debugf("[retrieve groups list from cluster]")

	groups, clusterErr := s.gc.RetrieveGroupsList()
	if clusterErr != nil {
		response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), errorResponse{ErrorMsg: clusterErr.Message}, restful.MIME_JSON)
		logger.Errorf("[retrieveGroups failed: %s]", clusterErr.Message)
		return
	}
	response.WriteAsJson(newClusterRetrieveGroupsResponse(groups))

	logger.Debugf("[retrieve groups from cluster returned]")
}

func (s *clusterHealthServer) retrieveGroup(request *restful.Request, response *restful.Response) {
	logger.Debugf("[retrieve group info from cluster]")

	group, err := url.QueryUnescape(request.PathParameter("group-name"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: msg}, restful.MIME_JSON)
		logger.Errorf("[retrieveGroup failed: %s]", msg)
		return
	}
	req, err := getRequest(request)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: err.Error()}, restful.MIME_JSON)
		logger.Errorf("[retrieveMembersInGroup failed: %s]", err)
		return
	}

	groupInfo, clusterErr := s.gc.RetrieveGroup(group, time.Duration(req.TimeoutSec)*time.Second)
	if clusterErr != nil {
		if clusterErr.Type == gateway.QueryPartiallyCompleteError {
			// partial content
			response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), groupInfo, restful.MIME_JSON)
		} else {
			response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), errorResponse{ErrorMsg: clusterErr.Message}, restful.MIME_JSON)
			logger.Errorf("[retrieveGroup failed: %s]", clusterErr.Message)
		}
	} else {
		response.WriteAsJson(groupInfo)
	}

	logger.Debugf("[retrieve group info from cluster returned]")
}

func (s *clusterHealthServer) retrieveMembersInGroup(request *restful.Request, response *restful.Response) {
	logger.Debugf("[retrieve members info in specific group from cluster]")

	group, err := url.QueryUnescape(request.PathParameter("group-name"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: msg}, restful.MIME_JSON)
		logger.Errorf("[retrieveMembersInGroup failed: %s]", msg)
		return
	}

	req, err := getRequest(request)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: err.Error()}, restful.MIME_JSON)
		logger.Errorf("[retrieveMembersInGroup failed: %s]", err)
		return
	}

	membersInfo, clusterErr := s.gc.RetrieveMembersList(group, time.Duration(req.TimeoutSec)*time.Second)
	if clusterErr != nil {
		response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), errorResponse{clusterErr.Message}, restful.MIME_JSON)
	} else {
		response.WriteAsJson(membersInfo)
	}

	logger.Debugf("[retrieve group info from cluster returned]")
}

func (s *clusterHealthServer) retrieveMember(request *restful.Request, response *restful.Response) {
	logger.Debugf("[retrieve member info from cluster]")

	memberName, err := url.QueryUnescape(request.PathParameter("member-name"))
	if err != nil || len(memberName) == 0 {
		msg := "invalid cluster member name"
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: msg}, restful.MIME_JSON)
		logger.Errorf("[retrieveMember failed: %s]", msg)
		return
	}

	group, err := url.QueryUnescape(request.PathParameter("group-name"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: msg}, restful.MIME_JSON)
		logger.Errorf("[retrieveMembersInGroup failed: %s]", msg)
		return
	}

	req, err := getRequest(request)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, errorResponse{ErrorMsg: err.Error()}, restful.MIME_JSON)
		logger.Errorf("[retrieveMembersInGroup failed: %s]", err)
		return
	}

	member, clusterErr := s.gc.RetrieveMember(group, memberName, time.Duration(req.TimeoutSec)*time.Second)
	if clusterErr != nil {
		response.WriteHeaderAndJson(clusterErr.Type.HTTPStatusCode(), errorResponse{clusterErr.Message}, restful.MIME_JSON)
	} else {
		response.WriteAsJson(member)
	}

	logger.Debugf("[retrieve member info from cluster returned]")
}

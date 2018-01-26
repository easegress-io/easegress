package rest

import (
	"fmt"
	"time"

	"common"
	"config"

	"github.com/emicklei/go-restful"
	"option"
)

//
// Admin API
//

type pluginCreationRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

type pluginsRetrieveRequest struct {
	NamePattern string   `json:"name_pattern,omitempty"`
	Types       []string `json:"types"`
}

type pluginsRetrieveResponse struct {
	Plugins []config.PluginSpec `json:"plugins"`
}

type pluginUpdateRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

////

type pipelineCreationRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

type pipelinesRetrieveRequest struct {
	NamePattern string   `json:"name_pattern,omitempty"`
	Types       []string `json:"types"`
}

type pipelinesRetrieveResponse struct {
	Pipelines []config.PipelineSpec `json:"pipelines"`
}

type pipelineUpdateRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

////

type pluginTypesRetrieveResponse struct {
	PluginTypes []string `json:"plugin_types"`
}

type pipelineTypesRetrieveResponse struct {
	PipelineTypes []string `json:"pipeline_types"`
}

//
// Statistics API
//

type indicatorsValueRetrieveRequest struct {
	IndicatorNames []string `json:"indicator_names"`
}

type indicatorNamesRetrieveResponse struct {
	Names []string `json:"names"`
}

type indicatorValueRetrieveResponse struct {
	Value interface{} `json:"value"`
}

type indicatorsValueRetrieveResponse struct {
	Values map[string]interface{} `json:"values"`
}

type indicatorDescriptionRetrieveResponse struct {
	Description interface{} `json:"desc"`
}

////

type gatewayUpTimeRetrieveResponse struct {
	UpTime time.Duration `json:"up_nanosec"`
}

//
// Cluster API
//
type errorResponse struct {
	ErrorMsg string `json:"Error"` // json tag as 'Error' to keep consistent with other API's error response
}

type clusterRequest struct {
	TimeoutSec uint16 `json:"timeout_sec" optional:"true" default:"120" description:"Timeout for cluster operations"` // 10-65535, zero means using default value (option.ClusterDefaultOpTimeout)
}

func getRequest(request *restful.Request) (*clusterRequest, error) {
	req := new(clusterRequest)
	err := request.ReadEntity(req)
	if err != nil {
		return nil, fmt.Errorf("decode request failed: %v", err)
	}

	if req.TimeoutSec == 0 {
		req.TimeoutSec = uint16(option.ClusterDefaultOpTimeout.Seconds())
	} else if req.TimeoutSec < 10 {
		return nil, fmt.Errorf("timeout (%d second(s)) should greater than or equal to 10 senconds",
			req.TimeoutSec)
	}
	return req, nil
}

//
// Cluster info API
//
type ClusterResponse struct {
	Time time.Time `json:"time" description:"Timestamp of the response"`
}

type clusterRetrieveGroupsResponse struct {
	ClusterResponse
	GroupsCount int      `json:"groups_count"`
	Groups      []string `json:"groups"`
}

func newClusterRetrieveGroupsResponse(groups []string) *clusterRetrieveGroupsResponse {
	return &clusterRetrieveGroupsResponse{
		ClusterResponse: ClusterResponse{
			Time: common.Now(),
		},
		GroupsCount: len(groups),
		Groups:      groups,
	}
}

//
// Cluster admin API
//

type clusterOperationSeqRequest struct {
	clusterRequest
}

type clusterOperationSeqResponse struct {
	Group             string `json:"cluster_group"`
	OperationSequence uint64 `json:"operation_seq"`
}

type clusterOperation struct {
	clusterRequest
	OperationSeq uint64 `json:"operation_seq"`
	Consistent   bool   `json:"consistent"`
}

type pluginCreationClusterRequest struct {
	clusterOperation
	pluginCreationRequest
}

type pluginsRetrieveClusterRequest struct {
	clusterRequest
	pluginsRetrieveRequest
	Consistent bool `json:"consistent"`
}

type pluginRetrieveClusterRequest struct {
	clusterRequest
	Consistent bool `json:"consistent"`
}

type pluginUpdateClusterRequest struct {
	clusterOperation
	pluginUpdateRequest
}

type pluginDeletionClusterRequest struct {
	clusterOperation
}

////

type pipelineCreationClusterRequest struct {
	clusterOperation
	pipelineCreationRequest
}

type pipelinesRetrieveClusterRequest struct {
	clusterRequest
	pipelinesRetrieveRequest
	Consistent bool `json:"consistent"`
}

type pipelineRetrieveClusterRequest struct {
	clusterRequest
	Consistent bool `json:"consistent"`
}

type pipelineUpdateClusterRequest struct {
	clusterOperation
	pipelineUpdateRequest
}

type pipelineDeletionClusterRequest struct {
	clusterOperation
}

////

type pluginTypesRetrieveClusterRequest struct {
	clusterRequest
	Consistent bool `json:"consistent"`
}

type pipelineTypesRetrieveClusterRequest struct {
	clusterRequest
	Consistent bool `json:"consistent"`
}

//
// Cluster statistics API
//

type statisticsClusterRequest struct {
	clusterRequest
	Detail bool `json:"detail"`
}

type indicatorsValueRetrieveClusterRequest struct {
	statisticsClusterRequest
	indicatorsValueRetrieveRequest
}

//
// Health check API
//

type healthInfoResponse struct {
	Build buildInfo `json:"build"`
}

type buildInfo struct {
	Name       string `json:"name"`
	Release    string `json:"release"`
	Build      string `json:"build"`
	Repository string `json:"repository"`
}

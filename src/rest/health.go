package rest

import (
	"fmt"
	"strings"
	"time"

	"github.com/ant0ine/go-json-rest/rest"

	"cluster/gateway"
	"common"
	"engine"
	"logger"
)

type healthCheckServer struct {
	gateway *engine.Gateway
	gc      *gateway.GatewayCluster
}

func newHealthCheckServer(gateway *engine.Gateway, gc *gateway.GatewayCluster) (*healthCheckServer, error) {
	return &healthCheckServer{
		gateway: gateway,
		gc:      gc,
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
	api.Use(rest.DefaultCommonStack...)
	api.SetApp(router)

	return api, nil
}

func (s *healthCheckServer) existing(w rest.ResponseWriter, req *rest.Request) {
	logger.Debugf("[check existing]")

	logger.Debugf("[existing status returned]")
}

func (s *healthCheckServer) info(w rest.ResponseWriter, req *rest.Request) {
	logger.Debugf("[get member info from cluster]")

	groupMaxSeqStr := "UNKNOW"
	groupMaxSeq, err := s.gc.QueryGroupMaxSeq(common.ClusterGroup, 10*time.Second)
	if err == nil {
		groupMaxSeqStr = fmt.Sprintf("%d", groupMaxSeq)
	}

	// keep same datatype of group max sequence for client
	localMaxSeqStr := fmt.Sprintf("%d", s.gc.OPLog().MaxSeq())

	members := make([]string, 0)
	for _, member := range s.gc.RestAliveMembersInSameGroup() {
		members = append(members, fmt.Sprintf("%s (%s:%d) %v",
			member.NodeName, member.Address.String(), member.Port, member.NodeTags))
	}

	w.WriteJson(struct {
		CI clusterInfo `json:"cluster"`
	}{
		clusterInfo{
			Name:                  s.gc.NodeName(),
			Mode:                  strings.ToLower(s.gc.Mode().String()),
			Group:                 common.ClusterGroup,
			GroupMaxSeq:           groupMaxSeqStr,
			LocalMaxSeq:           localMaxSeqStr,
			Peers:                 members,
			OPLogMaxSeqGapToPull:  common.OPLogMaxSeqGapToPull,
			OPLogPullMaxCountOnce: common.OPLogPullMaxCountOnce,
			OPLogPullInterval:     int(common.OPLogPullInterval.Seconds()),
			OPLogPullTimeout:      int(common.OPLogPullTimeout.Seconds()),
		},
	})

	logger.Debugf("[cluster member info returned]")
}

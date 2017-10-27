package rest

import (
	"cluster/gateway"
	"common"
	"engine"
	"logger"
	"net/http"
	"net/url"

	"github.com/ant0ine/go-json-rest/rest"
)

type clusterMetaServer struct {
	gateway *engine.Gateway
	gc      *gateway.GatewayCluster
}

func newClusterMetaServer(gateway *engine.Gateway, gc *gateway.GatewayCluster) (*clusterMetaServer, error) {
	return &clusterMetaServer{
		gateway: gateway,
		gc:      gc,
	}, nil
}

func (s *clusterMetaServer) Api() (*rest.Api, error) {
	pav := common.PrefixAPIVersion
	router, err := rest.MakeRouter(
		rest.Get(pav("/groups"), s.retrieveGroups),
		rest.Get(pav("/groups/#group"), s.retrieveGroup),

		rest.Get(pav("/members"), s.retrieveMembers),
		rest.Get(pav("/members/#member"), s.retrieveMember),
	)

	if err != nil {
		logger.Errorf("[make router for cluster meta server failed: %v]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(RestStack...)
	api.SetApp(router)

	return api, nil
}

func (s *clusterMetaServer) retrieveGroups(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve groups from cluster]")

	groups := s.gc.RetrieveGroups()
	w.WriteJson(clusterRetrieveGroupsResponse{
		Groups: groups,
	})

	logger.Debugf("[retrieve groups from cluster returned]")
}

func (s *clusterMetaServer) retrieveGroup(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve group info from cluster]")

	group, err := url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		msg := "invalid cluster group name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	groupInfo := s.gc.RetrieveGroup(group)
	w.WriteJson(groupInfo)

	logger.Debugf("[retrieve group info from cluster returned]")
}

func (s *clusterMetaServer) retrieveMembers(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve memebers from cluster]")

	members := s.gc.RetrieveMembers()
	w.WriteJson(clusterRetrieveMembersResponse{
		Members: members,
	})

	logger.Debugf("[retrieve members from cluster returned]")
}

func (s *clusterMetaServer) retrieveMember(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve member info from cluster]")

	memberName, err := url.QueryUnescape(r.PathParam("member"))
	if err != nil || len(memberName) == 0 {
		msg := "invalid cluster member name"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	memberInfo := s.gc.RetrieveMember(memberName)
	w.WriteJson(memberInfo)

	logger.Debugf("[retrieve member info from cluster returned]")
}

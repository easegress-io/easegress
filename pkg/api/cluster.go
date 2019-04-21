package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/registry"

	"github.com/kataras/iris"
	yaml "gopkg.in/yaml.v2"
)

func (s *Server) setupMemberAPIs() {
	memberAPIs := []*apiEntry{
		{
			Path:    "/members",
			Method:  "GET",
			Handler: s.listMembers,
		},
		{
			Path:    "/members/{member:string}",
			Method:  "DELETE",
			Handler: s.purgeMember,
		},
	}

	s.apis = append(s.apis, memberAPIs...)
}

type (
	// Member is the member info.
	Member struct {
		option.Options `json:",inline"`
	}

	// ListMembersResp is the response of list member.
	ListMembersResp struct {
		Leader  string                 `yaml:"leader"`
		Members []Member               `yaml:"members"`
		Status  []cluster.MemberStatus `yaml:"status"`
	}
)

// These methods which operate with cluster guarantee atomicity.

func (s *Server) listMembers(ctx iris.Context) {
	resp := ListMembersResp{
		Leader:  s.cluster.Leader(),
		Members: make([]Member, 0),
	}

	kv, err := s.cluster.GetPrefix(cluster.MemberConfigPrefix)
	if err != nil {
		clusterPanic(err)
	}

	for _, v := range kv {
		var o option.Options
		err := yaml.Unmarshal([]byte(v), &o)
		if err != nil {
			panic(fmt.Errorf("unmarshal %s to options failed: %v", v, err))
		}
		resp.Members = append(resp.Members, Member{Options: o})
	}

	kv, err = s.cluster.GetPrefix(cluster.MemberStatusPrefix)
	if err != nil {
		clusterPanic(err)
	}

	for _, v := range kv {
		var s cluster.MemberStatus

		err := yaml.Unmarshal([]byte(v), &s)
		if err != nil {
			panic(fmt.Errorf("unmarshal %s to member status failed: %v", v, err))
		}

		if time.Unix(s.LastHeartbeatTime, 0).Add(cluster.KEEP_ALIVE_INTERVAL * 2).Before(time.Now()) {
			s.EtcdStatus = "offline"
		}

		resp.Status = append(resp.Status, s)
	}

	buff, err := yaml.Marshal(resp)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", resp, err))
	}

	ctx.Write(buff)
}

func (s *Server) purgeMember(ctx iris.Context) {
	member := ctx.Params().Get("member")
	err := s.cluster.PurgeMember(member)
	if err != nil {
		clusterPanic(fmt.Errorf("failed to purge member : %s, error: %s", member, err))
	}

}

func (s *Server) _getObject(name string) registry.Spec {
	value, err := s.cluster.Get(cluster.ConfigObjectPrefix + name)
	if err != nil {
		clusterPanic(err)
	}

	if value == nil {
		return nil
	}

	spec, err := registry.SpecFromYAML(*value)
	if err != nil {
		panic(fmt.Errorf("bad spec(err: %v) from yaml: %s", err, *value))
	}

	return spec
}

func (s *Server) _listObjects() []registry.Spec {
	kvs, err := s.cluster.GetPrefix(cluster.ConfigObjectPrefix)
	if err != nil {
		clusterPanic(err)
	}

	specs := make([]registry.Spec, 0, len(kvs))
	for _, v := range kvs {
		spec, err := registry.SpecFromYAML(v)
		if err != nil {
			panic(fmt.Errorf("bad spec(err: %v) from yaml: %s", err, v))
		}
		specs = append(specs, spec)
	}

	return specs
}

func (s *Server) _putObject(spec registry.Spec) {
	err := s.cluster.Put(cluster.ConfigObjectPrefix+spec.GetName(),
		registry.YAMLFromSpec(spec))
	if err != nil {
		clusterPanic(err)
	}
}

func (s *Server) _deleteObject(name string) {
	err := s.cluster.Delete(cluster.ConfigObjectPrefix + name)
	if err != nil {
		clusterPanic(err)
	}
}

func (s *Server) _getObjectStatus(name string) map[string]string {
	prefix := fmt.Sprintf(cluster.StatusObjectPrefixFormat, name)
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		clusterPanic(err)
	}

	statuses := make(map[string]string)
	for k, v := range kvs {
		statuses[strings.TrimPrefix(k, prefix)] = v
	}

	return statuses
}

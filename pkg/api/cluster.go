package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/stat"
	"github.com/megaease/easegateway/pkg/store"

	"github.com/kataras/iris"
	yaml "gopkg.in/yaml.v2"
)

func clusterPanic(err error) {
	panic(clusterErr(err))
}

func (s *APIServer) setupMemberAPIs() {
	memberAPIs := []*apiEntry{
		{
			Path:    "/members",
			Method:  "GET",
			Handler: s.listMembers,
		},
	}

	s.apis = append(s.apis, memberAPIs...)
}

type (
	Member struct {
		option.Options `json:",inline"`
	}

	ListMemberResp struct {
		Leader  string   `json:"leader"`
		Members []Member `json:"members"`
	}
)

// These methods which operate with cluster guarantee atomicity.

func (s *APIServer) listMembers(ctx iris.Context) {
	resp := ListMemberResp{
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
			panic(fmt.Errorf("unmarshal %s to yaml failed: %v", v, err))
		}
		resp.Members = append(resp.Members, Member{Options: o})
	}

	buff, err := json.Marshal(resp)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", resp, err))
	}

	ctx.Write(buff)
}

func (s *APIServer) _getPlugin(name string) *store.PluginSpec {
	value, err := s.cluster.Get(cluster.ConfigPluginPrefix + name)
	if err != nil {
		clusterPanic(err)
	}

	if value == nil {
		return nil
	}

	spec, err := store.NewPluginSpec(*value)
	if err != nil {
		panic(err)
	}
	return spec
}

func (s *APIServer) _listPlugins() []*store.PluginSpec {
	kvs, err := s.cluster.GetPrefix(cluster.ConfigPluginPrefix)
	if err != nil {
		clusterPanic(err)
	}

	plugins := make([]*store.PluginSpec, 0, len(kvs))
	for _, v := range kvs {
		spec, err := store.NewPluginSpec(v)
		if err != nil {
			panic(err)
		}
		plugins = append(plugins, spec)
	}

	return plugins
}

func (s *APIServer) _putPlugin(spec *store.PluginSpec) {
	value, err := json.Marshal(spec)
	if err != nil {
		panic(err)
	}
	err = s.cluster.Put(cluster.ConfigPluginPrefix+spec.Name, string(value))
	if err != nil {
		clusterPanic(err)
	}
}

func (s *APIServer) _deletePlugin(name string) {
	err := s.cluster.Delete(cluster.ConfigPluginPrefix + name)
	if err != nil {
		clusterPanic(err)
	}
}

func (s *APIServer) _getPipeline(name string) *store.PipelineSpec {
	value, err := s.cluster.Get(cluster.ConfigPipelinePrefix + name)
	if err != nil {
		clusterPanic(err)
	}

	if value == nil {
		return nil
	}

	spec, err := store.NewPipelineSpec(*value)
	if err != nil {
		panic(err)
	}

	return spec
}

func (s *APIServer) _putPipeline(oldSpec, spec *store.PipelineSpec) {
	v, err := json.Marshal(spec)
	if err != nil {
		panic(err)
	}
	value := string(v)

	kvs := make(map[string]*string)
	kvs[cluster.ConfigPipelinePrefix+spec.Name] = &value

	for _, pluginName := range spec.Config.Plugins {
		key := fmt.Sprintf(cluster.ConfigPluginUsedKeyFormat, pluginName, spec.Name)
		emptyValue := ""
		kvs[key] = &emptyValue
	}
	if oldSpec != nil {
		for _, oldPluginName := range oldSpec.Config.Plugins {
			if _, exists := kvs[oldPluginName]; !exists {
				kvs[oldPluginName] = nil
			}
		}
	}

	err = s.cluster.PutAndDelete(kvs)
	if err != nil {
		clusterPanic(err)
	}
}

func (s *APIServer) _deletePipeline(spec *store.PipelineSpec) {
	kvs := make(map[string]*string)

	kvs[cluster.ConfigPipelinePrefix+spec.Name] = nil
	for _, pluginName := range spec.Config.Plugins {
		key := fmt.Sprintf(cluster.ConfigPluginUsedKeyFormat, pluginName, spec.Name)
		kvs[key] = nil
	}

	err := s.cluster.PutAndDelete(kvs)
	if err != nil {
		clusterPanic(err)
	}
}

func (s *APIServer) _listPipelines() []*store.PipelineSpec {
	kvs, err := s.cluster.GetPrefix(cluster.ConfigPipelinePrefix)
	if err != nil {
		clusterPanic(err)
	}

	pipelines := make([]*store.PipelineSpec, 0, len(kvs))
	for _, v := range kvs {
		spec, err := store.NewPipelineSpec(v)
		if err != nil {
			panic(err)
		}
		pipelines = append(pipelines, spec)
	}

	return pipelines
}

func (s *APIServer) _pluginUsedByPipelines(name string) []string {
	prefix := fmt.Sprintf(cluster.ConfigPluginUsedPrefixFormat, name)
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		clusterPanic(err)
	}

	pipelines := make([]string, 0, len(kvs))
	for k := range kvs {
		pipelines = append(pipelines, strings.TrimPrefix(k, prefix))
	}

	return pipelines
}

func (s *APIServer) _pipelineNames() []string {
	pipelines := s._listPipelines()

	names := make([]string, 0, len(pipelines))
	for _, spec := range pipelines {
		names = append(names, spec.Name)
	}

	return names
}

func (s *APIServer) _pluginNames() map[string]struct{} {
	plugins := s._listPlugins()

	names := make(map[string]struct{})
	for _, spec := range plugins {
		names[spec.Name] = struct{}{}
	}

	return names
}

// The keys of result are pipelineName, memberName.
func (s *APIServer) _listStats() map[string]map[string]stat.PipelineStat {
	kvs, err := s.cluster.GetPrefix(cluster.StatPipelinePrefix)
	if err != nil {
		clusterPanic(err)
	}

	stats := make(map[string]map[string]stat.PipelineStat)

	for key, value := range kvs {
		names := strings.Split(strings.TrimPrefix(key, cluster.StatPipelinePrefix), "/")
		if len(names) != 2 {
			panic(fmt.Errorf("get invalid key %s", key))
		}
		pipelineName, memberName := names[0], names[1]

		ps := new(stat.PipelineStat)
		err := json.Unmarshal([]byte(value), ps)
		if err != nil {
			panic(fmt.Errorf("unmarshal %s to json failed: %v", value, err))
		}

		if stats[pipelineName] == nil {
			stats[pipelineName] = make(map[string]stat.PipelineStat)
		}
		stats[pipelineName][memberName] = *ps
	}

	return stats
}

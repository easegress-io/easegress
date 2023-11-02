/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package nginx

import (
	"fmt"

	"github.com/megaease/easegress/v2/cmd/client/commandv2/common"
	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
)

func convertConfig(options *Options, config *Config) ([]*common.HTTPServerSpec, []*common.PipelineSpec, error) {
	httpServers := make([]*common.HTTPServerSpec, 0)
	pipelines := make([]*common.PipelineSpec, 0)
	for _, server := range config.Servers {
		s, p, err := convertServer(options, server)
		if err != nil {
			return nil, nil, err
		}
		httpServers = append(httpServers, s)
		pipelines = append(pipelines, p...)
	}
	return httpServers, pipelines, nil
}

func convertServer(options *Options, server *Server) (*common.HTTPServerSpec, []*common.PipelineSpec, error) {
	pipelines := make([]*common.PipelineSpec, 0)
	httpServer := common.NewHTTPServerSpec(fmt.Sprintf("%s-%d", options.Prefix, server.Port))
	httpServer = convertServerBase(httpServer, server.ServerBase)

	httpServer.Rules = make([]*routers.Rule, 0)
	for _, rule := range server.Rules {
		routerRule, pls := convertRule(options, rule)
		httpServer.Rules = append(httpServer.Rules, routerRule)
		pipelines = append(pipelines, pls...)
	}
	return httpServer, pipelines, nil
}

func convertServerBase(spec *common.HTTPServerSpec, base ServerBase) *common.HTTPServerSpec {
	spec.Port = uint16(base.Port)
	spec.Address = base.Address
	spec.HTTPS = base.HTTPS
	spec.CaCertBase64 = base.CaCert
	for k, v := range base.Certs {
		spec.Certs[k] = v
	}
	for k, v := range base.Keys {
		spec.Keys[k] = v
	}
	return spec
}

func convertRule(options *Options, rule *Rule) (*routers.Rule, []*common.PipelineSpec) {
	router := &routers.Rule{
		Hosts: make([]routers.Host, 0),
		Paths: make([]*routers.Path, 0),
	}
	for _, h := range rule.Hosts {
		router.Hosts = append(router.Hosts, routers.Host{
			Value:    h.Value,
			IsRegexp: h.IsRegexp,
		})
	}
	pipelines := make([]*common.PipelineSpec, 0)
	for _, p := range rule.Paths {
		name := options.GetPipelineName(p.Path)
		path := routers.Path{
			Backend: name,
		}
		switch p.Type {
		case PathTypeExact:
			path.Path = p.Path
		case PathTypePrefix:
			path.PathPrefix = p.Path
		case PathTypeRe:
			path.PathRegexp = p.Path
		case PathTypeReInsensitive:
			path.PathRegexp = fmt.Sprintf("(?i)%s", p.Path)
		default:
			general.Warnf("unknown path type: %s", p.Type)
		}
		router.Paths = append(router.Paths, &path)
		pipelineSpec := convertProxy(name, p.Backend)
		pipelines = append(pipelines, pipelineSpec)
	}
	return router, pipelines
}

func convertProxy(name string, info *ProxyInfo) *common.PipelineSpec {
	pipeline := common.NewPipelineSpec(name)
	// TODO: set header, update proxy filter
	proxy := common.NewProxyFilterSpec("proxy")
	pipeline.SetFilters([]filters.Spec{proxy})
	return pipeline
}

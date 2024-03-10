/*
 * Copyright (c) 2017, The Easegress Authors
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
	"regexp"
	"sort"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/commandv2/specs"
	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/builder"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func convertConfig(options *Options, config *Config) ([]*specs.HTTPServerSpec, []*specs.PipelineSpec, error) {
	httpServers := make([]*specs.HTTPServerSpec, 0)
	pipelines := make([]*specs.PipelineSpec, 0)
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

func convertServer(options *Options, server *Server) (*specs.HTTPServerSpec, []*specs.PipelineSpec, error) {
	pipelines := make([]*specs.PipelineSpec, 0)
	httpServer := specs.NewHTTPServerSpec(fmt.Sprintf("%s-%d", options.ResourcePrefix, server.Port))
	httpServer = convertServerBase(httpServer, server.ServerBase)

	httpServer.Rules = make([]*routers.Rule, 0)
	for _, rule := range server.Rules {
		routerRule, pls := convertRule(options, rule)
		httpServer.Rules = append(httpServer.Rules, routerRule)
		pipelines = append(pipelines, pls...)
	}
	return httpServer, pipelines, nil
}

func convertServerBase(spec *specs.HTTPServerSpec, base ServerBase) *specs.HTTPServerSpec {
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

// convertRule converts nginx conf to easegress rule.
// exact path > prefix path > regexp path.
// prefix path should be sorted by path length.
func convertRule(options *Options, rule *Rule) (*routers.Rule, []*specs.PipelineSpec) {
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

	pipelines := make([]*specs.PipelineSpec, 0)
	exactPaths := make([]*routers.Path, 0)
	prefixPaths := make([]*routers.Path, 0)
	rePaths := make([]*routers.Path, 0)
	for _, p := range rule.Paths {
		name := options.GetPipelineName(p.Path)
		path := &routers.Path{
			Backend: name,
		}
		// path for websocket should not limit body size.
		if isWebsocket(p.Backend) {
			path.ClientMaxBodySize = -1
		}
		switch p.Type {
		case PathTypeExact:
			path.Path = p.Path
			exactPaths = append(exactPaths, path)
		case PathTypePrefix:
			path.PathPrefix = p.Path
			prefixPaths = append(prefixPaths, path)
		case PathTypeRe:
			path.PathRegexp = p.Path
			rePaths = append(rePaths, path)
		case PathTypeReInsensitive:
			path.PathRegexp = fmt.Sprintf("(?i)%s", p.Path)
			rePaths = append(rePaths, path)
		default:
			general.Warnf("unknown path type: %s", p.Type)
		}
		pipelineSpec := convertProxy(name, p.Backend)
		pipelines = append(pipelines, pipelineSpec)
	}
	sort.Slice(prefixPaths, func(i, j int) bool {
		return len(prefixPaths[i].PathPrefix) > len(prefixPaths[j].PathPrefix)
	})
	router.Paths = append(router.Paths, exactPaths...)
	router.Paths = append(router.Paths, prefixPaths...)
	router.Paths = append(router.Paths, rePaths...)
	return router, pipelines
}

func convertProxy(name string, info *ProxyInfo) *specs.PipelineSpec {
	pipeline := specs.NewPipelineSpec(name)

	flow := make([]filters.Spec, 0)
	if len(info.SetHeaders) != 0 {
		adaptor := getRequestAdaptor(info)
		if adaptor != nil {
			flow = append(flow, adaptor)
		}
	}

	var proxy filters.Spec
	if isWebsocket(info) {
		proxy = getWebsocketFilter(info)
	} else {
		proxy = getProxyFilter(info)
	}
	flow = append(flow, proxy)
	pipeline.SetFilters(flow)
	return pipeline
}

var nginxEmbeddedVarRe *regexp.Regexp
var nginxToTemplateMap = map[string]string{
	"$host":           ".req.Host",
	"$hostname":       ".req.Host",
	"$content_length": `header .req.Header "Content-Length"`,
	"$content_type":   `header .req.Header "Content-Type"`,
	"$remote_addr":    ".req.RemoteAddr",
	"$remote_user":    "username .req",
	"$request_body":   ".req.Body",
	"$request_method": ".req.Method",
	"$request_uri":    ".req.RequestURI",
	"$scheme":         ".req.URL.Scheme",
}

func translateNginxEmbeddedVar(s string) string {
	if nginxEmbeddedVarRe == nil {
		nginxEmbeddedVarRe = regexp.MustCompile(`\$[a-zA-Z0-9_]+`)
	}
	return nginxEmbeddedVarRe.ReplaceAllStringFunc(s, func(s string) string {
		newValue := nginxToTemplateMap[s]
		if newValue == "" {
			newValue = s
			msg := "nginx embedded value %s is not supported now, "
			msg += "please check easegress RequestAdaptor filter for more information about template."
			general.Warnf(msg, s)
		}
		return fmt.Sprintf("{{ %s }}", newValue)
	})
}

func getRequestAdaptor(info *ProxyInfo) *builder.RequestAdaptorSpec {
	spec := specs.NewRequestAdaptorFilterSpec("request-adaptor")
	template := &builder.RequestAdaptorTemplate{
		Header: &httpheader.AdaptSpec{
			Set: make(map[string]string),
		},
	}
	for k, v := range info.SetHeaders {
		if k == "Upgrade" || k == "Connection" {
			continue
		}
		template.Header.Set[k] = translateNginxEmbeddedVar(v)
	}
	if len(template.Header.Set) == 0 {
		return nil
	}
	data := codectool.MustMarshalYAML(template)
	spec.Template = string(data)
	return spec
}

func isWebsocket(info *ProxyInfo) bool {
	return info.SetHeaders["Upgrade"] != "" && info.SetHeaders["Connection"] != ""
}

func getWebsocketFilter(info *ProxyInfo) *httpproxy.WebSocketProxySpec {
	for i, s := range info.Servers {
		s.Server = strings.Replace(s.Server, "http://", "ws://", 1)
		s.Server = strings.Replace(s.Server, "https://", "wss://", 1)
		info.Servers[i] = s
	}
	spec := specs.NewWebsocketFilterSpec("websocket")
	spec.Pools = []*httpproxy.WebSocketServerPoolSpec{{
		BaseServerPoolSpec: getBaseServerPool(info),
	}}
	return spec
}

func getProxyFilter(info *ProxyInfo) *httpproxy.Spec {
	spec := specs.NewProxyFilterSpec("proxy")
	if info.GzipMinLength != 0 {
		spec.Compression = &httpproxy.CompressionSpec{
			MinLength: uint32(info.GzipMinLength),
		}
	}
	spec.Pools = []*httpproxy.ServerPoolSpec{{
		BaseServerPoolSpec: getBaseServerPool(info),
	}}
	return spec
}

func getBaseServerPool(info *ProxyInfo) httpproxy.BaseServerPoolSpec {
	servers := make([]*proxies.Server, len(info.Servers))
	policy := proxies.LoadBalancePolicyRoundRobin
	for i, s := range info.Servers {
		servers[i] = &proxies.Server{
			URL:    s.Server,
			Weight: s.Weight,
		}
		if s.Weight != 1 {
			policy = proxies.LoadBalancePolicyWeightedRandom
		}
	}
	return httpproxy.BaseServerPoolSpec{
		Servers: servers,
		LoadBalance: &proxies.LoadBalanceSpec{
			Policy: policy,
		},
	}
}

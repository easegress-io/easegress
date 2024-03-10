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

package ingresscontroller

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"

	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	proxy "github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	apicorev1 "k8s.io/api/core/v1"
	apinetv1 "k8s.io/api/networking/v1"
)

const (
	defaultPipelineName = "pipeline-default"
)

type (
	// specTranslator translates k8s ingress related specs to Easegress http server
	// spec and pipeline specs
	specTranslator struct {
		k8sClient    *k8sClient
		httpSvr      *supervisor.Spec
		pipelines    map[string]*supervisor.Spec
		httpSvrCfg   *httpserver.Spec
		ingressClass string
	}

	pipelineSpecBuilder struct {
		Kind          string `json:"kind"`
		Name          string `json:"name"`
		pipeline.Spec `json:",inline"`
	}

	httpServerSpecBuilder struct {
		Kind            string `json:"kind"`
		Name            string `json:"name"`
		httpserver.Spec `json:",inline"`
	}
)

func newPipelineSpecBuilder(name string) *pipelineSpecBuilder {
	return &pipelineSpecBuilder{
		Kind: pipeline.Kind,
		Name: name,
		Spec: pipeline.Spec{},
	}
}

func (b *pipelineSpecBuilder) addProxy(endpoints []string, ingress *apinetv1.Ingress) error {
	const name = "proxy"

	proxyLoadBalance := ingress.Annotations["easegress.ingress.kubernetes.io/proxy-load-balance"]
	proxyHeaderHashKey := ingress.Annotations["easegress.ingress.kubernetes.io/proxy-header-hash-key"]
	proxyForwardKey := ingress.Annotations["easegress.ingress.kubernetes.io/proxy-forward-key"]
	proxyServerMaxSize := ingress.Annotations["easegress.ingress.kubernetes.io/proxy-server-max-size"]
	proxyTimeout := ingress.Annotations["easegress.ingress.kubernetes.io/proxy-timeout"]

	switch proxyLoadBalance {
	case "":
		proxyLoadBalance = proxies.LoadBalancePolicyRoundRobin
	case proxies.LoadBalancePolicyRoundRobin, proxies.LoadBalancePolicyRandom,
		proxies.LoadBalancePolicyWeightedRandom, proxies.LoadBalancePolicyIPHash,
		proxies.LoadBalancePolicyHeaderHash:
	default:
		return fmt.Errorf("invalid proxy-load-balance: %s", proxyLoadBalance)
	}

	var maxSize int64 = -1
	if proxyServerMaxSize != "" {
		result, err := strconv.ParseInt(proxyServerMaxSize, 0, 64)
		if err != nil {
			return fmt.Errorf("invalid proxy-server-max-size: %v", err)
		}
		maxSize = result
	}

	var timeout time.Duration
	if proxyTimeout != "" {
		result, err := time.ParseDuration(proxyTimeout)
		if err != nil {
			return fmt.Errorf("invalid proxy-timeout: %v", err)
		}
		timeout = result
	}

	pool := &proxy.ServerPoolSpec{
		BaseServerPoolSpec: proxy.BaseServerPoolSpec{
			LoadBalance: &proxy.LoadBalanceSpec{
				Policy:        proxyLoadBalance,
				HeaderHashKey: proxyHeaderHashKey,
				ForwardKey:    proxyForwardKey,
			},
		},
		ServerMaxBodySize: -1,
	}

	for _, ep := range endpoints {
		pool.Servers = append(pool.Servers, &proxy.Server{URL: ep})
	}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":              proxy.Kind,
		"name":              name,
		"pools":             []*proxy.ServerPoolSpec{pool},
		"serverMaxBodySize": maxSize,
		"timeout":           timeout,
	})

	return nil
}

func (b *pipelineSpecBuilder) addWebSocketProxy(endpoints []string, clntMaxMsgSize, svrMaxMsgSize int64, insecure bool, originPatterns []string) {
	const name = "websocketproxy"

	pool := &proxy.WebSocketServerPoolSpec{
		BaseServerPoolSpec: proxy.BaseServerPoolSpec{
			LoadBalance: &proxy.LoadBalanceSpec{},
		},
		ClientMaxMsgSize:   clntMaxMsgSize,
		ServerMaxMsgSize:   svrMaxMsgSize,
		InsecureSkipVerify: insecure,
		OriginPatterns:     originPatterns,
	}

	for _, ep := range endpoints {
		pool.Servers = append(pool.Servers, &proxy.Server{URL: ep})
	}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":  proxy.WebSocketProxyKind,
		"name":  name,
		"pools": []*proxy.WebSocketServerPoolSpec{pool},
	})
}

func (b *pipelineSpecBuilder) jsonConfig() string {
	buff, err := codectool.MarshalJSON(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", b, err)
	}
	return string(buff)
}

func newHTTPServerSpecBuilder(template *httpserver.Spec) *httpServerSpecBuilder {
	return &httpServerSpecBuilder{
		Kind: httpserver.Kind,
		Name: "http-server-ingress-controller",
		Spec: httpserver.Spec{
			Port:             template.Port,
			KeepAlive:        template.KeepAlive,
			HTTPS:            template.HTTPS,
			KeepAliveTimeout: template.KeepAliveTimeout,
			MaxConnections:   template.MaxConnections,
		},
	}
}

func (b *httpServerSpecBuilder) jsonConfig() string {
	buff, err := codectool.MarshalJSON(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", b, err)
	}
	return string(buff)
}

func newSpecTranslator(k8sClient *k8sClient, ingressClass string, httpSvrCfg *httpserver.Spec) *specTranslator {
	return &specTranslator{
		k8sClient:    k8sClient,
		httpSvrCfg:   httpSvrCfg,
		ingressClass: ingressClass,
		pipelines:    map[string]*supervisor.Spec{},
	}
}

func (st *specTranslator) httpServerSpec() *supervisor.Spec {
	return st.httpSvr
}

func (st *specTranslator) pipelineSpecs() map[string]*supervisor.Spec {
	return st.pipelines
}

func (st *specTranslator) getEndpoints(namespace string, service *apinetv1.IngressServiceBackend) ([]string, error) {
	svc, err := st.k8sClient.getService(namespace, service.Name)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, fmt.Errorf("service %s/%s does not exist", namespace, service.Name)
	}

	var svcPort *apicorev1.ServicePort
	for _, p := range svc.Spec.Ports {
		if p.Port == service.Port.Number || (len(p.Name) > 0 && p.Name == service.Port.Name) {
			svcPort = &p
			break
		}
	}
	if svcPort == nil {
		return nil, fmt.Errorf("service port in service %s/%s", namespace, service.Name)
	}
	protocol := "http"
	if svcPort.Port == 443 {
		protocol = "https"
	}

	var result []string
	if svc.Spec.Type == apicorev1.ServiceTypeExternalName {
		hostPort := net.JoinHostPort(svc.Spec.ExternalName, strconv.Itoa(int(svcPort.Port)))
		ep := fmt.Sprintf("%s://%s", protocol, hostPort)
		result = append(result, ep)
		return result, nil
	}

	endpoints, err := st.k8sClient.getEndpoints(namespace, service.Name)
	if err != nil {
		return nil, err
	}

	for _, subset := range endpoints.Subsets {
		var port int32
		for _, p := range subset.Ports {
			if svcPort.Name == p.Name {
				port = p.Port
			}
		}
		if port == 0 {
			continue
		}

		for _, addr := range subset.Addresses {
			hostPort := net.JoinHostPort(addr.IP, strconv.Itoa(int(port)))
			ep := fmt.Sprintf("%s://%s", protocol, hostPort)
			result = append(result, ep)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("service %s/%s endpoint not found", namespace, service.Name)
	}
	return result, nil
}

func supportWebSocket(ingress *apinetv1.Ingress) bool {
	v := ingress.Annotations["easegress.ingress.kubernetes.io/websocket"]
	if len(v) == 0 {
		return false
	}
	b, _ := strconv.ParseBool(v)
	return b
}

func (st *specTranslator) serviceToPipeline(ingress *apinetv1.Ingress, service *apinetv1.IngressServiceBackend) (*supervisor.Spec, error) {
	if service == nil || len(service.Name) == 0 {
		err := fmt.Errorf("invalid service name, ingress backend is object ref")
		logger.Errorf("%v", err)
		return nil, err
	}

	port := service.Port.Name
	if len(port) == 0 {
		port = strconv.Itoa(int(service.Port.Number))
	}
	pipelineName := fmt.Sprintf("pipeline-%s-%s-%s", ingress.Namespace, service.Name, port)
	ws := supportWebSocket(ingress)
	if ws {
		pipelineName += "-ws"
	}
	if st.pipelines[pipelineName] != nil {
		return st.pipelines[pipelineName], nil
	}

	endpoints, err := st.getEndpoints(ingress.Namespace, service)
	if err != nil {
		logger.Errorf("failed to get service endpoints: %v", err)
		return nil, err
	}

	builder := newPipelineSpecBuilder(pipelineName)
	if ws {
		val := ingress.Annotations["easegress.ingress.kubernetes.io/websocket-client-max-msg-size"]
		clntMaxMsgSize, _ := strconv.ParseInt(val, 0, 64)

		val = ingress.Annotations["easegress.ingress.kubernetes.io/websocket-server-max-msg-size"]
		svrMaxMsgSize, _ := strconv.ParseInt(val, 0, 64)

		val = ingress.Annotations["easegress.ingress.kubernetes.io/websocket-insecure-skip-verify"]
		insecure, _ := strconv.ParseBool(val)

		val = ingress.Annotations["easegress.ingress.kubernetes.io/websocket-origin-patterns"]
		originPatterns := strings.Split(val, ",")

		builder.addWebSocketProxy(endpoints, clntMaxMsgSize, svrMaxMsgSize, insecure, originPatterns)
	} else {
		err := builder.addProxy(endpoints, ingress)
		if err != nil {
			return nil, fmt.Errorf("failed to add proxy: %v", err)
		}
	}

	spec, err := supervisor.NewSpec(builder.jsonConfig())
	if err != nil {
		logger.Errorf("failed to generate pipeline spec: %v", err)
		return nil, err
	}

	st.pipelines[pipelineName] = spec
	return spec, err
}

func (st *specTranslator) translateDefaultPipeline(ingress *apinetv1.Ingress) error {
	if st.pipelines[defaultPipelineName] != nil {
		err := fmt.Errorf("the default pipeline has already been created")
		logger.Errorf("%v", err)
		return err
	}

	endpoints, err := st.getEndpoints(ingress.Namespace, ingress.Spec.DefaultBackend.Service)
	if err != nil {
		logger.Errorf("failed to get service endpoints: %v", err)
		return err
	}

	builder := newPipelineSpecBuilder(defaultPipelineName)
	err = builder.addProxy(endpoints, ingress)
	if err != nil {
		return fmt.Errorf("failed to add proxy: %v", err)
	}

	spec, err := supervisor.NewSpec(builder.jsonConfig())
	if err != nil {
		logger.Errorf("failed to generate pipeline spec: %v", err)
		return err
	}

	st.pipelines[defaultPipelineName] = spec
	return nil
}

func getCertificateBlocks(secret *apicorev1.Secret, namespace, secretName string) (string, string, error) {
	crt, ok := secret.Data["tls.crt"]
	if !ok || len(crt) == 0 {
		err := fmt.Errorf("'tls.crt' is missing or empty in secret %s/%s", namespace, secretName)
		return "", "", err
	}

	key, ok := secret.Data["tls.key"]
	if !ok || len(key) == 0 {
		err := fmt.Errorf("'tls.crt' is missing or empty in secret %s/%s", namespace, secretName)
		return "", "", err
	}

	return string(crt), string(key), nil
}

func (st *specTranslator) translateTLSConfig(b *httpServerSpecBuilder, ingresses []*apinetv1.Ingress) error {
	b.Certs = map[string]string{}
	b.Keys = map[string]string{}

	for _, ingress := range ingresses {
		for _, t := range ingress.Spec.TLS {
			if len(t.SecretName) == 0 {
				continue
			}
			cfgKey := ingress.Namespace + "/" + t.SecretName
			if _, ok := b.Certs[cfgKey]; ok {
				continue
			}

			secret, err := st.k8sClient.getSecret(ingress.Namespace, t.SecretName)
			if err != nil {
				logger.Errorf("failed to get secret %s%s: %v", ingress.Namespace, t.SecretName, err)
				continue
			}
			if secret == nil {
				logger.Errorf("secret %s/%s does not exist", ingress.Namespace, t.SecretName)
				continue
			}

			cert, key, err := getCertificateBlocks(secret, ingress.Namespace, t.SecretName)
			if err != nil {
				continue
			}

			b.Certs[cfgKey] = cert
			b.Keys[cfgKey] = key
		}
	}

	return nil
}

func (st *specTranslator) translateIngressRules(b *httpServerSpecBuilder, ingress *apinetv1.Ingress) {
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		r := &routers.Rule{}
		for _, path := range rule.HTTP.Paths {
			pipeline, err := st.serviceToPipeline(ingress, path.Backend.Service)
			if err != nil {
				continue
			}

			p := routers.Path{Backend: pipeline.Name()}
			if path.PathType != nil && *path.PathType == apinetv1.PathTypeExact {
				p.Path = path.Path
			} else {
				p.PathPrefix = path.Path
			}

			p.RewriteTarget = ingress.Annotations["easegress.ingress.kubernetes.io/rewrite-target"]
			p.ClientMaxBodySize = -1

			r.Paths = append(r.Paths, &p)
		}

		if len(r.Paths) == 0 {
			continue
		}

		var existingRule *routers.Rule
		if len(rule.Host) > 0 && rule.Host[0] == '*' {
			host := strings.ReplaceAll(rule.Host[1:], ".", "\\.")
			r.HostRegexp = fmt.Sprintf("^[^.]+%s$", host)
			for _, r1 := range b.Rules {
				if strings.EqualFold(r.HostRegexp, r1.HostRegexp) {
					existingRule = r1
					break
				}
			}
		} else {
			r.Host = rule.Host
			for _, r1 := range b.Rules {
				if strings.EqualFold(r.Host, r1.Host) {
					existingRule = r1
					break
				}
			}
		}

		if existingRule == nil {
			b.Rules = append(b.Rules, r)
		} else {
			existingRule.Paths = append(existingRule.Paths, r.Paths...)
		}
	}
}

func (st *specTranslator) translate() error {
	b := newHTTPServerSpecBuilder(st.httpSvrCfg)

	ingresses := st.k8sClient.getIngresses(st.ingressClass)

	if st.httpSvrCfg.HTTPS {
		st.translateTLSConfig(b, ingresses)
	}

	for _, ingress := range ingresses {
		if ingress.Spec.DefaultBackend != nil {
			st.translateDefaultPipeline(ingress)
		}
		st.translateIngressRules(b, ingress)
	}

	// sort rules by host
	// * precise hosts first(in alphabetical order)
	// * wildcard hosts next(in alphabetical order)
	// * empty host last
	sort.Slice(b.Rules, func(i, j int) bool {
		r1, r2 := b.Rules[i], b.Rules[j]

		if r1.Host != "" {
			if r2.Host == "" {
				return true
			}
			return r1.Host < r2.Host
		}
		if r2.Host != "" {
			return false
		}

		if r1.HostRegexp == "" {
			return false
		}
		if r2.HostRegexp == "" {
			return true
		}
		return r1.HostRegexp < r2.HostRegexp
	})

	if p := st.pipelines[defaultPipelineName]; p != nil {
		r := b.Rules[len(b.Rules)-1]
		if r.Host != "" || r.HostRegexp != "" {
			r = &routers.Rule{}
			b.Rules = append(b.Rules, r)
		}
		r.Paths = append(r.Paths, &routers.Path{
			Backend:    defaultPipelineName,
			PathPrefix: "/",
		})
	}

	// sort path:
	// * precise path first
	// * longer prefix first
	for _, r := range b.Rules {
		sort.Slice(r.Paths, func(i, j int) bool {
			p1, p2 := r.Paths[i], r.Paths[j]
			switch {
			case p1.Path != "" && p2.Path != "":
				return p1.Path < p2.Path
			case p1.Path != "" && p2.Path == "":
				return false
			case p1.Path == "" && p2.Path != "":
				return true
			default: // p1.Path == "" && p2.Path == "":
				return len(p1.PathPrefix) > len(p2.PathPrefix)
			}
		})
	}

	jsonConfig := b.jsonConfig()
	spec, e := supervisor.NewSpec(jsonConfig)
	if e != nil {
		return e
	}

	st.httpSvr = spec
	return nil
}

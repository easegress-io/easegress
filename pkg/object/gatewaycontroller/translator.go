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

package gatewaycontroller

import (
	"fmt"
	"strings"

	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/builder"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	redirector "github.com/megaease/easegress/v2/pkg/filters/redirectorv2"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/pathadaptor"
	"golang.org/x/exp/slices"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapis "sigs.k8s.io/gateway-api/apis/v1"
)

// specTranslator translates k8s gateway related specs to Easegress
// http server spec and pipeline specs
type specTranslator struct {
	k8sClient *k8sClient
	httpsvrs  []*httpServerSpecBuilder
	pipelines []*pipelineSpecBuilder
	routes    []*gwapis.HTTPRoute
}

type pipelineSpecBuilder struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	pipeline.Spec `json:",inline"`

	filters     []map[string]interface{}     `json:"-"`
	resilience  []map[string]interface{}     `json:"-"`
	reqAdaptor  *builder.RequestAdaptorSpec  `json:"-"`
	redirector  *redirector.Spec             `json:"-"`
	respAdaptor *builder.ResponseAdaptorSpec `json:"-"`
	proxy       *httpproxy.Spec              `json:"-"`
}

type httpServerSpecBuilder struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	httpserver.Spec `json:",inline"`
}

func newPipelineSpecBuilder(name string) *pipelineSpecBuilder {
	return &pipelineSpecBuilder{
		Kind:       pipeline.Kind,
		Name:       name,
		Spec:       pipeline.Spec{},
		filters:    make([]map[string]interface{}, 0),
		resilience: []map[string]interface{}{},
	}
}

func (b *pipelineSpecBuilder) jsonConfig() string {
	if len(b.filters) > 0 {
		for _, f := range b.filters {
			kind := f["kind"]
			if kind == builder.ResponseAdaptorKind || kind == builder.ResponseBuilderKind {
				continue
			}
			b.Filters = append(b.Filters, f)
			b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: f["name"].(string)})
		}
	}

	if b.reqAdaptor != nil {
		b.reqAdaptor.BaseSpec.MetaSpec.Name = "requestAdaptor"
		b.reqAdaptor.BaseSpec.MetaSpec.Kind = builder.RequestAdaptorKind
		buf, _ := codectool.MarshalJSON(b.reqAdaptor)
		m := map[string]any{}
		codectool.UnmarshalJSON(buf, &m)
		b.Filters = append(b.Filters, m)
		b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.reqAdaptor.Name()})
	}

	if b.redirector != nil {
		b.redirector.BaseSpec.MetaSpec.Name = "redirector"
		b.redirector.BaseSpec.MetaSpec.Kind = redirector.Kind
		buf, _ := codectool.MarshalJSON(b.redirector)
		m := map[string]any{}
		codectool.UnmarshalJSON(buf, &m)
		b.Filters = append(b.Filters, m)
		b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.redirector.Name()})
	}

	if b.proxy != nil {
		b.proxy.BaseSpec.MetaSpec.Name = "proxy"
		b.proxy.BaseSpec.MetaSpec.Kind = httpproxy.Kind
		for i := range b.proxy.Pools {
			for _, r := range b.resilience {
				if r["kind"] == resilience.CircuitBreakerKind.Name {
					b.proxy.Pools[i].CircuitBreakerPolicy = r["name"].(string)
				} else if r["kind"] == resilience.RetryKind.Name {
					b.proxy.Pools[i].RetryPolicy = r["name"].(string)
				}
			}
		}
		buf, _ := codectool.MarshalJSON(b.proxy)
		m := map[string]any{}
		codectool.UnmarshalJSON(buf, &m)
		b.Filters = append(b.Filters, m)
		b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.proxy.Name()})
	}

	if b.respAdaptor != nil {
		b.respAdaptor.BaseSpec.MetaSpec.Name = "responseAdaptor"
		b.respAdaptor.BaseSpec.MetaSpec.Kind = builder.ResponseAdaptorKind
		buf, _ := codectool.MarshalJSON(b.respAdaptor)
		m := map[string]any{}
		codectool.UnmarshalJSON(buf, &m)
		b.Filters = append(b.Filters, m)
		b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.respAdaptor.Name()})
	}

	if len(b.filters) > 0 {
		for _, f := range b.filters {
			kind := f["kind"]
			if kind == builder.ResponseAdaptorKind || kind == builder.ResponseBuilderKind {
				b.Filters = append(b.Filters, f)
				b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: f["name"].(string)})
			}
		}
	}

	b.Resilience = b.resilience

	buf, err := codectool.MarshalJSON(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", b, err)
	}
	return string(buf)
}

func (b *pipelineSpecBuilder) addExtensionRef(spec *FilterSpecFromCR) {
	data := map[string]interface{}{}
	err := codectool.UnmarshalYAML([]byte(spec.Spec), &data)
	if err != nil {
		logger.Errorf("unmarshal filter spec %v failed: %v", spec, err)
		return
	}
	data["name"] = spec.Name
	data["kind"] = spec.Kind

	if spec.Kind == "CircuitBreaker" || spec.Kind == "Retry" {
		b.resilience = append(b.resilience, data)
		return
	}

	kind := filters.GetKind(spec.Kind)
	if kind == nil {
		logger.Errorf("unknown filter kind %s in extensionRef", spec.Kind)
		return
	}
	b.filters = append(b.filters, data)
}

func (b *pipelineSpecBuilder) addRequestHeaderModifier(f *gwapis.HTTPHeaderFilter) {
	if b.reqAdaptor == nil {
		b.reqAdaptor = &builder.RequestAdaptorSpec{}
	}

	header := &httpheader.AdaptSpec{
		Add: map[string]string{},
		Set: map[string]string{},
		Del: []string{},
	}
	for i := range f.Add {
		h := &f.Add[i]
		header.Add[string(h.Name)] = h.Value
	}
	for i := range f.Set {
		h := &f.Set[i]
		header.Set[string(h.Name)] = h.Value
	}
	header.Del = f.Remove

	b.reqAdaptor.Header = header
}

func (b *pipelineSpecBuilder) addURLRewrite(f *gwapis.HTTPURLRewriteFilter) {
	if b.reqAdaptor == nil {
		b.reqAdaptor = &builder.RequestAdaptorSpec{}
	}

	if f.Hostname != nil {
		b.reqAdaptor.Host = string(*f.Hostname)
	}

	if f.Path == nil {
		return
	}

	if f.Path.Type == gwapis.FullPathHTTPPathModifier {
		b.reqAdaptor.Path = &pathadaptor.Spec{Replace: *f.Path.ReplaceFullPath}
	} else {
		b.reqAdaptor.Path = &pathadaptor.Spec{TrimPrefix: *f.Path.ReplacePrefixMatch}
	}
}

func (b *pipelineSpecBuilder) addRequestRedirect(f *gwapis.HTTPRequestRedirectFilter) {
	b.redirector.HTTPRequestRedirectFilter = *f
}

func (b *pipelineSpecBuilder) addRequestMirror(addr string) {
	if b.proxy == nil {
		b.proxy = &httpproxy.Spec{}
	}

	spec := &httpproxy.ServerPoolSpec{
		Filter: &httpproxy.RequestMatcherSpec{
			RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
				Policy: "random",
				Permil: 1000, // set to 1000 to send all requests to the mirror
			},
		},
		ServerMaxBodySize: -1,
	}
	spec.Servers = append(spec.Servers, &proxies.Server{URL: addr})

	b.proxy.Pools = append(b.proxy.Pools, spec)
}

func (b *pipelineSpecBuilder) addResponseHeaderModifier(f *gwapis.HTTPHeaderFilter) {
	if b.respAdaptor == nil {
		b.respAdaptor = &builder.ResponseAdaptorSpec{}
	}

	header := &httpheader.AdaptSpec{
		Add: map[string]string{},
		Set: map[string]string{},
		Del: []string{},
	}
	for i := range f.Add {
		h := &f.Add[i]
		header.Add[string(h.Name)] = h.Value
	}
	for i := range f.Set {
		h := &f.Set[i]
		header.Set[string(h.Name)] = h.Value
	}
	header.Del = f.Remove

	b.respAdaptor.Header = header
}

func (b *pipelineSpecBuilder) addBackend(addr string, weight int) {
	if b.proxy == nil {
		b.proxy = &httpproxy.Spec{}
		b.proxy.Pools = append(b.proxy.Pools, &httpproxy.ServerPoolSpec{ServerMaxBodySize: -1})
	}

	spec := b.proxy.Pools[len(b.proxy.Pools)-1]
	if spec.Filter != nil {
		spec = &httpproxy.ServerPoolSpec{ServerMaxBodySize: -1}
		b.proxy.Pools = append(b.proxy.Pools, spec)
	}

	spec.Servers = append(spec.Servers, &proxies.Server{URL: addr, Weight: weight})
}

func newHTTPServerSpecBuilder(name string) *httpServerSpecBuilder {
	return &httpServerSpecBuilder{
		Kind: httpserver.Kind,
		Name: name,
		Spec: httpserver.Spec{
			ClientMaxBodySize: -1,
			MaxConnections:    10240,
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

func newSpecTranslator(k8sClient *k8sClient) *specTranslator {
	return &specTranslator{
		k8sClient: k8sClient,
		httpsvrs:  []*httpServerSpecBuilder{},
		pipelines: []*pipelineSpecBuilder{},
	}
}

func (st *specTranslator) httpServerSpecs() map[string]*supervisor.Spec {
	specs := make(map[string]*supervisor.Spec)

	for _, sb := range st.httpsvrs {
		cfg := sb.jsonConfig()
		spec, err := supervisor.NewSpec(cfg)
		if err != nil {
			logger.Errorf("failed to build http server spec: %v", err)
		} else {
			specs[spec.Name()] = spec
		}
	}

	return specs
}

func (st *specTranslator) pipelineSpecs() map[string]*supervisor.Spec {
	specs := make(map[string]*supervisor.Spec)

	for _, sb := range st.pipelines {
		cfg := sb.jsonConfig()
		logger.Debugf("pipeline spec: %s", cfg)
		spec, err := supervisor.NewSpec(cfg)
		if err != nil {
			logger.Errorf("failed to build pipeline spec: %v", err)
		} else {
			specs[spec.Name()] = spec
		}
	}

	return specs
}

func getCertificateBlocks(secret *apicorev1.Secret, namespace, name string) (string, string, error) {
	crt, ok := secret.Data["tls.crt"]
	if !ok || len(crt) == 0 {
		err := fmt.Errorf("'tls.crt' is missing or empty in secret %s/%s", namespace, name)
		return "", "", err
	}

	key, ok := secret.Data["tls.key"]
	if !ok || len(key) == 0 {
		err := fmt.Errorf("'tls.crt' is missing or empty in secret %s/%s", namespace, name)
		return "", "", err
	}

	return string(crt), string(key), nil
}

func (st *specTranslator) getTLS(g *gwapis.Gateway, certRef *gwapis.SecretObjectReference) (string, string, error) {
	if certRef.Kind == nil || *certRef.Kind != "Secret" || certRef.Group == nil || (*certRef.Group != "" && *certRef.Group != "core") {
		err := fmt.Errorf("invalid certificateRef: gateway=%s, certRef=%s", g.Name, certRef.Name)
		return "", "", err
	}

	if certRef.Namespace != nil && string(*certRef.Namespace) != g.Namespace {
		err := fmt.Errorf("cross namespace support of reference policy is not supported currently")
		return "", "", err
	}

	secret, err := st.k8sClient.getSecret(g.Namespace, string(certRef.Name))
	if err != nil {
		err := fmt.Errorf("failed to fetch secret %s/%s: %w", g.Namespace, certRef.Name, err)
		return "", "", err
	}

	return getCertificateBlocks(secret, g.Namespace, string(certRef.Name))
}

func matchHostname(l *gwapis.Listener, hostnames []gwapis.Hostname) []string {
	result := []string{}
	if l.Hostname == nil || *l.Hostname == "" {
		for _, h := range hostnames {
			result = append(result, string(h))
		}
		return result
	}

	if len(hostnames) == 0 {
		result = append(result, string(*l.Hostname))
		return result
	}

	lparts := strings.Split(string(*l.Hostname), ".")

	for _, hostname := range hostnames {
		if hostname == *l.Hostname {
			result = append(result, string(hostname))
			continue
		}

		hparts := strings.Split(string(hostname), ".")
		if len(lparts) != len(hparts) {
			continue
		}

		if !slices.Equal(lparts[1:], hparts[1:]) {
			continue
		}

		if lparts[0] == "*" || hparts[0] == "*" {
			result = append(result, string(hostname))
			continue
		}
	}

	return result
}

func (st *specTranslator) translateRouteRule(dst *routers.Rule, src *gwapis.HTTPRouteRule, backend string) {
	for i := range src.Matches {
		m := &src.Matches[i]
		p := routers.Path{MatchAllHeader: true, MatchAllQuery: true, Backend: backend}

		if m.Path != nil {
			v := "/"
			if m.Path.Value != nil {
				v = *m.Path.Value
			}
			var t gwapis.PathMatchType
			if m.Path.Type != nil {
				t = *m.Path.Type
			}

			switch t {
			case gwapis.PathMatchExact:
				p.Path = v

			case "", gwapis.PathMatchPathPrefix:
				p.PathPrefix = v

			case gwapis.PathMatchRegularExpression:
				p.PathRegexp = v

			default:
				logger.Errorf("unknow path match type: %v", t)
				return
			}
		}

		for j := range m.Headers {
			h := &m.Headers[j]
			var t gwapis.HeaderMatchType
			if h.Type != nil {
				t = *h.Type
			}
			switch t {
			case "", gwapis.HeaderMatchExact:
				p.Headers = append(p.Headers, &routers.Header{
					Key:    string(h.Name),
					Values: []string{h.Value},
				})
			case gwapis.HeaderMatchRegularExpression:
				p.Headers = append(p.Headers, &routers.Header{
					Key:    string(h.Name),
					Regexp: h.Value,
				})
			default:
				logger.Errorf("unknown header match type: %v", t)
			}
		}

		for j := range m.QueryParams {
			q := &m.QueryParams[j]
			var t gwapis.QueryParamMatchType
			if q.Type != nil {
				t = *q.Type
			}
			switch t {
			case "", gwapis.QueryParamMatchExact:
				p.Queries = append(p.Queries, &routers.Query{
					Key:    string(q.Name),
					Values: []string{q.Value},
				})
			case gwapis.QueryParamMatchRegularExpression:
				p.Queries = append(p.Queries, &routers.Query{
					Key:    string(q.Name),
					Regexp: q.Value,
				})
			default:
				logger.Errorf("unknown query param match type: %v", t)
			}
		}

		if m.Method != nil {
			p.Methods = []string{string(*m.Method)}
		}

		dst.Paths = append(dst.Paths, &p)
	}
}

func (st *specTranslator) loadService(ns string, bor *gwapis.BackendObjectReference) string {
	if bor.Group == nil || bor.Kind == nil {
		return ""
	}

	if *bor.Group != "" && *bor.Group != "core" && *bor.Kind != "Service" {
		return ""
	}

	svc, err := st.k8sClient.getService(ns, string(bor.Name))
	if err != nil {
		return ""
	}

	p := &svc.Spec.Ports[0]
	if bor.Port != nil {
		for i := range svc.Spec.Ports {
			port := &svc.Spec.Ports[i]
			if port.Port == int32(*bor.Port) {
				p = port
				break
			}
		}
	}

	if p.Port == 80 {
		return fmt.Sprintf("http://%s.%s", svc.Name, svc.Namespace)
	} else if p.Port == 443 {
		return fmt.Sprintf("https://%s.%s", svc.Name, svc.Namespace)
	} else {
		return fmt.Sprintf("http://%s.%s:%d", svc.Name, svc.Namespace, p.Port)
	}
}

func (st *specTranslator) translatePipeline(ns, name string, r *gwapis.HTTPRouteRule) {
	sb := newPipelineSpecBuilder(name)

	for i := range r.Filters {
		f := &r.Filters[i]
		switch f.Type {
		case gwapis.HTTPRouteFilterRequestHeaderModifier:
			sb.addRequestHeaderModifier(f.RequestHeaderModifier)
		case gwapis.HTTPRouteFilterURLRewrite:
			sb.addURLRewrite(f.URLRewrite)
		case gwapis.HTTPRouteFilterRequestRedirect:
			sb.addRequestRedirect(f.RequestRedirect)
		case gwapis.HTTPRouteFilterRequestMirror:
			addr := st.loadService(ns, &f.RequestMirror.BackendRef)
			if addr != "" {
				sb.addRequestMirror(addr)
			}
		case gwapis.HTTPRouteFilterResponseHeaderModifier:
			sb.addResponseHeaderModifier(f.ResponseHeaderModifier)
		case gwapis.HTTPRouteFilterExtensionRef:
			g := string(f.ExtensionRef.Group)
			if g != FilterSpecGVR.Group {
				logger.Errorf("extension group %s is not supported, only support %s", g, FilterSpecGVR.Group)
				continue
			}
			filterSpec, err := st.k8sClient.GetFilterSpecFromCustomResource(ns, string(f.ExtensionRef.Name))
			if err != nil {
				logger.Errorf("failed to get filter spec %s/%s/%s, %v", ns, FilterSpecGVR.Group, f.ExtensionRef.Name, err)
				continue
			}
			sb.addExtensionRef(filterSpec)
		}
	}

	for i := range r.BackendRefs {
		b := &r.BackendRefs[i]
		addr := st.loadService(ns, &b.BackendObjectReference)
		w := 1
		if b.Weight != nil {
			w = int(*b.Weight)
		}
		sb.addBackend(addr, w)
	}

	st.pipelines = append(st.pipelines, sb)
}

func (st *specTranslator) translateHTTPListener(g *gwapis.Gateway, l *gwapis.Listener) {
	sb := newHTTPServerSpecBuilder(fmt.Sprintf("http-server-%s-%s", g.Name, l.Name))
	sb.Port = uint16(l.Port)

	if l.Protocol == gwapis.HTTPSProtocolType {
		sb.HTTPS = true
	}

	if l.TLS != nil && len(l.TLS.CertificateRefs) > 0 {
		certRef := &l.TLS.CertificateRefs[0]
		cert, key, err := st.getTLS(g, certRef)
		if err != nil {
			logger.Errorf(err.Error())
			return
		}
		cfgKey := g.Namespace + "/" + string(certRef.Name)
		sb.Certs[cfgKey] = cert
		sb.Keys[cfgKey] = key
	} else if sb.HTTPS {
		logger.Errorf("%s/%s: https protocol is used without certificate", g.Name, l.Name)
		return
	}

	for _, route := range st.routes {
		shouldAttach := false
		for _, pr := range route.Spec.ParentRefs {
			if pr.Group != nil && *pr.Group != gwapis.GroupName {
				continue
			}
			if pr.Kind != nil && *pr.Kind != "Gateway" {
				continue
			}
			if pr.Name != gwapis.ObjectName(g.Name) {
				continue
			}

			if pr.SectionName != nil && *pr.SectionName != l.Name {
				continue
			}
			if pr.Port != nil && pr.Port != &l.Port {
				continue
			}
			ns := route.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if ns == g.Namespace && string(pr.Name) == g.Name {
				shouldAttach = true
				break
			}
		}
		if !shouldAttach {
			continue
		}

		hostnames := matchHostname(l, route.Spec.Hostnames)
		if len(hostnames) == 0 {
			if l.Hostname != nil && *l.Hostname != "" && len(route.Spec.Hostnames) > 0 {
				continue
			}
		}

		rule := &routers.Rule{}

		for _, hostname := range hostnames {
			isRegexp := false
			if hostname[0] == '*' {
				isRegexp = true
				hostname = `^[^.]+\.` + hostname[1:] + "$"
			}
			rule.Hosts = append(rule.Hosts, routers.Host{IsRegexp: isRegexp, Value: hostname})
		}

		for i := range route.Spec.Rules {
			pipelineName := fmt.Sprintf("pipeline-%s-%s-%d", g.Name, l.Name, i)
			r := &route.Spec.Rules[i]
			st.translateRouteRule(rule, r, pipelineName)
			st.translatePipeline(route.Namespace, pipelineName, r)
		}

		sb.Rules = append(sb.Rules, rule)
	}

	st.httpsvrs = append(st.httpsvrs, sb)
}

func (st *specTranslator) translateGateway(c *gwapis.GatewayClass, g *gwapis.Gateway) {
	for i := range g.Spec.Listeners {
		l := &g.Spec.Listeners[i]

		switch l.Protocol {
		case gwapis.HTTPProtocolType, gwapis.HTTPSProtocolType:
			st.translateHTTPListener(g, l)
		default:
			logger.Errorf("protocol %v is not supported currently", l.Protocol)
		}
	}
}

func (st *specTranslator) translate() error {
	classes := st.k8sClient.GetGatewayClasses(gatewayControllerName)
	gateways := st.k8sClient.GetGateways()
	st.routes = st.k8sClient.GetHTTPRoutes()

	for _, c := range classes {
		err := st.updateGatewayClassStatus(c)
		if err != nil {
			logger.Errorf("%v", err)
		}
		for _, g := range gateways {
			if string(g.Spec.GatewayClassName) != c.Name {
				continue
			}
			err := st.updateGatewayStatus(c, g)
			if err != nil {
				logger.Errorf("%v", err)
			}
			st.translateGateway(c, g)
		}
	}

	return nil
}

// Conditions:
//
//	Last Transition Time:  1970-01-01T00:00:00Z
//	Message:               Waiting for controller
//	Reason:                Pending
//	Status:                Unknown
//	Type:                  Accepted
//	Last Transition Time:  1970-01-01T00:00:00Z
//	Message:               Waiting for controller
//	Reason:                Pending
//	Status:                Unknown
//	Type:                  Programmed
//
// we need to remove init conditions and add new.
func (st *specTranslator) updateGatewayStatus(c *gwapis.GatewayClass, g *gwapis.Gateway) error {
	compareCondition := func(c1, c2 metav1.Condition) bool {
		return c1.Type == c2.Type && c1.Status == c2.Status && c1.Reason == c2.Reason && c1.Message == c2.Message
	}
	condition := metav1.Condition{
		Type:               string(gwapis.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gwapis.GatewayReasonAccepted),
		Message:            "Gateway is accepted",
		LastTransitionTime: metav1.Now(),
	}

	gateway := g.DeepCopy()
	newConditions := []metav1.Condition{}
	for _, cond := range gateway.Status.Conditions {
		// remove the init condition.
		if cond.Type == string(gwapis.GatewayConditionAccepted) && cond.Status == metav1.ConditionUnknown && cond.Message == "Waiting for controller" {
			continue
		}
		if cond.Type == string(gwapis.GatewayConditionProgrammed) && cond.Status == metav1.ConditionUnknown && cond.Message == "Waiting for controller" {
			continue
		}
		// if we already have the condition, just return.
		if compareCondition(cond, condition) {
			return nil
		}
		newConditions = append(newConditions, cond)
	}
	newConditions = append(newConditions, condition)
	gateway.Status.Conditions = newConditions
	return st.k8sClient.UpdateGatewayStatus(g, gateway.Status)
}

// GatewayClass starts with status:
// Status:
//
//	Conditions:
//	  Last Transition Time:  1970-01-01T00:00:00Z
//	  Message:               Waiting for controller
//	  Reason:                Waiting
//	  Status:                Unknown
//	  Type:                  Accepted
//
// we need to remove init status and add new.
func (st *specTranslator) updateGatewayClassStatus(c *gwapis.GatewayClass) error {
	compareCondition := func(c1, c2 metav1.Condition) bool {
		return c1.Type == c2.Type && c1.Status == c2.Status && c1.Reason == c2.Reason && c1.Message == c2.Message
	}
	condition := metav1.Condition{
		Type:               string(gwapis.GatewayClassConditionStatusAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gwapis.GatewayClassReasonAccepted),
		Message:            "GatewayClass is accepted",
		LastTransitionTime: metav1.Now(),
	}

	gc := c.DeepCopy()
	newConditions := []metav1.Condition{}
	for _, cond := range gc.Status.Conditions {
		// remove the init condition.
		if cond.Type == string(gwapis.GatewayClassConditionStatusAccepted) && cond.Status == metav1.ConditionUnknown && cond.Message == "Waiting for controller" {
			continue
		}
		// if we already have the condition, just return.
		if compareCondition(cond, condition) {
			return nil
		}
		newConditions = append(newConditions, cond)
	}

	newConditions = append(newConditions, condition)
	gc.Status.Conditions = newConditions
	err := st.k8sClient.UpdateGatewayClassStatus(c, gc.Status)
	return err
}

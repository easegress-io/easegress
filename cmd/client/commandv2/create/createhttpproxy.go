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

package create

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/commandv2/specs"
	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/cmd/client/resources"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

// HTTPProxyOptions are the options to create a HTTPProxy.
type HTTPProxyOptions struct {
	Name  string
	Port  int
	Rules []string

	TLS        bool
	AutoCert   bool
	CaCertFile string
	CertFiles  []string
	KeyFiles   []string

	caCert string
	certs  []string
	keys   []string
	rules  []*HTTPProxyRule
}

var httpProxyOptions = &HTTPProxyOptions{}

var httpProxyExamples = `# General case
egctl create httpproxy NAME --port PORT \ 
	--rule HOST/PATH=ENDPOINT1,ENDPOINT2 \
	[--rule HOST/PATH=ENDPOINT1,ENDPOINT2] \
	[--tls] \
	[--auto-cert] \
	[--ca-cert-file CA_CERT_FILE] \
	[--cert-file CERT_FILE] \
	[--key-file KEY_FILE]

# Create a HTTPServer (with port 10080) and corresponding Pipelines to direct 
# request with path "/bar" to "http://127.0.0.1:8080" and "http://127.0.0.1:8081" and 
# request with path "/foo" to "http://127.0.0.1:8082".
egctl create httpproxy demo --port 10080 \
	--rule="/bar=http://127.0.0.1:8080,http://127.0.0.1:8081" \
	--rule="/foo=http://127.0.0.1:8082"

# Create a HTTPServer (with port 10081) and corresponding Pipelines to direct request 
# with path prefix "foo.com/prefix" to "http://127.0.0.1:8083".
egctl create httpproxy demo2 --port 10081 \
	--rule="foo.com/prefix*=http://127.0.0.1:8083"
`

// HTTPProxyCmd returns create command of HTTPProxy.
func HTTPProxyCmd() *cobra.Command {
	o := httpProxyOptions

	cmd := &cobra.Command{
		Use:     "httpproxy NAME",
		Short:   "Create a HTTPServer and corresponding Pipelines with a specific name",
		Args:    httpProxyArgs,
		Example: general.CreateMultiLineExample(httpProxyExamples),
		Run: func(cmd *cobra.Command, args []string) {
			err := httpProxyRun(cmd, args)
			if err != nil {
				general.ExitWithError(err)
			}
		},
	}

	cmd.Flags().IntVar(&o.Port, "port", -1, "Port of HTTPServer")
	cmd.Flags().StringArrayVar(&o.Rules, "rule", []string{}, "Rule in format host/path=endpoint1,endpoint2. Paths containing the leading character '*' are considered as PathPrefix.")

	cmd.Flags().BoolVar(&o.TLS, "tls", false, "Enable TLS")
	cmd.Flags().BoolVar(&o.AutoCert, "auto-cert", false, "Enable auto cert")
	cmd.Flags().StringVar(&o.CaCertFile, "ca-cert-file", "", "CA cert file")
	cmd.Flags().StringArrayVar(&o.CertFiles, "cert-file", []string{}, "Cert file")
	cmd.Flags().StringArrayVar(&o.KeyFiles, "key-file", []string{}, "Key file")
	return cmd
}

func httpProxyArgs(_ *cobra.Command, args []string) error {
	o := httpProxyOptions
	if len(args) != 1 {
		return fmt.Errorf("create httpproxy requires a name")
	}
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("port %d is invalid", o.Port)
	}
	if len(o.Rules) == 0 {
		return fmt.Errorf("rule is required")
	}
	if len(o.CertFiles) != len(o.KeyFiles) {
		return fmt.Errorf("cert files and key files are not matched")
	}
	return nil
}

func httpProxyRun(cmd *cobra.Command, args []string) error {
	o := httpProxyOptions
	o.Complete(args)
	if err := o.Parse(); err != nil {
		return err
	}
	hs, pls := o.Translate()
	allSpec := []interface{}{hs}
	for _, p := range pls {
		allSpec = append(allSpec, p)
	}
	for _, s := range allSpec {
		spec, err := toGeneralSpec(s)
		if err != nil {
			return err
		}
		if err := resources.CreateObject(cmd, spec); err != nil {
			return err
		}
	}
	return nil
}

// Complete completes all the required options.
func (o *HTTPProxyOptions) Complete(args []string) {
	o.Name = args[0]
}

// Parse parses all the optional options.
func (o *HTTPProxyOptions) Parse() error {
	// parse rules
	rules := []*HTTPProxyRule{}
	for _, rule := range o.Rules {
		r, err := parseRule(rule)
		if err != nil {
			return err
		}
		rules = append(rules, r)
	}
	o.rules = rules

	// parse ca cert
	if o.CaCertFile != "" {
		caCert, err := loadCertFile(o.CaCertFile)
		if err != nil {
			return err
		}
		o.caCert = caCert
	}

	// parse certs
	certs := []string{}
	for _, certFile := range o.CertFiles {
		cert, err := loadCertFile(certFile)
		if err != nil {
			return err
		}
		certs = append(certs, cert)
	}
	o.certs = certs

	// parse keys
	keys := []string{}
	for _, keyFile := range o.KeyFiles {
		key, err := loadCertFile(keyFile)
		if err != nil {
			return err
		}
		keys = append(keys, key)
	}
	o.keys = keys
	return nil
}

func (o *HTTPProxyOptions) getServerName() string {
	return o.Name
}

func (o *HTTPProxyOptions) getPipelineName(id int) string {
	return fmt.Sprintf("%s-%d", o.Name, id)
}

// Translate translates HTTPProxyOptions to HTTPServerSpec and PipelineSpec.
func (o *HTTPProxyOptions) Translate() (*specs.HTTPServerSpec, []*specs.PipelineSpec) {
	hs := specs.NewHTTPServerSpec(o.getServerName())
	hs.Port = uint16(o.Port)
	if o.TLS {
		hs.HTTPS = true
		hs.AutoCert = o.AutoCert
		hs.CaCertBase64 = o.caCert
		hs.Certs = map[string]string{}
		hs.Keys = map[string]string{}
		for i := 0; i < len(o.certs); i++ {
			// same key for cert and key
			hs.Certs[o.CertFiles[i]] = o.certs[i]
			hs.Keys[o.CertFiles[i]] = o.keys[i]
		}
	}
	routerRules, pipelines := o.translateRules()
	hs.Rules = routerRules
	return hs, pipelines
}

func (o *HTTPProxyOptions) translateRules() (routers.Rules, []*specs.PipelineSpec) {
	var rules routers.Rules
	var pipelines []*specs.PipelineSpec
	pipelineID := 0

	for _, rule := range o.rules {
		pipelineName := o.getPipelineName(pipelineID)
		pipelineID++

		routerPath := &routers.Path{
			Path:       rule.Path,
			PathPrefix: rule.PathPrefix,
			Backend:    pipelineName,
		}

		pipelineSpec := specs.NewPipelineSpec(pipelineName)
		translateToPipeline(rule.Endpoints, pipelineSpec)
		pipelines = append(pipelines, pipelineSpec)

		l := len(rules)
		if l != 0 && rules[l-1].Host == rule.Host {
			rules[l-1].Paths = append(rules[l-1].Paths, routerPath)
		} else {
			rules = append(rules, &routers.Rule{
				Host:  rule.Host,
				Paths: []*routers.Path{routerPath},
			})
		}
	}
	return rules, pipelines
}

func toGeneralSpec(data interface{}) (*general.Spec, error) {
	var yamlStr []byte
	var err error
	if yamlStr, err = codectool.MarshalYAML(data); err != nil {
		return nil, err
	}

	var spec *general.Spec
	if spec, err = general.GetSpecFromYaml(string(yamlStr)); err != nil {
		return nil, err
	}
	return spec, nil
}

func loadCertFile(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func translateToPipeline(endpoints []string, spec *specs.PipelineSpec) {
	proxy := translateToProxyFilter(endpoints)
	spec.SetFilters([]filters.Spec{proxy})
}

func translateToProxyFilter(endpoints []string) *httpproxy.Spec {
	spec := specs.NewProxyFilterSpec("proxy")

	servers := make([]*proxies.Server, len(endpoints))
	for i, endpoint := range endpoints {
		servers[i] = &proxies.Server{
			URL: endpoint,
		}
	}
	spec.Pools = []*httpproxy.ServerPoolSpec{{
		BaseServerPoolSpec: proxies.ServerPoolBaseSpec{
			Servers: servers,
			LoadBalance: &proxies.LoadBalanceSpec{
				Policy: proxies.LoadBalancePolicyRoundRobin,
			},
		},
	}}
	return spec
}

func parseRule(rule string) (*HTTPProxyRule, error) {
	parts := strings.Split(rule, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rule %s should in format 'host/path=endpoint1,endpoint2', invalid format", rule)
	}

	// host and path
	uri := strings.SplitN(parts[0], "/", 2)
	if len(uri) != 2 {
		return nil, fmt.Errorf("rule %s not contain a path", rule)
	}
	host := uri[0]
	var path, pathPrefix string
	if strings.HasSuffix(uri[1], "*") {
		pathPrefix = "/" + strings.TrimSuffix(uri[1], "*")
	} else {
		path = "/" + uri[1]
	}

	// endpoints
	endpoints := strings.Split(parts[1], ",")
	endpoints = general.Filter(endpoints, func(s string) bool {
		return s != ""
	})
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("endpoints in rule %s is empty", rule)
	}

	return &HTTPProxyRule{
		Host:       host,
		Path:       path,
		PathPrefix: pathPrefix,
		Endpoints:  endpoints,
	}, nil
}

// HTTPProxyRule is the rule of HTTPProxy.
type HTTPProxyRule struct {
	Host       string
	Path       string
	PathPrefix string
	Endpoints  []string
}

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

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/spf13/cobra"
)

type CreateHTTPProxyOptions struct {
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
	rules  []*CreateHTTPProxyRule
}

var createHTTPProxyOptions = &CreateHTTPProxyOptions{}

func CreateHTTPProxyCmd() *cobra.Command {
	o := createHTTPProxyOptions

	cmd := &cobra.Command{
		Use:   "httpproxy NAME",
		Short: "Create a HTTPServer and corresponding Pipelines with a specific name",
		Args:  createHTTPProxyArgs,
		Run: func(cmd *cobra.Command, args []string) {
			o.Complete(args)
			createHTTPProxyRun(cmd, args)
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

func createHTTPProxyArgs(_ *cobra.Command, args []string) error {
	o := createHTTPProxyOptions
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

func createHTTPProxyRun(cmd *cobra.Command, args []string) {
	o := createHTTPProxyOptions
	err := o.Parse()
	if err != nil {
		exitWithError(err)
	}
	hs, pls := o.Translate()
	hsSpec, err := toGeneralSpec(hs)
	if err != nil {
		exitWithError(err)
	}
	if err := createObject(cmd, hsSpec); err != nil {
		exitWithError(err)
	}
	for _, pl := range pls {
		plSpec, err := toGeneralSpec(pl)
		if err != nil {
			exitWithError(err)
		}
		if err := createObject(cmd, plSpec); err != nil {
			exitWithError(err)
		}
	}
}

type HTTPServerSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`

	httpserver.Spec `json:",inline"`
}

type PipelineSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`

	pipeline.Spec `json:",inline"`
}

func (o *CreateHTTPProxyOptions) Complete(args []string) {
	o.Name = args[0]
}

func (o *CreateHTTPProxyOptions) Parse() error {
	// parse rules
	rules := []*CreateHTTPProxyRule{}
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

func (o *CreateHTTPProxyOptions) getServerName() string {
	return o.Name + "-server"
}

func (o *CreateHTTPProxyOptions) getPipelineName(id int) string {
	return fmt.Sprintf("%s-pipeline-%d", o.Name, id)
}

func (o *CreateHTTPProxyOptions) Translate() (*HTTPServerSpec, []*PipelineSpec) {
	hs := &HTTPServerSpec{
		Name: o.getServerName(),
		Kind: httpserver.Kind,
		Spec: *((&httpserver.HTTPServer{}).DefaultSpec()).(*httpserver.Spec),
	}
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

func (o *CreateHTTPProxyOptions) translateRules() (routers.Rules, []*PipelineSpec) {
	var rules routers.Rules
	var pipelines []*PipelineSpec
	pipelineID := 0

	for _, rule := range o.rules {
		pipelineName := o.getPipelineName(pipelineID)
		pipelineID++

		routerPath := &routers.Path{
			Path:       rule.Path,
			PathPrefix: rule.PathPrefix,
			Backend:    pipelineName,
		}
		pipelines = append(pipelines, &PipelineSpec{
			Name: pipelineName,
			Kind: pipeline.Kind,
			Spec: *translateToPipeline(rule.Endpoints),
		})

		l := len(rules)
		if l != 0 && rules[l-1].Host == rule.Host {
			rules[l-1].Paths = append(rules[l-1].Paths, routerPath)
			pipelines = append(pipelines, &PipelineSpec{
				Name: pipelineName,
				Kind: pipeline.Kind,
				Spec: *translateToPipeline(rule.Endpoints),
			})
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
	yamlStr := codectool.MustMarshalYAML(data)
	spec, err := general.GetSpecFromYaml(string(yamlStr))
	if err != nil {
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

func translateToPipeline(endpoints []string) *pipeline.Spec {
	proxy := translateToProxyFilter(endpoints)
	data := codectool.MustMarshalYAML(proxy)
	maps, _ := general.UnmarshalMapInterface(data, false)

	spec := (&pipeline.Pipeline{}).DefaultSpec().(*pipeline.Spec)
	spec.Filters = maps
	return spec
}

func translateToProxyFilter(endpoints []string) *httpproxy.Spec {
	kind := filters.GetKind(httpproxy.Kind)
	spec := kind.DefaultSpec().(*httpproxy.Spec)
	spec.BaseSpec.MetaSpec.Name = "proxy"
	spec.BaseSpec.MetaSpec.Kind = httpproxy.Kind

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

func parseRule(rule string) (*CreateHTTPProxyRule, error) {
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

	return &CreateHTTPProxyRule{
		Host:       host,
		Path:       path,
		PathPrefix: pathPrefix,
		Endpoints:  endpoints,
	}, nil
}

type CreateHTTPProxyRule struct {
	Host       string
	Path       string
	PathPrefix string
	Endpoints  []string
}

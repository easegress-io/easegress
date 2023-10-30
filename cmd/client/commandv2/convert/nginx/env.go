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
	"encoding/json"

	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
)

// Env is the environment for converting.
// Server is the server environment. Used to create HTTPServer or in future GRPCServer.
// Proxy is the proxy environment. Used to create Pipeline.
type Env struct {
	Server   *ServerEnv
	Proxy    *ProxyEnv
	Upstream []*Directive
}

type Directive = crossplane.Directive

// ServerEnv is the environment for creating server.
// Listen contains the listen address, port and protocol.
// ServerName contains the server name (hosts in easegress).
type ServerEnv struct {
	Listen               *Directive
	ServerName           *Directive
	SSLClientCertificate *Directive
	SSLCertificate       []*Directive
	SSLCertificateKey    []*Directive
}

type ProxyEnv struct {
	Kind           string
	Pass           *Directive
	ProxySetHeader []*Directive
}

type HTTPServerInfo struct {
	Port    uint16
	Address string
	Hosts   []routers.Host

	CaCert string
	Certs  map[string]string
	Keys   map[string]string
}

type PipelineInfo struct {
	Servers     []proxies.Server
	LoadBalance *proxies.LoadBalanceSpec
	SetHeaders  map[string]string
}

func (env *Env) Clone() (*Env, error) {
	data, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	var newEnv Env
	err = json.Unmarshal(data, &newEnv)
	if err != nil {
		return nil, err
	}
	return &newEnv, nil
}

func (env *Env) MustClone() *Env {
	newEnv, err := env.Clone()
	if err != nil {
		panic(err)
	}
	return newEnv
}

func NewEnv() *Env {
	return &Env{
		Server: &ServerEnv{
			SSLCertificate:    make([]*Directive, 0),
			SSLCertificateKey: make([]*Directive, 0),
		},
		Proxy: &ProxyEnv{
			ProxySetHeader: make([]*Directive, 0),
		},
		Upstream: make([]*Directive, 0),
	}
}

var nginxBackends = map[string]struct{}{
	"root":           {},
	"return":         {},
	"rewrite":        {},
	"try_files":      {},
	"error_page":     {},
	"proxy_pass":     {},
	"fastcgi_pass":   {},
	"uwsgi_pass":     {},
	"scgi_pass":      {},
	"memcached_pass": {},
	"grpc_pass":      {},
}

func (env *Env) Update(d *Directive) {
	directiveMap := map[string]**Directive{
		"listen":                 &env.Server.Listen,
		"server_name":            &env.Server.ServerName,
		"ssl_client_certificate": &env.Server.SSLClientCertificate,
	}
	if val, ok := directiveMap[d.Directive]; ok {
		*val = d
		return
	}

	arrMap := map[string][]*Directive{
		"ssl_certificate":     env.Server.SSLCertificate,
		"ssl_certificate_key": env.Server.SSLCertificateKey,
		"proxy_set_header":    env.Proxy.ProxySetHeader,
		"upstream":            env.Upstream,
	}
	arr, ok := arrMap[d.Directive]
	if ok {
		arrMap[d.Directive] = append(arr, d)
		return
	}

	_, ok = nginxBackends[d.Directive]
	if ok {
		env.Proxy.Kind = d.Directive
		env.Proxy.Pass = d
	}
}

func (env *Env) Validate() error {
	return nil
}

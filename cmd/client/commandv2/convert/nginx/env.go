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
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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

// Env is the environment for converting.
// Server is the server environment. Used to create HTTPServer or in future GRPCServer.
// Proxy is the proxy environment. Used to create Pipeline.
type Env struct {
	Server   *ServerEnv   `json:"server"`
	Proxy    *ProxyEnv    `json:"proxy"`
	Upstream []*Directive `json:"upstream"`

	updateFn map[string]func(*Directive) `json:"-"`
}

// ServerEnv is the environment for creating server.
// Listen contains the listen address, port and protocol.
// ServerName contains the server name (hosts in easegress).
type ServerEnv struct {
	Listen               *Directive   `json:"listen"`
	ServerName           *Directive   `json:"server_name"`
	SSLClientCertificate *Directive   `json:"ssl_client_certificate"`
	SSLCertificate       []*Directive `json:"ssl_certificate"`
	SSLCertificateKey    []*Directive `json:"ssl_certificate_key"`
}

type ProxyEnv struct {
	Kind           string       `json:"kind"`
	Pass           *Directive   `json:"pass"`
	ProxySetHeader []*Directive `json:"proxy_set_header"`
}

func getEnvUpdateFn(env *Env) map[string]func(*Directive) {
	m := map[string]func(*Directive){
		"listen":                 func(d *Directive) { env.Server.Listen = d },
		"server_name":            func(d *Directive) { env.Server.ServerName = d },
		"ssl_client_certificate": func(d *Directive) { env.Server.SSLClientCertificate = d },
		"ssl_certificate":        func(d *Directive) { env.Server.SSLCertificate = append(env.Server.SSLCertificate, d) },
		"ssl_certificate_key":    func(d *Directive) { env.Server.SSLCertificateKey = append(env.Server.SSLCertificateKey, d) },
		"proxy_set_header":       func(d *Directive) { env.Proxy.ProxySetHeader = append(env.Proxy.ProxySetHeader, d) },
		"upstream":               func(d *Directive) { env.Upstream = append(env.Upstream, d) },
	}
	return m
}

func newEnv() *Env {
	env := &Env{
		Server: &ServerEnv{
			SSLCertificate:    make([]*Directive, 0),
			SSLCertificateKey: make([]*Directive, 0),
		},
		Proxy: &ProxyEnv{
			ProxySetHeader: make([]*Directive, 0),
		},
		Upstream: make([]*Directive, 0),
	}

	env.updateFn = getEnvUpdateFn(env)
	return env
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
	newEnv.updateFn = getEnvUpdateFn(env)
	return &newEnv, nil
}

func (env *Env) MustClone() *Env {
	newEnv, err := env.Clone()
	if err != nil {
		panic(err)
	}
	return newEnv
}

func (env *Env) Update(d *Directive) {
	fn, ok := env.updateFn[d.Directive]
	if ok {
		fn(d)
		return
	}
	_, ok = nginxBackends[d.Directive]
	if ok {
		env.Proxy.Kind = d.Directive
		env.Proxy.Pass = d
	}
}

func (env *Env) GetServerInfo() (*ServerInfo, error) {
	info := &ServerInfo{
		Port:  80,
		Certs: make(map[string]string),
		Keys:  make(map[string]string),
	}

	if env.Server.Listen != nil {
		address, port, https, err := processListen(env.Server.Listen)
		if err != nil {
			return nil, err
		}
		info.Address = address
		info.Port = port
		info.HTTPS = https
	}

	if env.Server.ServerName != nil {
		serverName := env.Server.ServerName
		hosts, err := processServerName(serverName)
		if err != nil {
			return nil, err
		}
		info.Hosts = hosts
	}

	return info, nil
}

func processServerName(d *Directive) ([]HostInfo, error) {
	hosts := make([]HostInfo, 0)
	for _, arg := range d.Args {
		if strings.HasPrefix(arg, "~") {
			_, err := regexp.Compile(arg[1:])
			if err != nil {
				return nil, fmt.Errorf("%s: %v", directiveInfo(d), err)
			}
			hosts = append(hosts, HostInfo{
				Value:    arg[1:],
				IsRegexp: true,
			})
		} else {
			count := strings.Count(arg, "*")
			if count > 1 {
				return nil, fmt.Errorf("%s: host %s contains more than one wildcard", directiveInfo(d), arg)
			}
			if count == 1 {
				if arg[0] != '*' && arg[len(arg)-1] != '*' {
					return nil, fmt.Errorf("%s: host %s contains wildcard in the middle", directiveInfo(d), arg)
				}
			}
			hosts = append(hosts, HostInfo{
				Value:    arg,
				IsRegexp: false,
			})
		}
	}
	return hosts, nil
}

func processListen(d *Directive) (address string, port int, https bool, err error) {
	mustContainArgs(d, 1)
	address, port, err = splitAddressPort(d.Args[0])
	if err != nil {
		return "", 0, false, fmt.Errorf("%s: %v", directiveInfo(d), err)
	}
	for _, arg := range d.Args[1:] {
		if arg == "ssl" {
			return address, port, true, nil
		}
	}
	return address, port, false, nil
}

type ServerInfo struct {
	Port    int
	Address string
	HTTPS   bool
	Hosts   []HostInfo

	CaCert string
	Certs  map[string]string
	Keys   map[string]string
}

type HostInfo struct {
	Value    string
	IsRegexp bool
}

type ProxyInfo struct {
	Servers    []*BackendInfo
	SetHeaders map[string]string
}

type BackendInfo struct {
	Server string
	Weight int
}

// splitAddressPort splits the listen directive into address and port.
// nginx examples:
// listen 127.0.0.1:8000;
// listen 127.0.0.1;
// listen 8000;
// listen *:8000;
// listen localhost:8000;
// listen [::]:8000;
// listen [::1];
func splitAddressPort(listen string) (string, int, error) {
	if listen == "" {
		return "", 0, fmt.Errorf("listen is empty")
	}
	if listen[0] == '[' {
		end := strings.Index(listen, "]")
		if end < 0 {
			return "", 0, fmt.Errorf("invalid listen: %s", listen)
		}
		host := listen[0 : end+1]
		portPart := listen[end+1:]
		if portPart == "" {
			return host, 80, nil
		}
		if portPart[0] != ':' {
			return "", 0, fmt.Errorf("invalid listen: %s", listen)
		}
		portPart = portPart[1:]
		port, err := strconv.Atoi(portPart)
		if err != nil {
			return "", 0, fmt.Errorf("invalid listen: %s", listen)
		}
		return host, port, nil
	}

	index := strings.Index(listen, ":")
	if index < 0 {
		port, err := strconv.Atoi(listen)
		if err != nil {
			return listen, 80, nil
		}
		return "", port, nil
	}
	host := listen[0:index]
	portPart := listen[index+1:]
	if portPart == "" {
		return host, 80, nil
	}
	port, err := strconv.Atoi(portPart)
	if err != nil {
		return "", 0, fmt.Errorf("invalid listen: %s", listen)
	}
	return host, port, nil
}

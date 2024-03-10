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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/general"
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

// ProxyEnv is the environment for creating proxy.
type ProxyEnv struct {
	Pass           *Directive   `json:"pass"`
	ProxySetHeader []*Directive `json:"proxy_set_header"`
	Gzip           *GzipEnv     `json:"gzip"`
}

// GzipEnv is the environment for creating gzip.
type GzipEnv struct {
	Gzip          *Directive `json:"gzip"`
	GzipMinLength *Directive `json:"gzip_min_length"`
}

func (env *Env) init() {
	env.updateFn = map[string]func(*Directive){
		"listen":                 func(d *Directive) { env.Server.Listen = d },
		"server_name":            func(d *Directive) { env.Server.ServerName = d },
		"ssl_client_certificate": func(d *Directive) { env.Server.SSLClientCertificate = d },
		"ssl_certificate":        func(d *Directive) { env.Server.SSLCertificate = append(env.Server.SSLCertificate, d) },
		"ssl_certificate_key":    func(d *Directive) { env.Server.SSLCertificateKey = append(env.Server.SSLCertificateKey, d) },
		"proxy_set_header":       func(d *Directive) { env.Proxy.ProxySetHeader = append(env.Proxy.ProxySetHeader, d) },
		"upstream":               func(d *Directive) { env.Upstream = append(env.Upstream, d) },
		"gzip":                   func(d *Directive) { env.Proxy.Gzip.Gzip = d },
		"gzip_min_length":        func(d *Directive) { env.Proxy.Gzip.GzipMinLength = d },
	}
}

func newEnv() *Env {
	env := &Env{
		Server: &ServerEnv{
			SSLCertificate:    make([]*Directive, 0),
			SSLCertificateKey: make([]*Directive, 0),
		},
		Proxy: &ProxyEnv{
			ProxySetHeader: make([]*Directive, 0),
			Gzip:           &GzipEnv{},
		},
		Upstream: make([]*Directive, 0),
	}
	env.init()
	return env
}

// Clone clones the environment.
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
	newEnv.init()
	return &newEnv, nil
}

// MustClone clones the environment.
func (env *Env) MustClone() *Env {
	newEnv, err := env.Clone()
	if err != nil {
		panic(err)
	}
	return newEnv
}

// Update updates the environment.
func (env *Env) Update(d *Directive) {
	fn, ok := env.updateFn[d.Directive]
	if ok {
		fn(d)
		return
	}
	_, ok = nginxBackends[d.Directive]
	if ok {
		env.Proxy.Pass = d
	}
}

// GetServerInfo gets the server info from environment.
func (env *Env) GetServerInfo() (*ServerInfo, error) {
	info := &ServerInfo{}
	info.Port = 80

	s := env.Server
	if s.Listen != nil {
		address, port, https, err := processListen(s.Listen)
		if err != nil {
			return nil, err
		}
		info.Address = address
		info.Port = port
		info.HTTPS = https
	}

	if s.ServerName != nil {
		serverName := s.ServerName
		hosts, err := processServerName(serverName)
		if err != nil {
			return nil, err
		}
		info.Hosts = hosts
	}

	if s.SSLClientCertificate != nil {
		cert := s.SSLClientCertificate
		mustContainArgs(cert, 1)
		caCert, err := loadCert(cert.Args[0])
		if err != nil {
			return nil, fmt.Errorf("%s: %v", directiveInfo(cert), err)
		}
		info.CaCert = caCert
	}

	if len(s.SSLCertificate) > 0 {
		certs, keys, err := processSSLCertificates(s.SSLCertificate, s.SSLCertificateKey)
		if err != nil {
			return nil, err
		}
		info.Certs = certs
		info.Keys = keys
	}
	return info, nil
}

// GetProxyInfo gets the proxy info from environment.
func (env *Env) GetProxyInfo() (*ProxyInfo, error) {
	p := env.Proxy
	if p.Pass == nil || p.Pass.Directive != "proxy_pass" {
		return nil, errors.New("no proxy_pass found")
	}
	servers, err := processProxyPass(p.Pass, env)
	if err != nil {
		return nil, err
	}

	var setHeaders map[string]string
	if len(p.ProxySetHeader) > 0 {
		setHeaders, err = processProxySetHeader(p.ProxySetHeader)
		if err != nil {
			return nil, err
		}
	}

	gzipMinLength := processGzip(p.Gzip)

	return &ProxyInfo{
		Servers:       servers,
		SetHeaders:    setHeaders,
		GzipMinLength: gzipMinLength,
	}, nil
}

func processGzip(gzip *GzipEnv) int {
	if gzip.Gzip == nil {
		return 0
	}
	mustContainArgs(gzip.Gzip, 1)
	if gzip.Gzip.Args[0] != "on" {
		return 0
	}
	if gzip.GzipMinLength == nil {
		// nginx default value
		return 20
	}
	mustContainArgs(gzip.GzipMinLength, 1)
	minLength, err := strconv.Atoi(gzip.GzipMinLength.Args[0])
	if err != nil {
		general.Warnf("%s: invalid number %v, use default value of 20 instead", directiveInfo(gzip.GzipMinLength), err)
		return 20
	}
	if minLength < 0 {
		general.Warnf("%s: negative number, use default value of 20 instead", directiveInfo(gzip.GzipMinLength))
		return 20
	}
	return minLength
}

func processProxySetHeader(ds []*Directive) (map[string]string, error) {
	res := make(map[string]string)
	for _, d := range ds {
		mustContainArgs(d, 2)
		res[d.Args[0]] = d.Args[1]
	}
	return res, nil
}

func processProxyPass(d *Directive, env *Env) ([]*BackendInfo, error) {
	mustContainArgs(d, 1)
	url := d.Args[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("%s: proxy_pass %s is not http or https", directiveInfo(d), url)
	}
	prefix := "http://"
	if strings.HasPrefix(url, "https://") {
		prefix = "https://"
	}
	address := url[len(prefix):]

	var upstream *Directive
	for _, u := range env.Upstream {
		mustContainArgs(u, 1)
		if address == u.Args[0] {
			upstream = u
			break
		}
	}
	if upstream == nil {
		return []*BackendInfo{{Server: url, Weight: 1}}, nil
	}
	res := make([]*BackendInfo, 0)
	for _, block := range upstream.Block {
		if block.Directive != "server" {
			continue
		}
		mustContainArgs(block, 1)
		server := block.Args[0]
		weight := 1
		for _, arg := range block.Args[1:] {
			if strings.HasPrefix(arg, "weight=") {
				w, err := strconv.Atoi(arg[7:])
				if err != nil {
					general.Warnf("%s: invalid weight %v, use default value of 1 instead", directiveInfo(block), err)
				} else {
					weight = w
				}
			}
		}
		res = append(res, &BackendInfo{Server: prefix + server, Weight: weight})
	}
	return res, nil
}

func processSSLCertificates(certs []*Directive, keys []*Directive) (map[string]string, map[string]string, error) {
	if len(certs) != len(keys) {
		var missMatch []*Directive
		if len(certs) > len(keys) {
			missMatch = certs[len(keys):]
		} else {
			missMatch = keys[len(certs):]
		}
		msg := ""
		for _, d := range missMatch {
			msg += directiveInfo(d) + "\n"
		}
		return nil, nil, fmt.Errorf("%s has miss certs or keys", msg)
	}

	certMap := make(map[string]string)
	keyMap := make(map[string]string)
	for i := 0; i < len(certs); i++ {
		cert := certs[i]
		key := keys[i]
		mustContainArgs(cert, 1)
		mustContainArgs(key, 1)
		certName := cert.Args[0]
		keyName := key.Args[0]
		certData, err := loadCert(certName)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", directiveInfo(cert), err)
		}
		keyData, err := loadCert(keyName)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", directiveInfo(key), err)
		}
		// cert and keys should have the same key to match.
		certMap[certName] = certData
		keyMap[certName] = keyData
	}
	return certMap, keyMap, nil
}

func loadCert(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	res := base64.StdEncoding.EncodeToString(data)
	return res, nil
}

func processServerName(d *Directive) ([]*HostInfo, error) {
	hosts := make([]*HostInfo, 0)
	for _, arg := range d.Args {
		if strings.HasPrefix(arg, "~") {
			_, err := regexp.Compile(arg[1:])
			if err != nil {
				return nil, fmt.Errorf("%s: %v", directiveInfo(d), err)
			}
			hosts = append(hosts, &HostInfo{
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
			hosts = append(hosts, &HostInfo{
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

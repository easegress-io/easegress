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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/megaease/easegress/v2/cmd/client/general"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
)

const (
	// DirectiveInclude is the include directive.
	DirectiveInclude = "include"
	// DirectiveHTTP is the http directive.
	DirectiveHTTP = "http"
	// DirectiveServer is the server directive.
	DirectiveServer = "server"
	// DirectiveLocation is the location directive.
	DirectiveLocation = "location"
)

// Directive is the nginx directive.
type Directive = crossplane.Directive

// Directives is the nginx directives.
type Directives = crossplane.Directives

// Payload is the nginx payload.
type Payload = crossplane.Payload

func parsePayload(payload *Payload) (*Config, error) {
	addFilenameToPayload(payload)
	directives := payload.Config[0].Parsed
	directives = loadIncludes(directives, payload)

	config := &Config{
		Servers: make([]*Server, 0),
	}
	for _, d := range directives {
		if d.Directive == DirectiveHTTP {
			servers, err := parseHTTPDirective(payload, d)
			if err != nil {
				return nil, err
			}
			for _, s := range servers {
				config.Servers, err = updateServers(config.Servers, s)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return config, nil
}

func parseHTTPDirective(payload *Payload, directive *Directive) ([]*Server, error) {
	env := newEnv()
	directives := loadIncludes(directive.Block, payload)
	for _, d := range directives {
		if d.Directive != DirectiveServer {
			env.Update(d)
		}
	}
	servers := make([]*Server, 0)
	for _, d := range directives {
		if d.Directive == DirectiveServer {
			server, err := parseServerDirective(env.MustClone(), payload, d)
			if err != nil {
				return nil, err
			}
			servers, err = updateServers(servers, server)
			if err != nil {
				return nil, err
			}
		}
	}
	return servers, nil
}

func updateServers(servers []*Server, s *Server) ([]*Server, error) {
	for i, server := range servers {
		if server.Port != s.Port {
			continue
		}
		// same port, try to merge.
		if server.Address != s.Address {
			return nil, fmt.Errorf("two server in port %d have different address %s vs %s", server.Port, server.Address, s.Address)
		}
		if s.HTTPS {
			servers[i].HTTPS = true
		}
		if s.CaCert != "" {
			servers[i].CaCert = s.CaCert
		}
		for k, v := range s.Certs {
			servers[i].Certs[k] = v
		}
		for k, v := range s.Keys {
			servers[i].Keys[k] = v
		}
		servers[i].Rules = append(servers[i].Rules, s.Rules...)
		return servers, nil
	}
	return append(servers, s), nil
}

func parseServerDirective(env *Env, payload *Payload, directive *Directive) (*Server, error) {
	directives := loadIncludes(directive.Block, payload)
	for _, d := range directives {
		if d.Directive != DirectiveLocation {
			env.Update(d)
		}
	}
	info, err := env.GetServerInfo()
	if err != nil {
		return nil, err
	}
	res := &Server{
		ServerBase: info.ServerBase,
		Rules: []*Rule{{
			Hosts: info.Hosts,
			Paths: make([]*Path, 0),
		}},
	}

	for _, d := range directives {
		if d.Directive == DirectiveLocation {
			paths, err := parseLocationDirective(env.MustClone(), payload, d)
			if err != nil {
				return nil, err
			}
			res.Rules[0].Paths = append(res.Rules[0].Paths, paths...)
		}
	}
	return res, nil
}

func parseLocationDirective(env *Env, payload *Payload, directive *Directive) ([]*Path, error) {
	directives := loadIncludes(directive.Block, payload)
	for _, d := range directives {
		env.Update(d)
	}
	res := make([]*Path, 0)
	proxyInfo, err := env.GetProxyInfo()
	// for various nginx backends, we only support proxy_pass now.
	// so we only warn when we can't get proxy info.
	if err != nil {
		general.Warnf("failed to get proxy for %s, %v", directiveInfo(directive), err)
	} else {
		path, pathType, err := parseLocationArgs(directive)
		if err != nil {
			general.Warnf("%s, %v", directiveInfo(directive), err)
		} else {
			res = append(res, &Path{
				Path:    path,
				Type:    pathType,
				Backend: proxyInfo,
			})
		}
	}

	// location can be nested.
	for _, d := range directives {
		if d.Directive == DirectiveLocation {
			paths, err := parseLocationDirective(env.MustClone(), payload, d)
			if err != nil {
				return nil, err
			}
			res = append(res, paths...)
		}
	}
	return res, nil
}

func parseLocationArgs(d *Directive) (string, PathType, error) {
	mustContainArgs(d, 1)
	arg0 := d.Args[0]
	if strings.HasPrefix(arg0, "/") {
		return arg0, PathTypePrefix, nil
	}
	mustContainArgs(d, 2)
	switch arg0 {
	case "=":
		return d.Args[1], PathTypeExact, nil
	case "~":
		return d.Args[1], PathTypeRe, nil
	case "~*":
		return d.Args[1], PathTypeReInsensitive, nil
	case "^~":
		return d.Args[1], PathTypePrefix, nil
	default:
		return "", PathTypePrefix, errors.New("invalid location args, only support =, ~, ~*, ^~")
	}
}

// addFilenameToPayload adds filename to payload recursively for all nested directives.
func addFilenameToPayload(payload *crossplane.Payload) {
	for _, config := range payload.Config {
		filename := filepath.Base(config.File)
		directives := config.Parsed
		for len(directives) > 0 {
			d := directives[0]
			d.File = filename
			directives = append(directives[1:], d.Block...)
		}
	}
}

// loadIncludes loads all include files for current directives recursively but not nested.
func loadIncludes(directives crossplane.Directives, payload *crossplane.Payload) crossplane.Directives {
	res := crossplane.Directives{}
	for _, d := range directives {
		if d.Directive != DirectiveInclude {
			res = append(res, d)
			continue
		}
		mustContainArgs(d, 1)
		name := d.Args[0]
		var include crossplane.Directives
		for _, config := range payload.Config {
			if config.File == name {
				include = config.Parsed
				break
			}
		}
		if include == nil {
			general.Warnf("can't find include file %s for %s", name, directiveInfo(d))
			continue
		}
		include = loadIncludes(include, payload)
		res = append(res, include...)
	}
	return res
}

func mustContainArgs(d *Directive, argNum int) {
	if len(d.Args) < argNum {
		general.ExitWithErrorf(
			"%s must have at least %d args, please update it or remote it.",
			directiveInfo(d), argNum,
		)
	}
}

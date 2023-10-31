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
	"path/filepath"

	"github.com/megaease/easegress/v2/cmd/client/general"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
)

const (
	DirectiveInclude  = "include"
	DirectiveHTTP     = "http"
	DirectiveServer   = "server"
	DirectiveLocation = "location"
)

type Directive = crossplane.Directive
type Directives = crossplane.Directives
type Payload = crossplane.Payload

func parsePayload(payload *Payload) {
	addFilenameToPayload(payload)
	directives := payload.Config[0].Parsed
	directives = loadIncludes(directives, payload)
	for _, d := range directives {
		if d.Directive == DirectiveHTTP {
			parseHTTPDirective(payload, d)
		}
	}
}

func parseHTTPDirective(payload *Payload, directive *Directive) {
	env := newEnv()
	directives := loadIncludes(directive.Block, payload)
	for _, d := range directives {
		if d.Directive != DirectiveServer {
			env.Update(d)
		}
	}
	for _, d := range directives {
		if d.Directive == DirectiveServer {
			parseServerDirective(env.MustClone(), payload, d)
		}
	}
}

func parseServerDirective(env *Env, payload *Payload, directive *Directive) {
	directives := loadIncludes(directive.Block, payload)
	for _, d := range directives {
		if d.Directive != DirectiveLocation {
			env.Update(d)
		}
	}
	for _, d := range directives {
		if d.Directive == DirectiveLocation {
			parseLocationDirective(env.MustClone(), payload, d)
		}
	}
}

func parseLocationDirective(env *Env, payload *Payload, directive *Directive) {
	directives := loadIncludes(directive.Block, payload)
	for _, d := range directives {
		env.Update(d)
	}
	for _, d := range directives {
		if d.Directive == DirectiveLocation {
			parseLocationDirective(env.MustClone(), payload, d)
		}
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

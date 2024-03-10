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

// Package ordered provides the router implementation of ordered routing policy.
package ordered

import (
	"regexp"
	"strings"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
)

type (
	muxRule struct {
		routers.Rule
		paths []*muxPath
	}

	muxPath struct {
		routers.Path
		pathRE *regexp.Regexp
	}

	orderedRouter struct {
		rules []*muxRule
	}
)

var kind = &routers.Kind{
	Name:        "Ordered",
	Description: "Ordered",

	CreateInstance: func(rules routers.Rules) routers.Router {
		muxRules := make([]*muxRule, len(rules))
		for i, rule := range rules {
			paths := make([]*muxPath, len(rule.Paths))
			for j, path := range rule.Paths {
				paths[j] = newMuxPath(path)
			}

			muxRules[i] = &muxRule{
				Rule:  *rule,
				paths: paths,
			}
		}
		return &orderedRouter{
			rules: muxRules,
		}
	},
}

func init() {
	routers.Register(kind)
}

func newMuxPath(p *routers.Path) *muxPath {
	var pathRE *regexp.Regexp
	if p.PathRegexp != "" {
		var err error
		pathRE, err = regexp.Compile(p.PathRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v", p.PathRegexp, err)
		}
	}

	return &muxPath{
		Path:   *p,
		pathRE: pathRE,
	}
}

func (mp *muxPath) Protocol() string {
	return "http"
}

func (mp *muxPath) matchPath(path string) bool {
	if mp.Path.Path == "" && mp.PathPrefix == "" && mp.pathRE == nil {
		return true
	}

	if mp.Path.Path != "" && mp.Path.Path == path {
		return true
	}
	if mp.PathPrefix != "" && strings.HasPrefix(path, mp.PathPrefix) {
		return true
	}
	if mp.pathRE != nil {
		return mp.pathRE.MatchString(path)
	}

	return false
}

func (mp *muxPath) Rewrite(context *routers.RouteContext) {
	if mp.RewriteTarget == "" {
		return
	}
	r := context.Request
	path := context.Path

	if mp.Path.Path != "" && mp.Path.Path == path {
		r.SetPath(mp.RewriteTarget)
		return
	}

	if mp.PathPrefix != "" && strings.HasPrefix(path, mp.PathPrefix) {
		path = mp.RewriteTarget + path[len(mp.PathPrefix):]
		r.SetPath(path)
		return
	}

	// sure (mp.pathRE != nil && mp.pathRE.MatchString(path)) is true
	path = mp.pathRE.ReplaceAllString(path, mp.RewriteTarget)
	r.SetPath(path)
}

func (r *orderedRouter) Search(context *routers.RouteContext) {
	req := context.Request
	ip := req.RealIP()
	path := context.Path

	for _, rule := range r.rules {
		if !rule.MatchHost(context) {
			continue
		}

		if !rule.AllowIP(ip) {
			context.IPMismatch = true
			continue
		}

		for _, mp := range rule.paths {
			if !mp.matchPath(path) {
				continue
			}

			if mp.Match(context) {
				context.Route = mp
				return
			}
		}
	}
}

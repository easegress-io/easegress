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

package order

import (
	"regexp"
	"strings"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httpserver/routers"
)

type (
	muxRule struct {
		routers.Rule
		routes []*route
	}

	route struct {
		routers.Path
		pathRE *regexp.Regexp
	}

	OrderRouter struct {
		rules []*muxRule
	}
)

// Kind is the kind of Proxy.
const Kind = "Order"

var kind = &routers.Kind{
	Name:        Kind,
	Description: "Order",

	CreateInstance: func(rules routers.Rules) routers.Router {
		muxRules := make([]*muxRule, len(rules))
		for i, rule := range rules {
			routes := make([]*route, len(rule.Paths))
			for j, path := range rule.Paths {
				routes[j] = newRoute(path)
			}

			muxRules[i] = &muxRule{
				Rule:   *rule,
				routes: routes,
			}
		}
		return &OrderRouter{
			rules: muxRules,
		}
	},
}

func init() {
	routers.Register(kind)
}

func newRoute(p *routers.Path) *route {
	var pathRE *regexp.Regexp
	if p.PathRegexp != "" {
		var err error
		pathRE, err = regexp.Compile(p.PathRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v", p.PathRegexp, err)
		}
	}

	return &route{
		Path:   *p,
		pathRE: pathRE,
	}
}

func (mp *route) matchPath(path string) bool {
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

func (mp *route) Rewrite(context *routers.RouteContext) {
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

func (r *OrderRouter) Search(context *routers.RouteContext) {
	req := context.Request
	ip := req.RealIP()
	path := context.Path

	for _, host := range r.rules {
		if !host.Match(req) {
			continue
		}

		if !host.AllowIP(ip) {
			context.IPNotAllowed = true
			return
		}

		for _, route := range host.routes {
			if !route.matchPath(path) {
				continue
			}

			if route.Match(context) {
				context.Route = route
				return
			}
		}
	}
}

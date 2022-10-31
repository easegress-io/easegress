package order

import (
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/httpserver/routers"
)

type (
	muxRule struct {
		httpserver.Rule
		paths []*muxPath
	}

	muxPath struct {
		httpserver.Path
	}

	orderRouter struct {
		rules []*muxRule
	}
)

// Kind is the kind of Proxy.
const Kind = "Order"

var kind = &routers.Kind{
	Name:        Kind,
	Description: "Order",

	CreateInstance: func(rules httpserver.Rules) routers.Router {
		muxRules := make([]*muxRule, len(rules))
		for i, rule := range rules {
			muxPaths := make([]*muxPath, len(rule.Paths))
			for j, path := range rule.Paths {
				muxPaths[j] = &muxPath{
					Path: *path,
				}
			}

			muxRules[i] = &muxRule{
				Rule:  *rule,
				paths: muxPaths,
			}
		}
		return &orderRouter{
			rules: muxRules,
		}
	},
}

func init() {
	routers.Register(kind)
}

func (r *orderRouter) Search(context *routers.RouteContext) {
	req := context.Request
	for _, host := range r.rules {
		if !host.Match(req) {
			continue
		}

		if !allowIP(host.ipFilter, ip) {
			return forbidden
		}


		for _, path := range host.paths {
			if !path.matchPath(req) {
				continue
			}

			if !path.matchMethod(req) {
				methodMismatch = true
				continue
			}

			// only if headers and query are empty, we can cache the result.
			if len(path.headers) == 0 && len(path.queries) == 0 {
				r = &route{code: 0, path: path}
				mi.putRouteToCache(req, r)
			}

			if len(path.headers) > 0 && !path.matchHeaders(req) {
				headerMismatch = true
				continue
			}

			if len(path.queries) > 0 && !path.matchQueries(req) {
				queryMismatch = true
				continue
			}

			if !allowIP(path.ipFilter, ip) {
				return forbidden
			}

			return &route{code: 0, path: path}
		}
	}

	if headerMismatch || queryMismatch {
		return badRequest
	}

	if methodMismatch {
		mi.putRouteToCache(req, methodNotAllowed)
		return methodNotAllowed
	}

	mi.putRouteToCache(req, notFound)
	return notFound
}

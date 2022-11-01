package order

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

type (
	muxRule struct {
		routers.Rule
		paths []*muxPath
	}

	muxPath struct {
		routers.Path
		pathRE *regexp.Regexp
		method httpprot.MethodType
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
			muxPaths := make([]*muxPath, len(rule.Paths))
			for j, path := range rule.Paths {
				muxPaths[j] = newMuxPath(path)
			}

			muxRules[i] = &muxRule{
				Rule:  *rule,
				paths: muxPaths,
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

func (r *OrderRouter) Search(context *routers.RouteContext) {
	req := context.Request
	ip := req.RealIP()

	for _, host := range r.rules {
		if !host.Match(req) {
			continue
		}

		if !host.AllowIP(ip) {
			context.Code = http.StatusForbidden
			return
		}

		for _, path := range host.paths {
			if !path.match(context.Path) {
				continue
			}

			if path.Match(context) {
				context.Route = path
				return
			}
		}
	}

	// if headerMismatch || queryMismatch {
	// 	context.Code = http.StatusBadRequest
	// 	return
	// }

	// context.Cache = true
	// if methodMismatch {
	// 	context.Code = http.StatusMethodNotAllowed
	// 	return
	// }
	// context.Code = http.StatusNotFound
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

	method := httpprot.MALL
	if len(p.Methods) != 0 {
		method = 0
		for _, m := range p.Methods {
			method |= httpprot.Methods[m]
		}
	}

	return &muxPath{
		Path:   *p,
		pathRE: pathRE,
		method: method,
	}
}

func (mp *muxPath) match(path string) bool {
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

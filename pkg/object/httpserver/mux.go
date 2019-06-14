package httpserver

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/ipfilter"
)

// TODO:
// 1. Rewrites

type (
	mux struct {
		spec     *Spec
		handlers *sync.Map

		rules atomic.Value // *muxRules
	}

	muxRules struct {
		cache *cache

		ipFilter     *ipfilter.IPFilter
		ipFilterChan *ipfilter.IPFilters

		rules []*muxRule
	}

	muxRule struct {
		ipFilter      *ipfilter.IPFilter
		ipFilterChain *ipfilter.IPFilters

		host       string
		hostRegexp string
		hostRE     *regexp.Regexp
		paths      []*muxPath
	}

	muxPath struct {
		ipFilter      *ipfilter.IPFilter
		ipFilterChain *ipfilter.IPFilters

		path       string
		pathPrefix string
		pathRegexp string
		pathRE     *regexp.Regexp
		methods    []string
		backend    string
	}
)

// newIPFilterChain returns nil if the number of final filters is zero.
func newIPFilterChain(parentIPFilters *ipfilter.IPFilters, childSpec *ipfilter.Spec) *ipfilter.IPFilters {
	var ipFilters *ipfilter.IPFilters
	if parentIPFilters != nil {
		ipFilters = ipfilter.NewIPfilters(parentIPFilters.Filters()...)
	} else {
		ipFilters = ipfilter.NewIPfilters()
	}

	if childSpec != nil {
		ipFilters.Append(ipfilter.New(childSpec))
	}

	if len(ipFilters.Filters()) == 0 {
		return nil
	}

	return ipFilters
}

func newIPFilter(spec *ipfilter.Spec) *ipfilter.IPFilter {
	if spec == nil {
		return nil
	}

	return ipfilter.New(spec)
}

func (mr *muxRules) pass(ctx *httpContext) bool {
	if mr.ipFilter == nil {
		return true
	}

	return mr.ipFilter.AllowHTTPContext(ctx)
}

func (mr *muxRules) getCacheItem(ctx *httpContext) *cacheItem {
	if mr.cache == nil {
		return nil
	}

	r := ctx.r
	key := fmt.Sprintf("%s%s", r.Method(), r.Path())
	return mr.cache.get(key)
}

func (mr *muxRules) putCacheItem(ctx *httpContext, ci *cacheItem) {
	if mr.cache == nil {
		return
	}

	r := ctx.r
	key := fmt.Sprintf("%s%s", r.Method(), r.Path())
	if !ci.cached {
		ci.cached = true
		// NOTE: It's fine to cover the existed item because of conccurently updating cache.
		mr.cache.put(key, ci)
	}
}

func newMuxRule(parentIPFilters *ipfilter.IPFilters, rule *Rule, paths []*muxPath) *muxRule {
	var hostRE *regexp.Regexp

	if rule.HostRegexp != "" {
		var err error
		hostRE, err = regexp.Compile(rule.HostRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v",
				rule.HostRegexp, err)
		}
	}

	return &muxRule{
		ipFilter:      newIPFilter(rule.IPFilter),
		ipFilterChain: newIPFilterChain(parentIPFilters, rule.IPFilter),

		host:       rule.Host,
		hostRegexp: rule.HostRegexp,
		hostRE:     hostRE,
		paths:      paths,
	}
}

func (mr *muxRule) pass(ctx *httpContext) bool {
	if mr.ipFilter == nil {
		return true
	}

	return mr.ipFilter.AllowHTTPContext(ctx)
}

func (mr *muxRule) match(ctx *httpContext) bool {
	r := ctx.r

	if mr.host == "" && mr.hostRE == nil {
		return true
	}

	if mr.host != "" && mr.host == r.Host() {
		return true
	}
	if mr.hostRE != nil && mr.hostRE.MatchString(r.Host()) {
		return true
	}

	return false
}

func newMuxPath(parentIPFilters *ipfilter.IPFilters, path *Path) *muxPath {
	var pathRE *regexp.Regexp
	if path.PathRegexp != "" {
		var err error
		pathRE, err = regexp.Compile(path.PathRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v",
				path.PathRegexp, err)
		}
	}

	return &muxPath{
		ipFilter:      newIPFilter(path.IPFilter),
		ipFilterChain: newIPFilterChain(parentIPFilters, path.IPFilter),

		path:       path.Path,
		pathPrefix: path.PathPrefix,
		pathRegexp: path.PathRegexp,
		pathRE:     pathRE,
		methods:    path.Methods,
		backend:    path.Backend,
	}
}

func (mp *muxPath) pass(ctx *httpContext) bool {
	if mp.ipFilter == nil {
		return true
	}

	return mp.ipFilter.AllowHTTPContext(ctx)
}

func (mp *muxPath) matchPath(ctx *httpContext) bool {
	r := ctx.r

	if mp.path == "" && mp.pathPrefix == "" && mp.pathRE == nil {
		return true
	}

	if mp.path != "" && mp.path == r.Path() {
		return true
	}
	if mp.pathPrefix != "" && strings.HasPrefix(r.Path(), mp.pathPrefix) {
		return true
	}
	if mp.pathRE != nil {
		return mp.pathRE.MatchString(r.Path())
	}

	return false
}

func (mp *muxPath) matchMethod(ctx *httpContext) bool {
	if len(mp.methods) == 0 {
		return true
	}

	return common.StrInSlice(ctx.r.Method(), mp.methods)
}

func newMux(spec *Spec, handlers *sync.Map) *mux {
	m := &mux{handlers: handlers}

	m.reloadRules(spec)

	return m
}

func (m *mux) reloadRules(spec *Spec) {
	m.spec = spec

	rules := &muxRules{
		ipFilter:     newIPFilter(spec.IPFilter),
		ipFilterChan: newIPFilterChain(nil, spec.IPFilter),
		rules:        make([]*muxRule, len(spec.Rules)),
	}

	if spec.CacheSize > 0 {
		rules.cache = newCache(spec.CacheSize)
	}

	var ipFilters []*ipfilter.IPFilter
	if spec.IPFilter != nil {
		ipFilters = append(ipFilters, ipfilter.New(spec.IPFilter))
	}

	for i := 0; i < len(rules.rules); i++ {
		specRule := spec.Rules[i]

		ruleIPFilterChain := newIPFilterChain(rules.ipFilterChan, specRule.IPFilter)

		paths := make([]*muxPath, len(specRule.Paths))
		for j := 0; j < len(paths); j++ {
			paths[j] = newMuxPath(ruleIPFilterChain, &specRule.Paths[j])
		}

		// NOTE: Given the parent ipFilters not its own.
		rules.rules[i] = newMuxRule(rules.ipFilterChan, &specRule, paths)
	}

	m.rules.Store(rules)
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rules := m.rules.Load().(*muxRules)

	startTime := time.Now()
	ctx := newHTTPContext(&startTime, w, r)
	defer ctx.finish()

	handleIPNotAllow := func() {
		ctx.AddTag(fmt.Sprintf("ip not allow"))
		ctx.w.SetStatusCode(http.StatusForbidden)
	}

	handleCacheItem := func(ci *cacheItem) {
		rules.putCacheItem(ctx, ci)

		if ci.ipFilterChan != nil {
			if !ci.ipFilterChan.AllowHTTPContext(ctx) {
				handleIPNotAllow()
				return
			}
		}

		switch {
		case ci.notFound:
			ctx.w.SetStatusCode(http.StatusNotFound)
		case ci.methodNotAllowed:
			ctx.w.SetStatusCode(http.StatusMethodNotAllowed)
		case ci.backend != "":
			handler, exists := m.handlers.Load(ci.backend)
			if exists {
				handler.(Handler).Handle(ctx)
			} else {
				ctx.AddTag(fmt.Sprintf("backend %s not found", ci.backend))
				ctx.w.SetStatusCode(http.StatusServiceUnavailable)
			}
		}
	}

	ci := rules.getCacheItem(ctx)
	if ci != nil {
		handleCacheItem(ci)
		return
	}

	if !rules.pass(ctx) {
		handleIPNotAllow()
		return
	}

	for _, rule := range rules.rules {
		if !rule.match(ctx) {
			continue
		}
		if !rule.pass(ctx) {
			handleIPNotAllow()
			return
		}

		for _, path := range rule.paths {
			if !path.matchPath(ctx) {
				continue
			}

			if path.matchMethod(ctx) {
				if !path.pass(ctx) {
					handleIPNotAllow()
					return
				}
				handleCacheItem(&cacheItem{ipFilterChan: path.ipFilterChain, backend: path.backend})
			} else {
				handleCacheItem(&cacheItem{ipFilterChan: path.ipFilterChain, methodNotAllowed: true})
			}
			return
		}
	}

	handleCacheItem(&cacheItem{ipFilterChan: rules.ipFilterChan, notFound: true})
}

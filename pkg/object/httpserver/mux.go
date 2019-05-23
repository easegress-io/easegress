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
)

// TODO:
// 1. Rewrites

type (
	mux struct {
		spec     *Spec
		handlers *sync.Map

		cache *cache

		rules atomic.Value // []*muxRule
	}

	muxRule struct {
		host       string
		hostRegexp string
		hostRE     *regexp.Regexp
		paths      []*muxPath
	}

	muxPath struct {
		path       string
		pathPrefix string
		pathRegexp string
		pathRE     *regexp.Regexp
		methods    []string
		backend    string
	}
)

func newMuxRule(rule *Rule, paths []*muxPath) *muxRule {
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
		host:       rule.Host,
		hostRegexp: rule.HostRegexp,
		hostRE:     hostRE,
		paths:      paths,
	}
}

func (mr *muxRule) match(r *http.Request) bool {
	if mr.host == "" && mr.hostRE == nil {
		return true
	}

	if mr.host != "" && mr.host == r.Host {
		return true
	}
	if mr.hostRE != nil && mr.hostRE.MatchString(r.Host) {
		return true
	}

	return false
}

func newMuxPath(path *Path) *muxPath {
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
		path:       path.Path,
		pathPrefix: path.PathPrefix,
		pathRegexp: path.PathRegexp,
		pathRE:     pathRE,
		methods:    path.Methods,
		backend:    path.Backend,
	}
}

func (mp *muxPath) matchPath(r *http.Request) bool {
	if mp.path == "" && mp.pathPrefix == "" && mp.pathRE == nil {
		return true
	}

	if mp.path != "" && mp.path == r.URL.Path {
		return true
	}
	if mp.pathPrefix != "" && strings.HasPrefix(r.URL.Path, mp.pathPrefix) {
		return true
	}
	if mp.pathRE != nil {
		return mp.pathRE.MatchString(r.URL.Path)
	}

	return false
}

func (mp *muxPath) matchMethod(r *http.Request) bool {
	if len(mp.methods) == 0 {
		return true
	}

	return common.StrInSlice(r.Method, mp.methods)
}

func newMux(spec *Spec, handlers *sync.Map) *mux {
	m := &mux{handlers: handlers}
	if spec.CacheSize > 0 {
		m.cache = newCache(spec.CacheSize)
	}

	m.reloadRules(spec)

	return m
}

func (m *mux) reloadRules(spec *Spec) {
	m.spec = spec

	rules := make([]*muxRule, len(spec.Rules))
	for i := 0; i < len(rules); i++ {
		specRule := spec.Rules[i]

		paths := make([]*muxPath, len(specRule.Paths))
		for j := 0; j < len(paths); j++ {
			paths[j] = newMuxPath(&specRule.Paths[j])
		}

		rules[i] = newMuxRule(&specRule, paths)
	}

	m.rules.Store(rules)
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	handleRequest := func(ci *cacheItem) {
		m.putCacheItem(r, ci)

		switch {
		case ci.notFound:
			w.WriteHeader(http.StatusNotFound)
		case ci.methodNotAllowed:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case ci.backend != "":
			handler, exists := m.handlers.Load(ci.backend)
			if exists {
				ctx := newHTTPContext(&startTime, w, r)
				handler.(Handler).Handle(ctx)
				ctx.finish()
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}
	}

	ci := m.getCacheItem(r)
	if ci != nil {
		handleRequest(ci)
		return
	}

	rules := m.rules.Load().([]*muxRule)
	for _, rule := range rules {
		if !rule.match(r) {
			continue
		}
		for _, path := range rule.paths {
			if !path.matchPath(r) {
				continue
			}
			if path.matchMethod(r) {
				handleRequest(&cacheItem{backend: path.backend})
			} else {
				handleRequest(&cacheItem{methodNotAllowed: true})
			}
			return
		}
	}

	handleRequest(&cacheItem{notFound: true})
}

func (m *mux) getCacheItem(r *http.Request) *cacheItem {
	if m.cache == nil {
		return nil
	}

	key := fmt.Sprintf("%s%s", r.Method, r.URL.Path)
	return m.cache.get(key)
}

func (m *mux) putCacheItem(r *http.Request, ci *cacheItem) {
	if m.cache == nil {
		return
	}

	key := fmt.Sprintf("%s%s", r.Method, r.URL.Path)
	if !ci.cached {
		ci.cached = true
		// NOTE: It's fine to cover the existed item because of conccurently updating cache.
		m.cache.put(key, ci)
	}
}

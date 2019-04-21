package httpserver

import (
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
// 1. Add fixed size(in case of memory explosion) to cache mux.
// 2. Rewrites

type (
	mux struct {
		spec     *Spec
		handlers *sync.Map

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
				handler, exists := m.handlers.Load(path.backend)
				if exists {
					ctx := newHTTPContext(&startTime, w, r)
					handler.(Handler).Handle(ctx)
					ctx.finish()
				} else {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

package routers

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type Rules []*Rule

type Paths []*Path

type Rule struct {
	// NOTICE: If the field is a pointer, it must have `omitempty` in tag `json`
	// when it has `omitempty` in tag `jsonschema`.
	// Otherwise it will output null value, which is invalid in json schema (the type is object).
	// the original reason is the jsonscheme(genjs) has not support multiple types.
	// Reference: https://github.com/alecthomas/jsonschema/issues/30
	// In the future if we have the scenario where we need marshal the field, but omitempty
	// in the schema, we are suppose to support multiple types on our own.
	IPFilterSpec *ipfilter.Spec `json:"ipFilter,omitempty" jsonschema:"omitempty"`
	Host         string         `json:"host" jsonschema:"omitempty"`
	HostRegexp   string         `json:"hostRegexp" jsonschema:"omitempty,format=regexp"`
	Paths        Paths          `json:"paths" jsonschema:"omitempty"`

	ipFilter      *ipfilter.IPFilter
	ipFilterChain *ipfilter.IPFilters
	hostRE        *regexp.Regexp
}

type Path struct {
	IPFilterSpec      *ipfilter.Spec `json:"ipFilter,omitempty" jsonschema:"omitempty"`
	Path              string         `json:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
	PathPrefix        string         `json:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
	PathRegexp        string         `json:"pathRegexp,omitempty" jsonschema:"omitempty,format=regexp"`
	RewriteTarget     string         `json:"rewriteTarget" jsonschema:"omitempty"`
	Methods           []string       `json:"methods,omitempty" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
	Backend           string         `json:"backend" jsonschema:"required"`
	ClientMaxBodySize int64          `json:"clientMaxBodySize" jsonschema:"omitempty"`
	Headers           Headers        `json:"headers" jsonschema:"omitempty"`
	Queries           Queries        `json:"queries,omitempty" jsonschema:"omitempty"`
	MatchAllHeader    bool           `json:"matchAllHeader" jsonschema:"omitempty"`
	MatchAllQuery     bool           `json:"matchAllQuery" jsonschema:"omitempty"`

	ipFilter               *ipfilter.IPFilter
	ipFilterChain          *ipfilter.IPFilters
	method                 httpprot.MethodType
	cacheable, noNeedMatch bool
}

type Headers []*Header

type Queries []*Query

type Header struct {
	Key    string   `json:"key" jsonschema:"required"`
	Values []string `json:"values,omitempty" jsonschema:"omitempty,uniqueItems=true"`
	Regexp string   `json:"regexp,omitempty" jsonschema:"omitempty,format=regexp"`

	re *regexp.Regexp
}

type Query struct {
	Key    string   `json:"key" jsonschema:"required"`
	Values []string `json:"values,omitempty" jsonschema:"omitempty,uniqueItems=true"`
	Regexp string   `json:"regexp,omitempty" jsonschema:"omitempty,format=regexp"`

	re *regexp.Regexp
}

func (rules Rules) Init(ipFilterChan *ipfilter.IPFilters) {
	for _, rule := range rules {
		rule.Init(ipFilterChan)
	}
}

func (rule *Rule) Init(parentIPFilters *ipfilter.IPFilters) {
	ruleIPFilterChain := ipfilter.NewIPFilterChain(parentIPFilters, rule.IPFilterSpec)
	for _, p := range rule.Paths {
		p.Init(ruleIPFilterChain)
	}

	var hostRE *regexp.Regexp

	if rule.HostRegexp != "" {
		var err error
		hostRE, err = regexp.Compile(rule.HostRegexp)
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v", rule.HostRegexp, err)
		}
	}

	rule.ipFilter = ipfilter.New(rule.IPFilterSpec)
	rule.ipFilterChain = ipfilter.NewIPFilterChain(parentIPFilters, rule.IPFilterSpec)
	rule.hostRE = hostRE
}

func (rule *Rule) Match(req *httpprot.Request) bool {
	if rule.Host == "" && rule.hostRE == nil {
		return true
	}

	host := req.HostOnly()

	if rule.Host != "" && rule.Host == host {
		return true
	}
	if rule.hostRE != nil && rule.hostRE.MatchString(host) {
		return true
	}

	return false
}

func (rule *Rule) AllowIP(ip string) bool {
	if rule.ipFilter == nil {
		return true
	}
	return rule.ipFilter.Allow(ip)
}

func (p *Path) Init(parentIPFilters *ipfilter.IPFilters) {
	p.ipFilter = ipfilter.New(p.IPFilterSpec)
	p.ipFilterChain = ipfilter.NewIPFilterChain(parentIPFilters, p.IPFilterSpec)

	p.Headers.init()
	p.Queries.init()

	method := httpprot.MALL
	if len(p.Methods) != 0 {
		method = 0
		for _, m := range p.Methods {
			method |= httpprot.Methods[m]
		}
	}

	p.method = method

	if len(p.Headers) == 0 && len(p.Queries) == 0 {
		p.cacheable = true
		if len(p.Methods) == 0 && p.ipFilter == nil {
			p.noNeedMatch = true
		}
	}
}

// Validate validates Path.
func (p *Path) Validate() error {
	if (stringtool.IsAllEmpty(p.Path, p.PathPrefix, p.PathRegexp)) && p.RewriteTarget != "" {
		return fmt.Errorf("rewriteTarget is specified but path is empty")
	}

	return nil
}

func (p *Path) AllowIP(ip string) bool {
	if p.ipFilter == nil {
		return true
	}

	return p.ipFilter.Allow(ip)
}

func (p *Path) AllowIPChain(ip string) bool {
	if p.ipFilterChain == nil {
		return true
	}

	return p.ipFilterChain.Allow(ip)
}

func (p *Path) Match(context *RouteContext) bool {
	context.Cacheable = p.cacheable

	if p.noNeedMatch {
		return true
	}

	// method match
	req := context.Request
	ip := req.RealIP()

	if req.MethodType()&p.method == 0 {
		context.MethodMismatch = true
		return false
	}

	if len(p.Headers) > 0 && !p.Headers.Match(req.HTTPHeader(), p.MatchAllHeader) {
		context.HeaderMismatch = true
		return false
	}

	if len(p.Queries) > 0 && !p.Queries.Match(req.Queries(), p.MatchAllQuery) {
		context.QueryMismatch = true
		return false
	}

	if !p.AllowIP(ip) {
		context.IPNotAllowed = true
	}

	return true
}

func (p *Path) GetBackend() string {
	return p.Backend
}

func (p *Path) GetClientMaxBodySize() int64 {
	return p.ClientMaxBodySize
}

func (hs Headers) init() {
	for _, h := range hs {
		if h.Regexp != "" {
			h.re = regexp.MustCompile(h.Regexp)
		}
	}
}

func (hs Headers) Validate() error {
	for _, h := range hs {
		if len(h.Values) == 0 && h.Regexp == "" {
			return fmt.Errorf("both of values and regexp are empty for key: %s", h.Key)
		}
	}
	return nil
}

func (hs Headers) Match(headers http.Header, matchAll bool) bool {
	if len(hs) == 0 {
		return true
	}

	if matchAll {
		for _, h := range hs {
			v := headers.Get(h.Key)
			if len(h.Values) > 0 && !stringtool.StrInSlice(v, h.Values) {
				return false
			}

			if h.Regexp != "" && !h.re.MatchString(v) {
				return false
			}
		}
	} else {
		for _, h := range hs {
			v := headers.Get(h.Key)
			if stringtool.StrInSlice(v, h.Values) {
				return true
			}

			if h.Regexp != "" && h.re.MatchString(v) {
				return true
			}
		}
	}

	return matchAll
}

func (qs Queries) init() {
	for _, q := range qs {
		if q.Regexp != "" {
			q.re = regexp.MustCompile(q.Regexp)
		}
	}
}

func (qs Queries) Validate() error {
	for _, q := range qs {
		if len(q.Values) == 0 && q.Regexp == "" {
			return fmt.Errorf("both of values and regexp are empty for key: %s", q.Key)
		}
	}
	return nil
}

func (qs Queries) Match(query url.Values, matchAll bool) bool {
	if len(qs) == 0 {
		return true
	}

	if matchAll {
		for _, q := range qs {
			v := query.Get(q.Key)
			if len(q.Values) > 0 && !stringtool.StrInSlice(v, q.Values) {
				return false
			}

			if q.Regexp != "" && !q.re.MatchString(v) {
				return false
			}
		}
	} else {
		for _, q := range qs {
			v := query.Get(q.Key)
			if stringtool.StrInSlice(v, q.Values) {
				return true
			}

			if q.Regexp != "" && q.re.MatchString(v) {
				return true
			}
		}
	}

	return matchAll
}

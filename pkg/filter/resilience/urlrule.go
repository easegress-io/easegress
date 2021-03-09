package resilience

import (
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

type (
	// StringMatch defines the match rule of a string
	StringMatch struct {
		Exact  string `yaml:"exact"`
		Prefix string `yaml:"prefix"`
		RegEx  string `yaml:"regex"`
		re     *regexp.Regexp
	}

	// URLRule defines the match rule of a http request
	URLRule struct {
		Methods   []string    `yaml:"methods" jsonschema:"omitempty"`
		URL       StringMatch `yaml:"url" jsonschema:"required"`
		PolicyRef string      `yaml:"policyRef" jsonschema:"omitempty"`
	}
)

func (r *URLRule) Match(req context.HTTPRequest) bool {
	if len(r.Methods) > 0 && r.Methods[0] != "*" {
		if !stringtool.StrInSlice(req.Method(), r.Methods) {
			return false
		}
	}

	path := req.Path()
	// TODO: if u == "" {}

	if r.URL.Exact != "" && path == r.URL.Exact {
		return true
	}

	if r.URL.Prefix != "" && strings.HasPrefix(path, r.URL.Prefix) {
		return true
	}

	if r.URL.RegEx == "" {
		return false
	}

	if r.URL.re == nil {
		// TODO: handle panic
		r.URL.re = regexp.MustCompile(r.URL.RegEx)
	}

	return r.URL.re.MatchString(path)
}

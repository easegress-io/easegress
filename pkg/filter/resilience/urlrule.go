package resilience

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

type (
	// StringMatch defines the match rule of a string
	StringMatch struct {
		Exact  string `yaml:"exact" jsonschema:"omitempty"`
		Prefix string `yaml:"prefix" jsonschema:"omitempty"`
		RegEx  string `yaml:"regex" jsonschema:"omitempty"`
		re     *regexp.Regexp
	}

	// URLRule defines the match rule of a http request
	URLRule struct {
		Methods   []string    `yaml:"methods" jsonschema:"omitempty,uniqueItems=true"`
		URL       StringMatch `yaml:"url" jsonschema:"required"`
		PolicyRef string      `yaml:"policyRef" jsonschema:"omitempty"`
	}
)

// Validate validates the StringMatch object
func (sm StringMatch) Validate() error {
	if sm.RegEx != "" {
		_, e := regexp.Compile(sm.RegEx)
		return e
	}

	if sm.Exact != "" {
		return nil
	}

	if sm.Prefix != "" {
		return nil
	}

	return fmt.Errorf("at least one pattern must be configured")
}

// Match matches a URL to the rule
func (r *URLRule) Match(req context.HTTPRequest) bool {
	if len(r.Methods) > 0 && r.Methods[0] != "*" {
		if !stringtool.StrInSlice(req.Method(), r.Methods) {
			return false
		}
	}

	path := req.Path()

	if r.URL.Exact != "" && path == r.URL.Exact {
		return true
	}

	if r.URL.Prefix != "" && strings.HasPrefix(path, r.URL.Prefix) {
		return true
	}

	if r.URL.re == nil {
		return false
	}

	return r.URL.re.MatchString(path)
}

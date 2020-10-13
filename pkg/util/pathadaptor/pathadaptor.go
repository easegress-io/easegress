package pathadaptor

import (
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/logger"
)

type (
	// Spec describes rules for PathAdaptor.
	Spec struct {
		Replace       string         `yaml:"replace,omitempty" jsonschema:"omitempty"`
		AddPrefix     string         `yaml:"addPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		TrimPrefix    string         `yaml:"trimPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		RegexpReplace *RegexpReplace `yaml:"regexpReplace,omitempty" jsonschema:"omitempty"`
	}

	// RegexpReplace use regexp-replace pair to rewrite path.
	RegexpReplace struct {
		Regexp  string `yaml:"regexp" jsonschema:"required,format=regexp"`
		Replace string `yaml:"replace"`

		re *regexp.Regexp
	}

	// PathAdaptor is the path Adaptor.
	PathAdaptor struct {
		spec *Spec
	}
)

// New creates a pathAdaptor.
func New(spec *Spec) *PathAdaptor {
	if spec.RegexpReplace != nil {
		var err error
		spec.RegexpReplace.re, err = regexp.Compile(spec.RegexpReplace.Regexp)
		if err != nil {
			logger.Errorf("BUG: compile regexp %s failed: %v",
				spec.RegexpReplace.Regexp, err)
		}
	}

	return &PathAdaptor{
		spec: spec,
	}
}

// Adapt adapts path.
func (pa *PathAdaptor) Adapt(path string) string {
	if len(pa.spec.Replace) != 0 {
		return pa.spec.Replace
	}

	if len(pa.spec.AddPrefix) != 0 {
		return pa.spec.AddPrefix + path
	}

	if len(pa.spec.TrimPrefix) != 0 {
		return strings.TrimPrefix(path, pa.spec.TrimPrefix)
	}

	if pa.spec.RegexpReplace != nil && pa.spec.RegexpReplace.re != nil {
		return pa.spec.RegexpReplace.re.ReplaceAllString(path,
			pa.spec.RegexpReplace.Replace)
	}

	return path
}

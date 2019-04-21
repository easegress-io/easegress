package httpfilter

import (
	"fmt"
	"regexp"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
)

type (
	// Spec describes HTTPFilter.
	Spec struct {
		Headers map[string]*ValueFilter `yaml:"headers" v:"dive,keys,required,endkeys,required"`
	}

	// ValueFilter describes value.
	ValueFilter struct {
		V string `yaml:"-" v:"parent"`

		// NOTICE: It allows empty value.
		Values []string `yaml:"values" v:"unique"`
		Regexp string   `yaml:"regexp" v:"omitempty,regexp"`
		re     *regexp.Regexp
	}

	// HTTPFilter filters HTTP entity.
	HTTPFilter struct {
		spec *Spec
	}
)

// Validate valites ValueFilter.
func (vf ValueFilter) Validate() error {
	if len(vf.Values) == 0 && vf.Regexp == "" {
		return fmt.Errorf("neither values nor regexp is specified")
	}

	return nil
}

// New creates an HTTPFilter.
func New(spec *Spec) *HTTPFilter {
	hf := &HTTPFilter{
		spec: spec,
	}

	for _, vf := range spec.Headers {
		if len(vf.Regexp) != 0 {
			re, err := regexp.Compile(vf.Regexp)
			if err != nil {
				logger.Errorf("BUG: compile regexp %s failed: %v",
					vf.Regexp, err)
				continue
			}
			vf.re = re
		}
	}

	return hf
}

// Filter filters HTTPContext.
func (hf *HTTPFilter) Filter(ctx context.HTTPContext) bool {
	h := ctx.Request().Header()
	for key, vf := range hf.spec.Headers {
		values := h.GetAll(key)
		for _, value := range values {
			if common.StrInSlice(value, vf.Values) {
				return true
			}
			if vf.re != nil && vf.re.MatchString(value) {
				return true
			}
		}

	}

	return false
}

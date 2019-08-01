package mock

import (
	"strings"

	"github.com/megaease/easegateway/pkg/context"
)

type (
	// Mock is plugin Mock.
	Mock struct {
		spec *Spec
		body []byte
	}

	// Spec describes the Mock.
	Spec []*Rule

	// Rule is the mock rule.
	Rule struct {
		Path       string            `yaml:"path,omitempty" v:"omitempty,prefix=/"`
		PathPrefix string            `yaml:"pathPrefix,omitempty" v:"omitempty,prefix=/"`
		Code       int               `yaml:"code" v:"required,omitempty,httpcode"`
		Headers    map[string]string `yaml:"headers" v:"dive,keys,required,endkeys,required"`
		Body       string            `yaml:"body"`
	}
)

// New creates a Mock.
func New(spec *Spec, runtime *Runtime) *Mock {
	return &Mock{
		spec: spec,
	}
}

// Close closes Mock.
// Nothing to do.
func (f *Mock) Close() {}

// Mock fallabcks HTTPContext.
func (f *Mock) Mock(ctx context.HTTPContext) {
	path := ctx.Request().Path()
	w := ctx.Response()

	mock := func(rule *Rule) {
		w.SetStatusCode(rule.Code)
		for key, value := range rule.Headers {
			w.Header().Set(key, value)
		}
		w.SetBody(strings.NewReader(rule.Body))
	}

	for _, rule := range *f.spec {
		if rule.Path == "" && rule.PathPrefix == "" {
			mock(rule)
			return
		}

		if rule.Path == path || strings.HasPrefix(path, rule.PathPrefix) {
			mock(rule)
			return
		}
	}
}

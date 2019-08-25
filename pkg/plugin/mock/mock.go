package mock

import (
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
)

const (
	// Kind is the kind of Mock.
	Kind = "Mock"

	resultMocked = "mocked"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultMocked},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// Mock is plugin Mock.
	Mock struct {
		spec *Spec
		body []byte
	}

	// Spec describes the Mock.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		Rules []*Rule `yaml:"rules"`
	}

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
func New(spec *Spec, prev *Mock) *Mock {
	return &Mock{
		spec: spec,
	}
}

// Handle mocks HTTPContext.
func (m *Mock) Handle(ctx context.HTTPContext) (result string) {
	path := ctx.Request().Path()
	w := ctx.Response()

	mock := func(rule *Rule) {
		w.SetStatusCode(rule.Code)
		for key, value := range rule.Headers {
			w.Header().Set(key, value)
		}
		w.SetBody(strings.NewReader(rule.Body))
		result = resultMocked
	}

	for _, rule := range m.spec.Rules {
		if rule.Path == "" && rule.PathPrefix == "" {
			mock(rule)
			return
		}

		if rule.Path == path || strings.HasPrefix(path, rule.PathPrefix) {
			mock(rule)
			return
		}
	}

	return ""
}

// Status returns status.
func (m *Mock) Status() interface{} {
	return nil
}

// Close closes Mock.
func (m *Mock) Close() {}

package mock

import (
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of Mock.
	Kind = "Mock"

	resultMocked = "mocked"
)

var (
	results = []string{resultMocked}
)

func init() {
	httppipeline.Register(&Mock{})
}

type (
	// Mock is filter Mock.
	Mock struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		body []byte
	}

	// Spec describes the Mock.
	Spec struct {
		Rules []*Rule `yaml:"rules"`
	}

	// Rule is the mock rule.
	Rule struct {
		Path       string            `yaml:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix string            `yaml:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		Code       int               `yaml:"code" jsonschema:"required,format=httpcode"`
		Headers    map[string]string `yaml:"headers" jsonschema:"omitempty"`
		Body       string            `yaml:"body" jsonschema:"omitempty"`
		Delay      string            `yaml:"delay" jsonschema:"omitempty,format=duration"`

		delay time.Duration
	}
)

// Kind returns the kind of Mock.
func (m *Mock) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of Mock.
func (m *Mock) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Mock.
func (m *Mock) Description() string {
	return "Mock mocks the response."
}

// Results returns the results of Mock.
func (m *Mock) Results() []string {
	return results
}

// Init initializes Mock.
func (m *Mock) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	m.pipeSpec, m.spec, m.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	m.reload()
}

// Inherit inherits previous generation of Mock.
func (m *Mock) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	m.Init(pipeSpec, super)
}

func (m *Mock) reload() {
	for _, r := range m.spec.Rules {
		if r.Delay == "" {
			continue
		}
		var err error
		r.delay, err = time.ParseDuration(r.Delay)
		if err != nil {
			logger.Errorf("BUG: parse duration %s failed: %v", r.Delay, err)
		}
	}
}

// Handle mocks HTTPContext.
func (m *Mock) Handle(ctx context.HTTPContext) (result string) {
	result = m.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (m *Mock) handle(ctx context.HTTPContext) (result string) {
	path := ctx.Request().Path()
	w := ctx.Response()

	mock := func(rule *Rule) {
		w.SetStatusCode(rule.Code)
		for key, value := range rule.Headers {
			w.Header().Set(key, value)
		}
		w.SetBody(strings.NewReader(rule.Body))
		result = resultMocked

		if rule.delay < 0 {
			return
		}

		logger.Debugf("delay for %v ...", rule.delay)
		select {
		case <-ctx.Done():
			logger.Debugf("request cancelled in the middle of delay mocking")
		case <-time.After(rule.delay):
		}
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

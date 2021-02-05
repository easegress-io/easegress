package bridge

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of Mock.
	Kind = "Bridge"

	destNotFound     = "destNotFound"
	invokeDestFailed = "invokeDestFailed"

	bridgeDestHeader = "X-Easegateway-Bridge-Dest"
)

func init() {
	httppipeline.Register(&httppipeline.FilterRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{destNotFound, invokeDestFailed},

		Description: `# Bridge Filter

A Bridge Filter route requests to from one pipeline to other pipelines or http proxies under a http server.

1. The upstream filter set the target pipeline/proxy to the http header,  'X-Easegateway-Bridge-Dest'.
2. Bridge will extract the value from 'X-Easegateway-Bridge-Dest' and try to match in the configuration.
   It will send the request if a dest matched. abort the process if no match.
3. Bridge will select the first dest from the filter configuration if there's no header named 'X-Easegateway-Bridge-Dest'`,
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// Bridge is filter Bridge.
	Bridge struct {
		spec *Spec
	}

	// Spec describes the Mock.
	Spec struct {
		httppipeline.FilterMeta `yaml:",inline"`

		Destinations []string `yaml:"destinations" jsonschema:"required,pattern=^[^ \t]+$"`
	}
)

// New creates a Mock.
func New(spec *Spec, prev *Bridge) *Bridge {
	if len(spec.Destinations) <= 0 {
		logger.Errorf("not any destination defined")
	}

	return &Bridge{
		spec: spec,
	}
}

// Handle mocks HTTPContext.
func (m *Bridge) Handle(ctx context.HTTPContext) (result string) {
	if len(m.spec.Destinations) <= 0 {
		panic("not any destination defined")
	}

	r := ctx.Request()
	dest := r.Header().Get(bridgeDestHeader)
	found := false
	if dest == "" {
		logger.Warnf("dest not defined, will choose the first dest: %s", m.spec.Destinations[0])
		dest = m.spec.Destinations[0]
		found = true
	} else {
		for _, d := range m.spec.Destinations {
			if d == dest {
				r.Header().Del(bridgeDestHeader)
				found = true
				break
			}
		}
	}

	if !found {
		logger.Errorf("dest not found: %s", dest)
		return destNotFound
	}

	ro, exists := supervisor.Global.GetRunningObject(dest, supervisor.CategoryPipeline)
	if !exists {
		logger.Errorf("failed invok %s", m.spec.Destinations[0])
		return invokeDestFailed
	}

	handler, ok := ro.Instance().(httpserver.HTTPHandler)
	if !ok {
		logger.Errorf("%s is not a handler", m.spec.Destinations[0])
		return invokeDestFailed
	}

	handler.Handle(ctx)
	return ""
}

// Status returns status.
func (m *Bridge) Status() interface{} {
	return nil
}

// Close closes Mock.
func (m *Bridge) Close() {}

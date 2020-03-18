package bridge

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/scheduler"
)

const (
	// Kind is the kind of Mock.
	Kind = "Bridge"

	destNotFound     = "destNotFound"
	invokeDestFailed = "invokeDestFailed"

	bridgeDestHeader = "X-Easegateway-Bridge-Dest"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{destNotFound, invokeDestFailed},

		Description: `# Bridge Plugin

A Bridge Plugin route requests to from one pipeline to other pipelines or http proxies under a http server.

1. The upstream plugin set the target pipeline/proxy to the http header,  'X-Easegateway-Bridge-Dest'. 
2. Bridge will extract the value from 'X-Easegateway-Bridge-Dest' and try to match in the configuration. 
   It will send the request if a dest matched. abort the process if no match.
3. Bridge will select the first dest from the plugin configuration if there's no header named 'X-Easegateway-Bridge-Dest'`,
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// Mock is plugin Mock.
	Bridge struct {
		spec *Spec
	}

	// Spec describes the Mock.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

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
	if dest == "" {
		logger.Warnf("dest not defined, will choose the first dest: %s", m.spec.Destinations[0])
		err := scheduler.SendHTTPRequet(m.spec.Destinations[0], ctx)
		if err != nil {
			logger.Errorf("failed to invoke %s", m.spec.Destinations[0])
			return invokeDestFailed
		}

		return ""
	}

	for _, d := range m.spec.Destinations {
		if d == dest {
			r.Header().Del(bridgeDestHeader)
			err := scheduler.SendHTTPRequet(d, ctx)
			if err != nil {
				logger.Errorf("failed to invoke %s", m.spec.Destinations[0])
				return invokeDestFailed
			}
			return ""
		}
	}

	logger.Errorf("dest not found: %s", dest)
	return destNotFound
}

// Status returns status.
func (m *Bridge) Status() interface{} {
	return nil
}

// Close closes Mock.
func (m *Bridge) Close() {}

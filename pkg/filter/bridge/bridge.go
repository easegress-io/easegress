package bridge

import (
	"net/http"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of Bridge.
	Kind = "Bridge"

	// Description is the Description of Bridge.
	Description = `# Bridge Filter

A Bridge Filter route requests to from one pipeline to other pipelines or http proxies under a http server.

1. The upstream filter set the target pipeline/proxy to the http header,  'X-Easegateway-Bridge-Dest'.
2. Bridge will extract the value from 'X-Easegateway-Bridge-Dest' and try to match in the configuration.
   It will send the request if a dest matched. abort the process if no match.
3. Bridge will select the first dest from the filter configuration if there's no header named 'X-Easegateway-Bridge-Dest'`

	resultDestinationNotFound     = "destinationNotFound"
	resultInvokeDestinationFailed = "invokeDestinationFailed"

	bridgeDestHeader = "X-Easegateway-Bridge-Dest"
)

var (
	results = []string{resultDestinationNotFound, resultInvokeDestinationFailed}
)

func init() {
	httppipeline.Register(&Bridge{})
}

type (
	// Bridge is filter Bridge.
	Bridge struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec
	}

	// Spec describes the Mock.
	Spec struct {
		Destinations []string `yaml:"destinations" jsonschema:"required,pattern=^[^ \t]+$"`
	}
)

// Kind returns the kind of Bridge.
func (b *Bridge) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Bridge.
func (b *Bridge) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Bridge.
func (b *Bridge) Description() string {
	return Description
}

// Results returns the results of Bridge.
func (b *Bridge) Results() []string {
	return results
}

// Init initializes Bridge.
func (b *Bridge) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	b.pipeSpec, b.spec, b.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	b.reload()
}

// Inherit inherits previous generation of Bridge.
func (b *Bridge) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	b.Init(pipeSpec, super)
}

func (b *Bridge) reload() {
	if len(b.spec.Destinations) <= 0 {
		logger.Errorf("not any destination defined")
	}
}

// Handle builds a bridge for pipeline.
func (b *Bridge) Handle(ctx context.HTTPContext) (result string) {
	result = b.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (b *Bridge) handle(ctx context.HTTPContext) (result string) {
	if len(b.spec.Destinations) <= 0 {
		panic("not any destination defined")
	}

	r := ctx.Request()
	dest := r.Header().Get(bridgeDestHeader)
	found := false
	if dest == "" {
		logger.Warnf("destination not defined, will choose the first dest: %s", b.spec.Destinations[0])
		dest = b.spec.Destinations[0]
		found = true
	} else {
		for _, d := range b.spec.Destinations {
			if d == dest {
				r.Header().Del(bridgeDestHeader)
				found = true
				break
			}
		}
	}

	if !found {
		logger.Errorf("dest not found: %s", dest)
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		return resultDestinationNotFound
	}

	ro, exists := supervisor.Global.GetRunningObject(dest, supervisor.CategoryPipeline)
	if !exists {
		logger.Errorf("failed to get running object %s", b.spec.Destinations[0])
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		return resultDestinationNotFound
	}

	handler, ok := ro.Instance().(protocol.HTTPHandler)
	if !ok {
		logger.Errorf("%s is not a handler", b.spec.Destinations[0])
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		return resultInvokeDestinationFailed
	}

	handler.Handle(ctx)
	return ""
}

// Status returns status.
func (b *Bridge) Status() interface{} {
	return nil
}

// Close closes Bridge.
func (b *Bridge) Close() {}

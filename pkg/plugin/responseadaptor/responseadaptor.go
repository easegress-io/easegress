package responseadaptor

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

const (
	// Kind is the kind of ResponseAdaptor.
	Kind = "ResponseAdaptor"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         nil,
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// ResponseAdaptor is plugin ResponseAdaptor.
	ResponseAdaptor struct {
		spec *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		Header *httpheader.AdaptSpec `yaml:"header" v:"required"`
	}
)

// New creates an HTTPAdaptor.
func New(spec *Spec, prev *ResponseAdaptor) *ResponseAdaptor {
	return &ResponseAdaptor{
		spec: spec,
	}
}

// Handle adapts response.
func (ra *ResponseAdaptor) Handle(ctx context.HTTPContext) string {
	ctx.Response().Header().Adapt(ra.spec.Header)
	return ""
}

// Status returns status.
func (ra *ResponseAdaptor) Status() interface{} { return nil }

// Close closes ResponseAdaptor.
func (ra *ResponseAdaptor) Close() {}

package responseadaptor

import (
	"bytes"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
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

		Header *httpheader.AdaptSpec `yaml:"header" jsonschema:"required"`

		Body string `yaml:"body" jsonschema:"omitempty"`
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
	hte := ctx.Template()
	ctx.Response().Header().Adapt(ra.spec.Header, hte)

	if len(ra.spec.Body) != 0 {
		if hte.HasTemplates(ra.spec.Body) {
			if body, err := hte.Render(ra.spec.Body); err != nil {
				logger.Errorf("BUG responseadaptor render body faile , template %s , err %v",
					ra.spec.Body, err)
			} else {
				ctx.Response().SetBody(bytes.NewReader([]byte(body)))
			}
		} else {
			ctx.Response().SetBody(bytes.NewReader([]byte(ra.spec.Body)))
		}
	}
	return ""
}

// Status returns status.
func (ra *ResponseAdaptor) Status() interface{} { return nil }

// Close closes ResponseAdaptor.
func (ra *ResponseAdaptor) Close() {}

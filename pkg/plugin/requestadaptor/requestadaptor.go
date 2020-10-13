package requestadaptor

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/pathadaptor"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

const (
	// Kind is the kind of RequestAdaptor.
	Kind = "RequestAdaptor"
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
	// RequestAdaptor is plugin RequestAdaptor.
	RequestAdaptor struct {
		spec *Spec

		pa *pathadaptor.PathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		Method string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path   *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
	}
)

// New creates an HTTPAdaptor.
func New(spec *Spec, prev *RequestAdaptor) *RequestAdaptor {
	var pa *pathadaptor.PathAdaptor
	if spec.Path != nil {
		pa = pathadaptor.New(spec.Path)
	}

	return &RequestAdaptor{
		spec: spec,
		pa:   pa,
	}
}

// Handle adapts request.
func (ra *RequestAdaptor) Handle(ctx context.HTTPContext) string {
	r := ctx.Request()
	method, path, header := r.Method(), r.Path(), r.Header()

	if ra.spec.Method != "" && ra.spec.Method != method {
		ctx.AddTag(stringtool.Cat("requestAdaptor: method ",
			method, " adapted to ", ra.spec.Method))
		r.SetMethod(ra.spec.Method)
	}
	if ra.pa != nil {
		adaptedPath := ra.pa.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(stringtool.Cat("requestAdaptor: path ",
				path, " adapted to ", adaptedPath))
		}
		r.SetPath(adaptedPath)
	}
	if ra.spec.Header != nil {
		header.Adapt(ra.spec.Header)
	}

	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} { return nil }

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {}

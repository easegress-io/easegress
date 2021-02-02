package requestadaptor

import (
	"bytes"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
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
	httppipeline.Register(&httppipeline.FilterRecord{
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
	// RequestAdaptor is filter RequestAdaptor.
	RequestAdaptor struct {
		spec *Spec

		pa *pathadaptor.PathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		httppipeline.FilterMeta `yaml:",inline"`

		Method string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path   *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		Body   string                `yaml:"body" jsonschema:"omitempty"`
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
	hte := ctx.Template()
	if ra.spec.Header != nil {
		header.Adapt(ra.spec.Header, hte)
	}

	if len(ra.spec.Body) != 0 {
		if hte.HasTemplates(ra.spec.Body) {
			if body, err := hte.Render(ra.spec.Body); err != nil {
				logger.Errorf("BUG resquest render body faile , template %s , err %v",
					ra.spec.Body, err)
			} else {
				ctx.Request().SetBody(bytes.NewReader([]byte(body)))
			}
		} else {
			ctx.Request().SetBody(bytes.NewReader([]byte(ra.spec.Body)))
		}
	}

	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} { return nil }

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {}

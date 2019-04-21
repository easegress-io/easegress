package httpadaptor

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

type (
	// Spec is HTTPAdaptor Spec.
	Spec struct {
		Request  *Request  `yaml:"request"`
		Response *Response `yaml:"response"`
	}

	// Request describes rules for adapting request.
	Request struct {
		Method string                `yaml:"method" v:"omitempty,httpmethod"`
		Path   *pathAdaptorSpec      `yaml:"path"`
		Header *httpheader.AdaptSpec `yaml:"header"`
	}

	// Response describes rules for adapting response.
	Response struct {
		Header *httpheader.AdaptSpec `yaml:"header"`
	}

	// HTTPAdaptor is HTTP Adaptor.
	HTTPAdaptor struct {
		spec               *Spec
		requestPathAdaptor *pathAdaptor
	}
)

// New creates an HTTPAdaptor.
func New(spec *Spec) *HTTPAdaptor {
	var requestPathAdaptor *pathAdaptor
	if spec.Request != nil && spec.Request.Path != nil {
		requestPathAdaptor = newPathAdaptor(spec.Request.Path)
	}

	return &HTTPAdaptor{
		spec:               spec,
		requestPathAdaptor: requestPathAdaptor,
	}
}

// AdaptRequest adapts request by returning instead of applying to ctx.
func (ha *HTTPAdaptor) AdaptRequest(ctx context.HTTPContext, headerInPlace bool) (
	method, path string, header *httpheader.HTTPHeader) {

	r := ctx.Request()
	method, path, header = r.Method(), r.Path(), r.Header()

	if ha.spec.Request != nil {
		if ha.spec.Request.Method != "" {
			method = ha.spec.Request.Method
		}
		if ha.requestPathAdaptor != nil {
			path = ha.requestPathAdaptor.Adapt(path)
		}
		if ha.spec.Request.Header != nil {
			if !headerInPlace {
				header = header.Copy()
			}
			header.Adapt(ha.spec.Request.Header)
		}
	}

	return
}

// AdaptResponse adapts response by applying to ctx.
func (ha *HTTPAdaptor) AdaptResponse(ctx context.HTTPContext) {
	if ha.spec.Response != nil {
		if ha.spec.Response.Header != nil {
			ctx.Response().Header().Adapt(ha.spec.Response.Header)
		}
	}
}

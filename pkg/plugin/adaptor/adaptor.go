package adaptor

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpadaptor"
)

type (
	// Adaptor is the plugin level wrapper for HTTPAdaptor.
	Adaptor struct {
		ha *httpadaptor.HTTPAdaptor
	}

	// Spec is the plugin level wrapper for HTTPAdaptorSpec.
	Spec httpadaptor.Spec
)

// New creates an Adaptor.
func New(spec *Spec, runtime *Runtime) *Adaptor {
	return &Adaptor{
		ha: httpadaptor.New((*httpadaptor.Spec)(spec)),
	}
}

// Close closes RequestAdaptor.
// Nothing to do.
func (a *Adaptor) Close() {}

// AdaptRequest adapts HTTPContext Requst.
func (a *Adaptor) AdaptRequest(ctx context.HTTPContext) {
	a.ha.AdaptRequest(ctx, true /*headerInPlace*/)
}

// AdaptResponse adapts HTTPContext Response.
func (a *Adaptor) AdaptResponse(ctx context.HTTPContext) {
	a.ha.AdaptResponse(ctx)
}

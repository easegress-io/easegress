package backend

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpbackend"
)

type (
	// Backend is plugin Backend.
	Backend struct {
		spec *Spec

		backend *httpbackend.HTTPBackend
	}

	// Spec describes the Backend.
	Spec = httpbackend.Spec
)

// New creates a Backend.
func New(spec *Spec, runtime *Runtime) *Backend {
	return &Backend{
		spec:    spec,
		backend: httpbackend.New(spec),
	}
}

// Close closes Backend.
func (b *Backend) Close() {}

// Handle handles HTTPContext.
func (b *Backend) Handle(ctx context.HTTPContext) {
	b.backend.HandleWithResponse(ctx)
}

// OnResponseGot is the wrapper for the same func of HTTPBackend.
func (b *Backend) OnResponseGot(fn httpbackend.ResponseGotFunc) {
	b.backend.OnResponseGot(fn)
}

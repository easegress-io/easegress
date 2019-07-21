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
	b := &Backend{
		spec:    spec,
		backend: httpbackend.New(spec),
	}

	runtime.b = b

	return b
}

// Close closes Backend.
func (b *Backend) Close() {
	b.backend.Close()
}

// Handle handles HTTPContext.
func (b *Backend) Handle(ctx context.HTTPContext) {
	b.backend.HandleWithResponse(ctx)
}

// OnResponseGot is the wrapper for the same func of HTTPBackend.
func (b *Backend) OnResponseGot(fn httpbackend.ResponseGotFunc) {
	b.backend.OnResponseGot(fn)
}

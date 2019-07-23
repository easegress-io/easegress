package backend

import (
	"github.com/megaease/easegateway/pkg/util/httpbackend"
)

type (

	// Runtime contains runtime info of Backend.
	Runtime struct {
		b *Backend
	}

	// Status contains status info of Backend.
	Status = httpbackend.Status
)

// NewRuntime creates a Backend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return r.b.backend.Status()
}

// Close closes Runtime.
func (r *Runtime) Close() {}

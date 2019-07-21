package mirrorbackend

import (
	"github.com/megaease/easegateway/pkg/util/httpbackend"
)

type (

	// Runtime contains runtime info of MirrorBackend.
	Runtime struct {
		mb *MirrorBackend
	}

	// Status contains status info of MirrorBackend.
	Status = httpbackend.Status
)

// NewRuntime creates a MirrorBackend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return r.mb.backend.Status()
}

// Close closes Runtime.
func (r *Runtime) Close() {}

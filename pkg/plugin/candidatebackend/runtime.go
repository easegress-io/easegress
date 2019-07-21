package candidatebackend

import (
	"github.com/megaease/easegateway/pkg/util/httpbackend"
)

type (

	// Runtime contains runtime info of CandidateBackend.
	Runtime struct {
		cb *CandidateBackend
	}

	// Status contains status info of CandidateBackend.
	Status = httpbackend.Status
)

// NewRuntime creates a CandidateBackend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return r.cb.backend.Status()
}

// Close closes Runtime.
func (r *Runtime) Close() {}

package candidatebackend

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpbackend"
	"github.com/megaease/easegateway/pkg/util/httpfilter"
)

type (
	// CandidateBackend is plugin CandidateBackend.
	CandidateBackend struct {
		spec *Spec

		backend *httpbackend.HTTPBackend
		filter  *httpfilter.HTTPFilter
	}

	// Spec describes the CandidateBackend.
	Spec struct {
		httpbackend.Spec `yaml:",inline" v:"required"`
		Filter           *httpfilter.Spec `yaml:"filter" v:"required"`
	}
)

// New creates a CandidateBackend.
func New(spec *Spec, runtime *Runtime) *CandidateBackend {
	cb := &CandidateBackend{
		spec:    spec,
		backend: httpbackend.New(&spec.Spec),
		filter:  httpfilter.New(spec.Filter),
	}

	runtime.cb = cb

	return cb
}

// Close closes CandidateBackend.
// Nothing to do.
func (cb *CandidateBackend) Close() {}

// Handle handles HTTPContext.
func (cb *CandidateBackend) Handle(ctx context.HTTPContext) {
	cb.backend.HandleWithResponse(ctx)
}

// Filter filters HTTPContext, return true if it will handle the HTTPContext.
func (cb *CandidateBackend) Filter(ctx context.HTTPContext) bool {
	if cb.filter.Filter(ctx) {
		return true
	}

	return false
}

// OnResponseGot is the wrapper for the same func of HTTPBackend.
func (cb *CandidateBackend) OnResponseGot(fn httpbackend.ResponseGotFunc) {
	cb.backend.OnResponseGot(fn)
}

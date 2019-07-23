package mirrorbackend

import (
	"fmt"
	"net/http"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpbackend"
	"github.com/megaease/easegateway/pkg/util/httpfilter"
)

type (
	// MirrorBackend is plugin MirrorBackend.
	MirrorBackend struct {
		spec *Spec

		client  *http.Client
		backend *httpbackend.HTTPBackend
		filter  *httpfilter.HTTPFilter
	}

	// Spec describes the MirrorBackend.
	Spec struct {
		V                string `yaml:"-" v:"parent"`
		httpbackend.Spec `yaml:",inline" v:"required"`
		Filter           *httpfilter.Spec `yaml:"filter"`
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	if s.Spec.Adaptor != nil && s.Spec.Adaptor.Response != nil {
		return fmt.Errorf("mirrorBackend can't adpat response")
	}

	return nil
}

// New creates a MirrorBackend.
func New(spec *Spec, runtime *Runtime) *MirrorBackend {
	var filter *httpfilter.HTTPFilter
	if spec.Filter != nil {
		filter = httpfilter.New(spec.Filter)
	}

	mb := &MirrorBackend{
		spec: spec,

		client:  http.DefaultClient,
		backend: httpbackend.New(&spec.Spec),
		filter:  filter,
	}

	runtime.mb = mb

	return mb
}

// Close closes MirrorBackend.
// Nothing to do.
func (cb *MirrorBackend) Close() {
	cb.backend.Close()
}

// Handle handles HTTPContext.
func (cb *MirrorBackend) Handle(ctx context.HTTPContext) {
	if cb.filter == nil || cb.filter.Filter(ctx) {
		cb.backend.HandleWithoutResponse(ctx)
	}
}

// OnResponseGot is the wrapper for the same func of HTTPBackend.
func (cb *MirrorBackend) OnResponseGot(fn httpbackend.ResponseGotFunc) {
	cb.backend.OnResponseGot(fn)
}

package httpproxy

import (
	"github.com/megaease/easegateway/pkg/plugin/adaptor"
	"github.com/megaease/easegateway/pkg/plugin/backend"
	"github.com/megaease/easegateway/pkg/plugin/candidatebackend"
	"github.com/megaease/easegateway/pkg/plugin/circuitbreaker"
	"github.com/megaease/easegateway/pkg/plugin/compression"
	"github.com/megaease/easegateway/pkg/plugin/mirrorbackend"
	"github.com/megaease/easegateway/pkg/plugin/ratelimiter"
	"github.com/megaease/easegateway/pkg/plugin/validator"
)

type (

	// Runtime contains all runtime info of HTTPProxy.
	Runtime struct {
		fallback *proxyFallbackRuntime

		validator        *validator.Runtime
		rateLimiter      *ratelimiter.Runtime
		circuitBreaker   *circuitbreaker.Runtime
		adaptor          *adaptor.Runtime
		mirrorBackend    *mirrorbackend.Runtime
		candidateBackend *candidatebackend.Runtime
		backend          *backend.Runtime
		compression      *compression.Runtime
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status struct {
		Timestamp uint64 `yaml:"timestamp"`

		Fallback *proxyFallbackStatus `yaml:"fallback,omitempty"`

		Validator        *validator.Status        `yaml:"validator,omitempty"`
		RateLimiter      *ratelimiter.Status      `yaml:"rateLimiter,omitempty"`
		CircuitBreaker   *circuitbreaker.Status   `yaml:"circuitBreaker,omitempty"`
		Adaptor          *adaptor.Status          `yaml:"adaptor,omitempty"`
		MirrorBackend    *mirrorbackend.Status    `yaml:"mirrorBackend,omitempty"`
		CandidateBackend *candidatebackend.Status `yaml:"candidateBackend,omitempty"`
		Backend          *backend.Status          `yaml:"backend,omitempty"`
		Compression      *compression.Status      `yaml:"compression,omitempty"`
	}
)

// InjectTimestamp injects timestamp.
func (s *Status) InjectTimestamp(t uint64) { s.Timestamp = t }

// NewRuntime creates an HTTPProxy runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) reload(spec *Spec) {
	switch {
	case spec.Fallback != nil && r.fallback == nil:
		r.fallback = newProxyFallbackRuntime()
	case spec.Fallback == nil && r.fallback != nil:
		r.fallback.close()
		r.fallback = nil
	}

	switch {
	case spec.Validator != nil && r.validator == nil:
		r.validator = validator.NewRuntime()
	case spec.Validator == nil && r.validator != nil:
		r.validator.Close()
		r.validator = nil
	}

	switch {
	case spec.RateLimiter != nil && r.rateLimiter == nil:
		r.rateLimiter = ratelimiter.NewRuntime()
	case spec.RateLimiter == nil && r.rateLimiter != nil:
		r.rateLimiter.Close()
		r.rateLimiter = nil
	}

	switch {
	case spec.CircuitBreaker != nil && r.circuitBreaker == nil:
		r.circuitBreaker = circuitbreaker.NewRuntime()
	case spec.CircuitBreaker == nil && r.circuitBreaker != nil:
		r.circuitBreaker.Close()
		r.circuitBreaker = nil
	}

	switch {
	case spec.Adaptor != nil && r.adaptor == nil:
		r.adaptor = adaptor.NewRuntime()
	case spec.Adaptor == nil && r.adaptor != nil:
		r.adaptor.Close()
		r.adaptor = nil
	}

	switch {
	case spec.MirrorBackend != nil && r.mirrorBackend == nil:
		r.mirrorBackend = mirrorbackend.NewRuntime()
	case spec.MirrorBackend == nil && r.mirrorBackend != nil:
		r.mirrorBackend.Close()
		r.mirrorBackend = nil
	}

	switch {
	case spec.CandidateBackend != nil && r.candidateBackend == nil:
		r.candidateBackend = candidatebackend.NewRuntime()
	case spec.CandidateBackend == nil && r.candidateBackend != nil:
		r.candidateBackend.Close()
		r.candidateBackend = nil
	}

	switch {
	case spec.Backend != nil && r.backend == nil:
		r.backend = backend.NewRuntime()
	case spec.Backend == nil && r.backend != nil:
		r.backend.Close()
		r.backend = nil
	}

	switch {
	case spec.Compression != nil && r.compression == nil:
		r.compression = compression.NewRuntime()
	case spec.Compression == nil && r.compression != nil:
		r.compression.Close()
		r.compression = nil
	}
}

// Status returns Status genreated by Runtime.
// NOTE: Caller must not call Status while reloading.
func (r *Runtime) Status() *Status {
	status := &Status{}

	if r.fallback != nil {
		status.Fallback = r.fallback.status()
	}
	if r.validator != nil {
		status.Validator = r.validator.Status()
	}
	if r.rateLimiter != nil {
		status.RateLimiter = r.rateLimiter.Status()
	}
	if r.circuitBreaker != nil {
		status.CircuitBreaker = r.circuitBreaker.Status()
	}
	if r.adaptor != nil {
		status.Adaptor = r.adaptor.Status()
	}
	if r.mirrorBackend != nil {
		status.MirrorBackend = r.mirrorBackend.Status()
	}
	if r.candidateBackend != nil {
		status.CandidateBackend = r.candidateBackend.Status()
	}
	if r.backend != nil {
		status.Backend = r.backend.Status()
	}
	if r.compression != nil {
		status.Compression = r.compression.Status()
	}

	return status
}

// Close closes runtime.
func (r *Runtime) Close() {
	r.reload(&Spec{})
}

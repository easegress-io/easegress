package httpproxy

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/plugin/fallback"
)

const (
	// plugins not supported to fallback
	fallbackPluginNil fallbackPlugin = iota
	fallbackPluginRateLimiter
	fallbackPluginCircuitBreaker
	fallbackPluginCandidateBackend
	fallbackPluginBackend
)

type (
	fallbackPlugin uint8

	// proxyFallback is the HTTPProxy wrapper for plugin fallback.
	proxyFallback struct {
		spec     *proxyFallbackSpec
		fallback *fallback.Fallback
	}

	proxyFallbackSpec struct {
		fallback.Spec `yaml:",inline"`

		ForRateLimiter           bool  `yaml:"forRateLimiter"`
		ForCircuitBreaker        bool  `yaml:"forCircuitBreaker"`
		ForCandidateBackendCodes []int `yaml:"forCandidateBackendCodes" v:"dive,httpcode"`
		ForBackendCodes          []int `yaml:"forBackendCodes" v:"dive,httpcode"`
	}

	proxyFallbackRuntime struct {
		*fallback.Runtime
	}

	proxyFallbackStatus struct {
		*fallback.Status `yaml:",inline"`
	}
)

func newProxyFallback(spec *proxyFallbackSpec, runtime *proxyFallbackRuntime) *proxyFallback {
	return &proxyFallback{
		spec:     spec,
		fallback: fallback.New(&spec.Spec, fallback.NewRuntime()),
	}
}

// getFallbackErr tries to fallback, return fallbackErr if succeed, otherwise nil.
func (f *proxyFallback) getFallbackErr(ctx context.HTTPContext, pt fallbackPlugin, err error) (fallbackErr error) {
SWITCH:
	switch pt {
	case fallbackPluginNil:
		return nil
	case fallbackPluginRateLimiter:
		if !f.spec.ForRateLimiter {
			return nil
		}
		fallbackErr = fmt.Errorf("fallback for rateLimiter failed: %v", err)
	case fallbackPluginCircuitBreaker:
		if !f.spec.ForCircuitBreaker {
			return nil
		}
		fallbackErr = fmt.Errorf("fallback for circuitBreaker failed: %v", err)
	case fallbackPluginCandidateBackend:
		for _, code := range f.spec.ForCandidateBackendCodes {
			if code == ctx.Response().StatusCode() {
				fallbackErr = fmt.Errorf("fallback for candidateBackend failure code: %d", code)
				break SWITCH
			}
		}
		return nil
	case fallbackPluginBackend:
		for _, code := range f.spec.ForBackendCodes {
			if code == ctx.Response().StatusCode() {
				fallbackErr = fmt.Errorf("fallback backend failure code: %d", code)
				break SWITCH
			}
		}
		return nil
	default:
		logger.Errorf("BUG: unknown falbackPlugin: %v", pt)
		return nil
	}

	f.fallback.Fallback(ctx)

	return
}

func (f *proxyFallback) close() {
	f.fallback.Close()
}

func newProxyFallbackRuntime() *proxyFallbackRuntime {
	return &proxyFallbackRuntime{
		Runtime: fallback.NewRuntime(),
	}
}

func (runtime *proxyFallbackRuntime) close() {
	runtime.Runtime.Close()
}

func (runtime *proxyFallbackRuntime) status() *proxyFallbackStatus {
	return nil
}

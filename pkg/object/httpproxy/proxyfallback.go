package httpproxy

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/plugin/fallback"
)

const (
	fallbackPluginRateLimiter fallbackPlugin = iota
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
		ForCandidateBackendCodes []int `yaml:"forCandidateBackendCodes" v:"omitempty,dive,httpcode"`
		ForBackendCodes          []int `yaml:"forBackendCodes" v:"omitempty,dive,httpcode"`
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

func (f *proxyFallback) tryFallback(ctx context.HTTPContext, pt fallbackPlugin, err error) bool {
SWITCH:
	switch pt {
	case fallbackPluginRateLimiter:
		if !f.spec.ForRateLimiter {
			return false
		}
		err = fmt.Errorf("rateLimiter failed: %v", err)
	case fallbackPluginCircuitBreaker:
		if !f.spec.ForCircuitBreaker {
			return false
		}
		err = fmt.Errorf("circuitBreaker failed: %v", err)
	case fallbackPluginCandidateBackend:
		for _, code := range f.spec.ForCandidateBackendCodes {
			if code == ctx.Response().StatusCode() {
				err = fmt.Errorf("candidateBackend failure code: %d", code)
				break SWITCH
			}
		}
		return false
	case fallbackPluginBackend:
		for _, code := range f.spec.ForBackendCodes {
			if code == ctx.Response().StatusCode() {
				err = fmt.Errorf("backend failure code: %d", code)
				break SWITCH
			}
		}
		return false
	default:
		logger.Errorf("BUG: unknown falbackPlugin: %v", pt)
	}

	f.fallback.Fallback(ctx)
	ctx.Cancel(err)

	return true
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

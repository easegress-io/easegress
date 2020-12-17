package backend

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/fallback"
)

const (
	// Kind is the kind of Backend.
	Kind = "Backend"

	resultCircuitBreaker = "circuitBreaker"
	resultFallback       = "fallback"
	resultInternalError  = "interalError"
	resultClientError    = "clientError"
	resultServerError    = "serverError"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results: []string{
			resultCircuitBreaker,
			resultFallback,
			resultInternalError,
			resultClientError,
			resultServerError,
		},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

var (
	// All Backend instances use one globalClient in order to reuse
	// some resounces such as keepalive connections.
	globalClient = &http.Client{
		// NOTE: Timeout could be no limit, real client or server could cancel it.
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				// NOTE: Could make it an paramenter,
				// when the requests need cross WAN.
				InsecureSkipVerify: true,
			},
			DisableCompression: false,
			// NOTE: The large number of Idle Connctions can
			// reduce overhead of building connections.
			MaxIdleConns:          10240,
			MaxIdleConnsPerHost:   512,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

type (
	// Backend is the plugin Backend.
	Backend struct {
		spec *Spec

		fallback *fallback.Fallback

		mainPool      *pool
		candidatePool *pool
		mirrorPool    *pool

		compression *compression
	}

	// Spec describes the Backend.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		Fallback      *FallbackSpec    `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		MainPool      *PoolSpec        `yaml:"mainPool" jsonschema:"required"`
		CandidatePool *PoolSpec        `yaml:"candidatePool,omitempty" jsonschema:"omitempty"`
		MirrorPool    *PoolSpec        `yaml:"mirrorPool,omitempty" jsonschema:"omitempty"`
		FailureCodes  []int            `yaml:"failureCodes" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
		Compression   *CompressionSpec `yaml:"compression,omitempty" jsonschema:"omitempty"`
	}

	// FallbackSpec describes the fallback policy.
	FallbackSpec struct {
		ForCodes          bool `yaml:"forCodes"`
		ForCircuitBreaker bool `yaml:"forCircuitBreaker"`
		fallback.Spec     `yaml:",inline"`
	}

	// Status wraps httpstat.Status.
	Status struct {
		MainPool      *PoolStatus `yaml:"mainPool"`
		CandidatePool *PoolStatus `yaml:"candidatePool,omitempty"`
		MirrorPool    *PoolStatus `yaml:"mirrorPool,omitempty"`
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	// NOTE: The tag of v parent may be behind mainPool.
	if s.MainPool == nil {
		return fmt.Errorf("mainPool is required")
	}

	if s.MainPool.Filter != nil {
		return fmt.Errorf("filter must be empty in mainPool")
	}

	if s.CandidatePool != nil && s.CandidatePool.Filter == nil {
		return fmt.Errorf("filter of candidatePool is required")
	}

	if s.MirrorPool != nil {
		if s.MirrorPool.Filter == nil {
			return fmt.Errorf("filter of mirrorPool is required")
		}
		if s.MirrorPool.MemoryCache != nil {
			return fmt.Errorf("memoryCache must be empty in mirrorPool")
		}
		if s.MirrorPool.CircuitBreaker != nil {
			return fmt.Errorf("circuitBreaker must be empty in mirrorPool")
		}
	}

	if len(s.FailureCodes) == 0 {
		if s.Fallback != nil {
			return fmt.Errorf("fallback needs failureCodes")
		}
		if s.MainPool.CircuitBreaker != nil {
			return fmt.Errorf("circuitBreaker need failureCodes")
		}
		if s.CandidatePool != nil && s.CandidatePool.CircuitBreaker != nil {
			return fmt.Errorf("circuitBreaker need failureCodes")
		}
	}

	return nil
}

// New creates a Backend.
func New(spec *Spec, prev *Backend) *Backend {
	if prev != nil {
		prev.Close()
	}

	b := &Backend{
		spec: spec,
		mainPool: newPool(spec.MainPool, "backend#main",
			true /*writeResponse*/, spec.FailureCodes),
	}

	if spec.Fallback != nil {
		b.fallback = fallback.New(&spec.Fallback.Spec)
	}

	if spec.CandidatePool != nil {
		b.candidatePool = newPool(spec.CandidatePool, "backend#candidate",
			true /*writeResponse*/, spec.FailureCodes)
	}
	if spec.MirrorPool != nil {
		b.mirrorPool = newPool(spec.MirrorPool, "backend#mirror",
			false /*writeResponse*/, spec.FailureCodes)
	}

	if spec.Compression != nil {
		b.compression = newcompression(spec.Compression)
	}

	return b
}

// Status returns Backend status.
func (b *Backend) Status() *Status {
	s := &Status{
		MainPool: b.mainPool.status(),
	}
	if b.candidatePool != nil {
		s.CandidatePool = b.candidatePool.status()
	}
	if b.mirrorPool != nil {
		s.MirrorPool = b.mirrorPool.status()
	}
	return s
}

// Close closes Backend.
func (b *Backend) Close() {
	b.mainPool.close()

	if b.candidatePool != nil {
		b.candidatePool.close()
	}

	if b.mirrorPool != nil {
		b.mirrorPool.close()
	}
}

func (b *Backend) fallbackForCircuitBreaker(ctx context.HTTPContext) bool {
	if b.fallback != nil && b.spec.Fallback.ForCircuitBreaker {
		b.fallback.Fallback(ctx)
		return true
	}
	return false
}
func (b *Backend) fallbackForCodes(ctx context.HTTPContext) bool {
	if b.fallback != nil && b.spec.Fallback.ForCodes {
		for _, code := range b.spec.FailureCodes {
			if ctx.Response().StatusCode() == code {
				b.fallback.Fallback(ctx)
				return true
			}
		}
	}
	return false
}

// Handle handles HTTPContext.
func (b *Backend) Handle(ctx context.HTTPContext) (result string) {
	if b.mirrorPool != nil && b.mirrorPool.filter.Filter(ctx) {
		master, slave := newMasterSlaveReader(ctx.Request().Body())
		ctx.Request().SetBody(master)

		wg := &sync.WaitGroup{}
		wg.Add(1)
		defer func() {
			if result == "" {
				// NOTE: Waiting for mirrorPool finishing
				// only if mainPool/candidatePool handled
				// with normal result.
				wg.Wait()
			}
		}()

		go func() {
			defer wg.Done()
			b.mirrorPool.handle(ctx, slave)
		}()
	}

	var p *pool
	if b.candidatePool != nil && b.candidatePool.filter.Filter(ctx) {
		p = b.candidatePool
	} else {
		p = b.mainPool
	}

	if p.memoryCache != nil && p.memoryCache.Load(ctx) {
		return ""
	}

	if p.circuitBreaker != nil {
		var err error
		result, err = p.circuitBreaker.protect(ctx, ctx.Request().Body(), p.handle)
		if err != nil {
			if b.fallbackForCircuitBreaker(ctx) {
				return resultFallback
			}
			return resultCircuitBreaker
		}
	} else {
		result = p.handle(ctx, ctx.Request().Body())
	}

	if result != "" {
		return result
	}

	if b.fallbackForCodes(ctx) {
		return resultFallback
	}

	// compression and memoryCache only work for
	// normal traffic from real backend servers.
	if b.compression != nil {
		b.compression.compress(ctx)
	}

	if p.memoryCache != nil {
		p.memoryCache.Store(ctx)
	}

	return ""
}

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
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/fallback"
)

const (
	// Kind is the kind of Backend.
	Kind = "Backend"

	resultFallback      = "fallback"
	resultInternalError = "interalError"
	resultClientError   = "clientError"
	resultServerError   = "serverError"
)

var (
	results = []string{
		resultFallback,
		resultInternalError,
		resultClientError,
		resultServerError,
	}
)

func init() {
	httppipeline.Register(&Backend{})
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
	// Backend is the filter Backend.
	Backend struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		fallback *fallback.Fallback

		mainPool      *pool
		candidatePool []*pool
		mirrorPool    *pool

		compression *compression
	}

	// Spec describes the Backend.
	Spec struct {
		httppipeline.FilterMetaSpec `yaml:",inline"`

		Fallback      *FallbackSpec    `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		MainPool      *PoolSpec        `yaml:"mainPool" jsonschema:"required"`
		CandidatePool []*PoolSpec      `yaml:"candidatePool,omitempty" jsonschema:"omitempty"`
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

	// Status is the status of Backend.
	Status struct {
		MainPool      *PoolStatus   `yaml:"mainPool"`
		CandidatePool []*PoolStatus `yaml:"candidatePool,omitempty"`
		MirrorPool    *PoolStatus   `yaml:"mirrorPool,omitempty"`
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

	if s.CandidatePool != nil {
		for _, v := range s.CandidatePool {
			if v.Filter == nil {
				return fmt.Errorf("filter of candidatePool is required")
			}
		}
	}

	if s.MirrorPool != nil {
		if s.MirrorPool.Filter == nil {
			return fmt.Errorf("filter of mirrorPool is required")
		}
		if s.MirrorPool.MemoryCache != nil {
			return fmt.Errorf("memoryCache must be empty in mirrorPool")
		}
	}

	if len(s.FailureCodes) == 0 {
		if s.Fallback != nil {
			return fmt.Errorf("fallback needs failureCodes")
		}
	}

	return nil
}

// Kind returns the kind of Backend.
func (b *Backend) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Backend.
func (b *Backend) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Backend.
func (b *Backend) Description() string {
	return "Backend sets the proxy of backend servers"
}

// Results returns the results of Backend.
func (b *Backend) Results() []string {
	return results
}

// Init initializes Backend.
func (b *Backend) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	b.pipeSpec, b.spec, b.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	b.reload()
}

// Inherit inherits previous generation of Backend.
func (b *Backend) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	b.Init(pipeSpec, super)
}

func (b *Backend) reload() {
	b.mainPool = newPool(b.spec.MainPool, "backend#main",
		true /*writeResponse*/, b.spec.FailureCodes)

	if b.spec.Fallback != nil {
		b.fallback = fallback.New(&b.spec.Fallback.Spec)
	}

	if b.spec.CandidatePool != nil {
		var candidatePool []*pool
		for k, _ := range b.spec.CandidatePool {
			candidatePool = append(candidatePool, newPool(b.spec.CandidatePool[k], fmt.Sprintf("backedn#candidate#%d", k),
				true, b.spec.FailureCodes))
		}
		b.candidatePool = candidatePool
	}
	if b.spec.MirrorPool != nil {
		b.mirrorPool = newPool(b.spec.MirrorPool, "backend#mirror",
			false /*writeResponse*/, b.spec.FailureCodes)
	}

	if b.spec.Compression != nil {
		b.compression = newcompression(b.spec.Compression)
	}
}

// Status returns Backend status.
func (b *Backend) Status() interface{} {
	s := &Status{
		MainPool: b.mainPool.status(),
	}
	if b.candidatePool != nil {
		for k, _ := range b.candidatePool {
			s.CandidatePool = append(s.CandidatePool, b.candidatePool[k].status())
		}
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
		for _, v := range b.candidatePool {
			v.close()
		}
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
	if b.candidatePool != nil {
		for k, v := range b.candidatePool {
			if v.filter.Filter(ctx) {
				p = b.candidatePool[k]
				break
			}
		}
	} else {
		p = b.mainPool
	}

	if p.memoryCache != nil && p.memoryCache.Load(ctx) {
		return ""
	}

	result = ctx.ExecuteHandlerWithWrapper(func() string {
		return p.handle(ctx, ctx.Request().Body())
	})

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

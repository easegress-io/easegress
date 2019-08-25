package backend

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/durationreadcloser"
	"github.com/megaease/easegateway/pkg/util/fallback"
	"github.com/megaease/easegateway/pkg/util/httpfilter"
	"github.com/megaease/easegateway/pkg/util/httpstat"
	"github.com/megaease/easegateway/pkg/util/memorycache"
)

const (
	// Kind is the kind of Backend.
	Kind = "Backend"

	resultCircuitBreaker = "circuitBreaker"
	resultFallback       = "fallback"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultCircuitBreaker, resultFallback},
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

	pool struct {
		filter *httpfilter.HTTPFilter

		servers        *servers
		httpStat       *httpstat.HTTPStat
		count          uint64 // for roundRobin
		memoryCache    *memorycache.MemoryCache
		circuitBreaker *circuitBreaker
	}

	// Spec describes the Backend.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		httppipeline.PluginMeta `yaml:",inline"`

		Fallback      *fallbackSpec    `yaml:"fallback"`
		MainPool      *poolSpec        `yaml:"mainPool" v:"required"`
		CandidatePool *poolSpec        `yaml:"candidatePool" v:"omitempty"`
		MirrorPool    *poolSpec        `yaml:"mirrorPool" v:"omitempty"`
		FailureCodes  []int            `yaml:"failureCodes" v:"omitempty,dive,httpcode"`
		Compression   *CompressionSpec `yaml:"compression"`
	}

	fallbackSpec struct {
		ForCodes          bool `yaml:"forCodes"`
		ForCircuitBreaker bool `yaml:"forCircuitBreaker"`
		fallback.Spec     `yaml:",inline"`
	}

	// poolSpec decribes a pool of servers.
	poolSpec struct {
		V string `yaml:"-" v:"parent"`

		Filter         *httpfilter.Spec    `yaml:"filter,omitempty"`
		ServersTags    []string            `yaml:"serversTags" v:"unique,dive,required"`
		Servers        []*server           `yaml:"servers" v:"required,dive"`
		LoadBalance    *loadBalance        `yaml:"loadBalance" v:"required"`
		MemoryCache    *memorycache.Spec   `yaml:"memoryCache,omitempty"`
		CircuitBreaker *circuitBreakerSpec `yaml:"circuitBreaker,omitempty"`
	}

	poolStatus struct {
		Stat           *httpstat.Status `yaml:"stat"`
		CircuitBreaker string           `yalm:"circuitBreaker,omitempty"`
	}

	// Status wraps httpstat.Status.
	Status struct {
		MainPool      *poolStatus `yaml:"mainPool"`
		CandidatePool *poolStatus `yaml:"candidatePool,omitempty"`
		MirrorPool    *poolStatus `yaml:"mirrorPool,omitempty"`
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

// Validate validates poolSpec.
func (s poolSpec) Validate() error {
	serversGotWeight := 0
	for _, server := range s.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(s.Servers) {
		return fmt.Errorf("not all servers have weight(%d/%d)",
			serversGotWeight, len(s.Servers))
	}

	servers := newServers(&s)
	if servers.len() == 0 {
		return fmt.Errorf("serversTags picks none of servers")
	}

	return nil
}

// New creates a Backend.
func New(spec *Spec, prev *Backend) *Backend {
	b := &Backend{
		spec:     spec,
		mainPool: newPool(spec.MainPool, spec.FailureCodes),
	}

	if spec.Fallback != nil {
		b.fallback = fallback.New(&spec.Fallback.Spec)
	}

	if spec.CandidatePool != nil {
		b.candidatePool = newPool(spec.CandidatePool, spec.FailureCodes)
	}
	if spec.MirrorPool != nil {
		b.mirrorPool = newPool(spec.MirrorPool, spec.FailureCodes)
	}

	if spec.Compression != nil {
		b.compression = newcompression(spec.Compression)
	}

	return b
}

func newPool(spec *poolSpec, failureCodes []int) *pool {
	var filter *httpfilter.HTTPFilter
	if spec.Filter != nil {
		filter = httpfilter.New(spec.Filter)
	}

	var memoryCache *memorycache.MemoryCache
	if spec.MemoryCache != nil {
		memoryCache = memorycache.New(spec.MemoryCache)
	}

	var cb *circuitBreaker
	if spec.CircuitBreaker != nil {
		cb = newCircuitBreaker(spec.CircuitBreaker, failureCodes)
	}

	return &pool{
		filter:         filter,
		servers:        newServers(spec),
		httpStat:       httpstat.New(),
		memoryCache:    memoryCache,
		circuitBreaker: cb,
	}
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
func (b *Backend) Close() {}

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
		result, err := b.mirrorPool.handleWithoutResponse(ctx)
		if err != nil {
			ctx.AddTag(fmt.Sprintf("mirrorBackendFailed: %v", err))
		}
		defer func() {
			res := <-result
			if res != "" {
				ctx.AddTag(fmt.Sprintf("mirrorBackendResult: %s", res))
			}
		}()
	}

	var p *pool
	if b.candidatePool != nil && b.candidatePool.filter.Filter(ctx) {
		p = b.candidatePool
	} else {
		p = b.mainPool
	}

	if p.memoryCache != nil && p.memoryCache.Load(ctx) {
		return
	}

	if p.circuitBreaker != nil {
		if p.circuitBreaker.protect(ctx, p.handle) != nil {
			if b.fallbackForCircuitBreaker(ctx) {
				return resultFallback
			}
			return resultCircuitBreaker
		}
	} else {
		// TODO: fix handle failed situation, such as connection refused.
		p.handle(ctx)
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

	return
}

func (p *pool) status() *poolStatus {
	s := &poolStatus{Stat: p.httpStat.Status()}
	if p.circuitBreaker != nil {
		s.CircuitBreaker = p.circuitBreaker.status()
	}
	return s
}

func (p *pool) handle(ctx context.HTTPContext) {
	r := ctx.Request()
	w := ctx.Response()

	server := p.servers.next(ctx)
	ctx.AddTag(fmt.Sprintf("backendAddr: %s", server.URL))

	url := server.URL + r.Path()
	if r.Query() != "" {
		url += "?" + r.Query()
	}
	req, err := http.NewRequest(r.Method(), url, r.Body())
	if err != nil {
		logger.Errorf("BUG: new request failed: %v", err)
		w.SetStatusCode(http.StatusInternalServerError)
		ctx.AddTag(fmt.Sprintf("backendBug: %v", err))
		return
	}
	req.Header = r.Header().Std()

	var (
		startTime     time.Time
		firstByteTime time.Time
	)
	trace := &httptrace.ClientTrace{
		GetConn: func(_ string) {
			startTime = time.Now()
		},
		GotFirstResponseByte: func() {
			firstByteTime = time.Now()
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(ctx, trace))

	resp, err := globalClient.Do(req)
	if err != nil {
		w.SetStatusCode(http.StatusServiceUnavailable)
		ctx.AddTag(fmt.Sprintf("backendErr: %v", err))
		return
	}

	w.SetStatusCode(resp.StatusCode)
	ctx.AddTag(fmt.Sprintf("backendCode: %d", resp.StatusCode))
	w.Header().AddFromStd(resp.Header)
	body := durationreadcloser.New(resp.Body)
	w.SetBody(body)

	ctx.OnFinish(func() {
		totalDuration := firstByteTime.Sub(startTime) + body.Duration()
		ctx.AddTag(fmt.Sprintf("backendDuration: %v", totalDuration))
		p.httpStat.Stat(ctx)
	})
}

// handleWithoutResponse handles HTTPContext without response.
func (p *pool) handleWithoutResponse(ctx context.HTTPContext) (chan string, error) {
	r := ctx.Request()

	server := p.servers.next(ctx)
	ctx.AddTag(fmt.Sprintf("mirrorBackendAddr: %s", server.URL))

	url := server.URL + r.Path()
	if r.Query() != "" {
		url += "?" + r.Query()
	}
	req, err := http.NewRequest(r.Method(), url, r.Body())
	if err != nil {
		logger.Errorf("BUG: new request failed: %v", err)
		return nil, fmt.Errorf("new request failed: %v", err)
	}
	req.Header = r.Header().Std()

	result := make(chan string)
	go func() {
		// NOTE: The Do func will consume a lot of time
		// if the server is slow. So we'd better do it asynchronously.
		resp, err := globalClient.Do(req)
		if err != nil {
			result <- fmt.Sprintf("mirrorBackendFailed:%v", err)
			return
		}
		result <- ""
		ctx.OnFinish(func() {
			p.httpStat.Stat(ctx)
		})
		// NOTE: Need to be read to completion and closed.
		// Reference: https://golang.org/pkg/net/http/#Response
		defer resp.Body.Close()
		io.Copy(ioutil.Discard, resp.Body)
	}()

	return result, nil
}

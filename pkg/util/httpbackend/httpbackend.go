package httpbackend

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
	"github.com/megaease/easegateway/pkg/util/durationreadcloser"
	"github.com/megaease/easegateway/pkg/util/httpadaptor"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/httpstat"
	"github.com/megaease/easegateway/pkg/util/memorycache"
)

var (
	// All HTTPBackend instances use one globalClient in order to reuse
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
	// ResponseGotFunc is the function type for
	// instantly calling back after getting real response.
	ResponseGotFunc = func(ctx context.HTTPContext)

	// HTTPBackend is the common HTTPBackend.
	HTTPBackend struct {
		spec *Spec

		servers  *servers
		httpStat *httpstat.HTTPStat

		responseGotFuncs []ResponseGotFunc

		// NOTE: Will use its own client instead of globalClient,
		// if some arguments need to be exposed to admin.
		client      *http.Client
		count       uint64 // for roundRobin
		adaptor     *httpadaptor.HTTPAdaptor
		memoryCache *memorycache.MemoryCache
	}

	// Spec describes the HTTPBackend.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		ServersTags []string          `yaml:"serversTags" v:"unique,dive,required"`
		Servers     []*Server         `yaml:"servers" v:"required,dive"`
		LoadBalance *LoadBalance      `yaml:"loadBalance" v:"required"`
		Adaptor     *httpadaptor.Spec `yaml:"adaptor"`
		MemoryCache *memorycache.Spec `yaml:"memoryCache"`
	}

	// Status wraps httpstat.Status.
	Status = httpstat.Status
)

// Validate validates Spec.
func (s Spec) Validate() error {
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

// New creates a HTTPBackend.
func New(spec *Spec) *HTTPBackend {
	var adaptor *httpadaptor.HTTPAdaptor
	if spec.Adaptor != nil {
		adaptor = httpadaptor.New(spec.Adaptor)
	}
	var memoryCache *memorycache.MemoryCache
	if spec.MemoryCache != nil {
		memoryCache = memorycache.New(spec.MemoryCache)
	}

	servers := newServers(spec)
	return &HTTPBackend{
		spec:        spec,
		servers:     servers,
		httpStat:    httpstat.New(),
		client:      globalClient,
		adaptor:     adaptor,
		memoryCache: memoryCache,
	}
}

func (b *HTTPBackend) adaptRequest(ctx context.HTTPContext, headerInPlace bool) (
	method, path string, header *httpheader.HTTPHeader) {
	r := ctx.Request()
	method, path, header = r.Method(), r.Path(), r.Header()
	if b.adaptor != nil {
		return b.adaptor.AdaptRequest(ctx, headerInPlace)
	}
	return
}

func (b *HTTPBackend) adaptResponse(ctx context.HTTPContext) {
	if b.adaptor != nil {
		b.adaptor.AdaptResponse(ctx)
	}
}

// Status returns HTTPBackend status.
func (b *HTTPBackend) Status() *Status {
	return b.httpStat.Status()
}

// OnResponseGot registers ResponseGotFunc.
func (b *HTTPBackend) OnResponseGot(fn ResponseGotFunc) {
	b.responseGotFuncs = append(b.responseGotFuncs, fn)
}

// HandleWithResponse handles HTTPContext with returning response.
func (b *HTTPBackend) HandleWithResponse(ctx context.HTTPContext) {
	if b.memoryCache != nil {
		if b.memoryCache.Load(ctx) {
			return
		}
		defer b.memoryCache.Store(ctx)
	}

	r := ctx.Request()
	w := ctx.Response()

	server := b.servers.next(ctx)
	ctx.AddTag(fmt.Sprintf("backendAddr:%s", server.URL))

	method, path, header := b.adaptRequest(ctx, true /*headerInPlace*/)
	url := server.URL + path
	if r.Query() != "" {
		url += "?" + r.Query()
	}
	req, err := http.NewRequest(method, url, r.Body())
	if err != nil {
		logger.Errorf("BUG: new request failed: %v", err)
		w.SetStatusCode(http.StatusInternalServerError)
		ctx.AddTag(fmt.Sprintf("backendBug:%s", err.Error()))
		return
	}
	req.Header = header.Std()

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

	resp, err := b.client.Do(req)
	if err != nil {
		w.SetStatusCode(http.StatusServiceUnavailable)
		ctx.AddTag(fmt.Sprintf("backendErr:%s", err.Error()))
		return
	}

	w.SetStatusCode(resp.StatusCode)
	ctx.AddTag(fmt.Sprintf("backendCode:%d", resp.StatusCode))
	w.Header().AddFromStd(resp.Header)
	body := durationreadcloser.New(resp.Body)
	w.SetBody(body)

	for _, fn := range b.responseGotFuncs {
		fn(ctx)
	}

	ctx.OnFinish(func() {
		totalDuration := firstByteTime.Sub(startTime) + body.Duration()
		ctx.AddTag(fmt.Sprintf("backendDuration:%v", totalDuration))
		b.httpStat.Stat(ctx)
	})
}

// HandleWithoutResponse handles HTTPContext withou returning response.
func (b *HTTPBackend) HandleWithoutResponse(ctx context.HTTPContext) {
	r := ctx.Request()

	server := b.servers.next(ctx)
	ctx.AddTag(fmt.Sprintf("mirrorBackendAddr:%s", server.URL))

	method, path, header := b.adaptRequest(ctx, false /*headerInPlace*/)
	url := server.URL + path
	if r.Query() != "" {
		url += "?" + r.Query()
	}
	req, err := http.NewRequest(method, url, r.Body())
	if err != nil {
		logger.Errorf("BUG: new request failed: %v", err)
		return
	}
	req.Header = header.Std()

	resp, err := b.client.Do(req)
	if err != nil {
		ctx.AddTag(fmt.Sprintf("mirrorBackendFailed:%v", err))
		return
	}
	ctx.OnFinish(func() {
		b.httpStat.Stat(ctx)
	})

	go func() {
		// NOTE: Need to be read to completion and closed.
		// Reference: https://golang.org/pkg/net/http/#Response
		defer resp.Body.Close()
		io.Copy(ioutil.Discard, resp.Body)
	}()
}

// Close closes HTTPBackend.
func (b *HTTPBackend) Close() {}

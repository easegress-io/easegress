package backend

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/callbackreader"
	"github.com/megaease/easegateway/pkg/util/httpfilter"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/httpstat"
	"github.com/megaease/easegateway/pkg/util/memorycache"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

type (
	pool struct {
		tagPrefix     string
		writeResponse bool

		filter *httpfilter.HTTPFilter

		servers        *servers
		httpStat       *httpstat.HTTPStat
		count          uint64 // for roundRobin
		memoryCache    *memorycache.MemoryCache
		circuitBreaker *circuitBreaker
	}

	// poolSpec decribes a pool of servers.
	poolSpec struct {
		Filter         *httpfilter.Spec    `yaml:"filter,omitempty" jsonschema:"omitempty"`
		ServersTags    []string            `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
		Servers        []*server           `yaml:"servers" jsonschema:"required,minItems=1"`
		LoadBalance    *loadBalance        `yaml:"loadBalance" jsonschema:"required"`
		MemoryCache    *memorycache.Spec   `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
		CircuitBreaker *circuitBreakerSpec `yaml:"circuitBreaker,omitempty" jsonschema:"omitempty"`
	}

	poolStatus struct {
		Stat           *httpstat.Status `yaml:"stat"`
		CircuitBreaker string           `yaml:"circuitBreaker,omitempty"`
	}
)

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

func newPool(spec *poolSpec, tagPrefix string,
	writeResponse bool, failureCodes []int) *pool {
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
		tagPrefix:     tagPrefix,
		writeResponse: writeResponse,

		filter:         filter,
		servers:        newServers(spec),
		httpStat:       httpstat.New(),
		memoryCache:    memoryCache,
		circuitBreaker: cb,
	}
}

func (p *pool) status() *poolStatus {
	s := &poolStatus{Stat: p.httpStat.Status()}
	if p.circuitBreaker != nil {
		s.CircuitBreaker = p.circuitBreaker.status()
	}
	return s
}

func (p *pool) handle(ctx context.HTTPContext, reqBody io.Reader) string {
	addTag := func(subPerfix, msg string) {
		ctx.Lock()
		ctx.AddTag(stringtool.Cat(p.tagPrefix, "#", subPerfix, ": ", msg))
		ctx.Unlock()
	}

	w := ctx.Response()

	server := p.servers.next(ctx)
	addTag("addr", server.URL)

	req, err := p.prepareRequest(ctx, server, reqBody)
	if err != nil {
		msg := stringtool.Cat("prepare request failed: ", err.Error())
		logger.Errorf("BUG: %s", msg)
		addTag("bug", msg)
		w.SetStatusCode(http.StatusInternalServerError)
		return resultInternalError
	}

	resp, err := p.doRequest(ctx, req)
	if err != nil {
		addTag("trace", req.detail())
		if ctx.ClientDisconnected() {
			// NOTE: The HTTPContext will set 499 by itself if client is Disconnected.
			// w.SetStatusCode((499)
			return resultClientError
		}

		w.SetStatusCode(http.StatusServiceUnavailable)
		return resultServerError
	}

	addTag("code", strconv.Itoa(resp.StatusCode))

	ctx.Lock()
	defer ctx.Unlock()
	respBody := p.statRequestResponse(ctx, req, resp)

	if p.writeResponse {
		w.SetStatusCode(resp.StatusCode)
		w.Header().AddFromStd(resp.Header)
		w.SetBody(respBody)

		return ""
	}

	go func() {
		// NOTE: Need to be read to completion and closed.
		// Reference: https://golang.org/pkg/net/http/#Response
		// And we do NOT do statistics of duration and respSize
		// for it, because we can't wait for it to finish.
		defer resp.Body.Close()
		io.Copy(ioutil.Discard, resp.Body)
	}()

	return ""
}

func (p *pool) prepareRequest(ctx context.HTTPContext, server *server, reqBody io.Reader) (req *request, err error) {
	return p.newRequest(ctx, server, reqBody)
}

func (p *pool) doRequest(ctx context.HTTPContext, req *request) (*http.Response, error) {
	resp, err := globalClient.Do(req.std)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (p *pool) statRequestResponse(ctx context.HTTPContext, req *request, resp *http.Response) io.Reader {
	var count int

	callbackBody := callbackreader.New(resp.Body)
	callbackBody.OnAfter(func(num int, p []byte, n int, err error) ([]byte, int, error) {
		count += n
		if err == io.EOF {
			req.finish()
		}

		return p, n, err
	})

	ctx.OnFinish(func() {
		ctx.AddTag(stringtool.Cat(p.tagPrefix, fmt.Sprintf("#duration: %s", req.total())))

		metric := &httpstat.Metric{
			StatusCode: resp.StatusCode,
			Duration:   req.total(),
			ReqSize:    ctx.Request().Size(),
			RespSize:   uint64(responseMetaSize(resp) + count),
		}
		if !p.writeResponse {
			metric.RespSize = 0
		}
		p.httpStat.Stat(metric)
	})

	return callbackBody
}

func responseMetaSize(resp *http.Response) int {
	text := http.StatusText(resp.StatusCode)
	if text == "" {
		text = "status code " + strconv.Itoa(resp.StatusCode)
	}

	// NOTE: We don't use httputil.DumpResponse because it does not
	// completely output plain HTTP Request.

	headerDump := httpheader.New(resp.Header).Dump()

	respMeta := stringtool.Cat(resp.Proto, " ", strconv.Itoa(resp.StatusCode), " ", text, "\r\n",
		headerDump, "\r\n\r\n")

	return len(respMeta)
}

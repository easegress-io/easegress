/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/opentracing/opentracing-go"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/callbackreader"
	"github.com/megaease/easegress/pkg/util/httpfilter"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/httpstat"
	"github.com/megaease/easegress/pkg/util/memorycache"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type (
	pool struct {
		spec *PoolSpec

		tagPrefix     string
		writeResponse bool

		filter *httpfilter.HTTPFilter

		servers     *servers
		httpStat    *httpstat.HTTPStat
		memoryCache *memorycache.MemoryCache
	}

	// PoolSpec describes a pool of servers.
	PoolSpec struct {
		SpanName        string            `yaml:"spanName" jsonschema:"omitempty"`
		Filter          *httpfilter.Spec  `yaml:"filter" jsonschema:"omitempty"`
		ServersTags     []string          `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
		Servers         []*Server         `yaml:"servers" jsonschema:"omitempty"`
		ServiceRegistry string            `yaml:"serviceRegistry" jsonschema:"omitempty"`
		ServiceName     string            `yaml:"serviceName" jsonschema:"omitempty"`
		LoadBalance     *LoadBalance      `yaml:"loadBalance" jsonschema:"required"`
		MemoryCache     *memorycache.Spec `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
	}

	// PoolStatus is the status of Pool.
	PoolStatus struct {
		Stat *httpstat.Status `yaml:"stat"`
	}
)

// Validate validates poolSpec.
func (s PoolSpec) Validate() error {
	if s.ServiceName == "" && len(s.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

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

	if s.ServiceName == "" {
		servers := newStaticServers(s.Servers, s.ServersTags, s.LoadBalance)
		if servers.len() == 0 {
			return fmt.Errorf("serversTags picks none of servers")
		}
	}

	return nil
}

func newPool(super *supervisor.Supervisor, spec *PoolSpec, tagPrefix string,
	writeResponse bool, failureCodes []int) *pool {

	var filter *httpfilter.HTTPFilter
	if spec.Filter != nil {
		filter = httpfilter.New(spec.Filter)
	}

	var memoryCache *memorycache.MemoryCache
	if spec.MemoryCache != nil {
		memoryCache = memorycache.New(spec.MemoryCache)
	}

	return &pool{
		spec: spec,

		tagPrefix:     tagPrefix,
		writeResponse: writeResponse,

		filter:      filter,
		servers:     newServers(super, spec),
		httpStat:    httpstat.New(),
		memoryCache: memoryCache,
	}
}

func (p *pool) status() *PoolStatus {
	s := &PoolStatus{Stat: p.httpStat.Status()}
	return s
}

func (p *pool) handle(ctx context.HTTPContext, reqBody io.Reader, client *http.Client) string {
	addLazyTag := func(subPrefix, msg string, intMsg int) {
		ctx.Lock()
		ctx.AddLazyTag(p.tagPrefix, subPrefix, msg, intMsg)
		ctx.Unlock()
	}

	setStatusCode := func(code int) {
		ctx.Lock()
		ctx.Response().SetStatusCode(code)
		ctx.Unlock()
	}

	server, err := p.servers.next(ctx)
	if err != nil {
		addLazyTag("serverErr", err.Error(), -1)
		setStatusCode(http.StatusServiceUnavailable)
		return resultInternalError
	}
	addLazyTag("addr", server.URL, -1)

	req, err := p.prepareRequest(ctx, server, reqBody)
	if err != nil {
		msg := stringtool.Cat("prepare request failed: ", err.Error())
		logger.Errorf("BUG: %s", msg)
		addLazyTag("bug", msg, -1)
		setStatusCode(http.StatusInternalServerError)
		return resultInternalError
	}

	resp, span, err := p.doRequest(ctx, req, client)
	if err != nil {
		// NOTE: May add option to cancel the tracing if failed here.
		// ctx.Span().Cancel()

		addLazyTag("doRequestErr", fmt.Sprintf("%v", err), -1)
		addLazyTag("trace", req.detail(), -1)
		if ctx.ClientDisconnected() {
			// NOTE: The HTTPContext will set 499 by itself if client is Disconnected.
			// w.SetStatusCode((499)
			return resultClientError
		}

		setStatusCode(http.StatusServiceUnavailable)
		return resultServerError
	}

	addLazyTag("code", "", resp.StatusCode)

	ctx.Lock()
	defer ctx.Unlock()
	// NOTE: The code below can't use addTag and setStatusCode in case of deadlock.

	respBody := p.statRequestResponse(ctx, req, resp, span)

	if p.writeResponse {
		ctx.Response().SetStatusCode(resp.StatusCode)
		ctx.Response().Header().SetRaw(resp.Header)
		ctx.Response().SetBody(respBody)

		return ""
	}

	go func() {
		// NOTE: Need to be read to completion and closed.
		// Reference: https://golang.org/pkg/net/http/#Response
		// And we do NOT do statistics of duration and respSize
		// for it, because we can't wait for it to finish.
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
	}()

	return ""
}

func (p *pool) prepareRequest(ctx context.HTTPContext, server *Server, reqBody io.Reader) (req *request, err error) {
	return p.newRequest(ctx, server, reqBody)
}

func (p *pool) doRequest(ctx context.HTTPContext, req *request, client *http.Client) (*http.Response, tracing.Span, error) {
	req.start()

	spanName := p.spec.SpanName
	if spanName == "" {
		spanName = req.server.URL
	}

	span := ctx.Span().NewChildWithStart(spanName, req.startTime())
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.std.Header))

	resp, err := fnSendRequest(req.std, client)
	if err != nil {
		return nil, nil, err
	}
	return resp, span, nil
}

var httpstatMetricPool = sync.Pool{
	New: func() interface{} {
		return &httpstat.Metric{}
	},
}

func (p *pool) statRequestResponse(ctx context.HTTPContext,
	req *request, resp *http.Response, span tracing.Span) io.Reader {

	var count int

	callbackBody := callbackreader.New(resp.Body)
	callbackBody.OnAfter(func(num int, p []byte, n int, err error) ([]byte, int, error) {
		count += n
		if err == io.EOF {
			req.finish()
			span.Finish()
		}

		return p, n, err
	})

	ctx.OnFinish(func() {
		if !p.writeResponse {
			req.finish()
			span.Finish()
		}
		duration := req.total()
		ctx.AddLazyTag(p.tagPrefix, "#duration: ", duration.String(), -1)
		// use recycled object
		metric := httpstatMetricPool.Get().(*httpstat.Metric)
		metric.StatusCode = resp.StatusCode
		metric.Duration = duration
		metric.ReqSize = ctx.Request().Size()
		metric.RespSize = uint64(responseMetaSize(resp) + count)

		if !p.writeResponse {
			metric.RespSize = 0
		}
		p.httpStat.Stat(metric)
		// recycle object
		httpstatMetricPool.Put(metric)
	})

	return callbackBody
}

func responseMetaSize(resp *http.Response) int {
	text := http.StatusText(resp.StatusCode)
	if text == "" {
		text = "status code " + strconv.Itoa(resp.StatusCode)
	}

	// meta length is the length of:
	// resp.Proto + " "
	// + strconv.Itoa(resp.StatusCode) + " "
	// + text + "\r\n",
	// + resp.Header().Dump() + "\r\n\r\n"
	//
	// but to improve performance, we won't build this string

	size := len(resp.Proto) + 1
	if resp.StatusCode >= 100 && resp.StatusCode < 1000 {
		size += 3 + 1
	} else {
		size += len(strconv.Itoa(resp.StatusCode)) + 1
	}
	size += len(text) + 2
	size += httpheader.New(resp.Header).Length() + 4

	return size
}

func (p *pool) close() {
	p.servers.close()
}

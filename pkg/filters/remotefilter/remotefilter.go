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

package remotefilter

import (
	"bytes"
	stdcontext "context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of RemoteFilter.
	Kind = "RemoteFilter"

	resultFailed          = "failed"
	resultResponseAlready = "responseAlready"

	// 64KB
	maxBodyBytes = 64 * 1024
	// 192KB
	maxContextBytes = 3 * maxBodyBytes
)

var results = []string{resultFailed, resultResponseAlready}

func init() {
	pipeline.Register(&RemoteFilter{})
}

// All RemoteFilter instances use one globalClient in order to reuse
// some resounces such as keepalive connections.
var globalClient = &http.Client{
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
		// NOTE: The large number of Idle Connections can
		// reduce overhead of building connections.
		MaxIdleConns:          10240,
		MaxIdleConnsPerHost:   512,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// Kind returns the kind of RemoteFilter.
func (rf *RemoteFilter) Kind() string {
	return Kind
}

// DefaultSpec returns default spec.
func (rf *RemoteFilter) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of RemoteFilter.
func (rf *RemoteFilter) Description() string {
	return "RemoteFilter invokes remote apis."
}

// Results returns the results of RemoteFilter.
func (rf *RemoteFilter) Results() []string {
	return results
}

type (
	// RemoteFilter is the filter making remote service acting like internal filter.
	RemoteFilter struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
	}

	// Spec describes RemoteFilter.
	Spec struct {
		URL     string `yaml:"url" jsonschema:"required,format=uri"`
		Timeout string `yaml:"timeout" jsonschema:"omitempty,format=duration"`

		timeout time.Duration
	}

	contextEntity struct {
		Request  *requestEntity  `json:"request"`
		Response *responseEntity `json:"response"`
	}

	requestEntity struct {
		RealIP string `json:"realIP"`

		Method string `json:"method"`

		Scheme   string `json:"scheme"`
		Host     string `json:"host"`
		Path     string `json:"path"`
		Query    string `json:"query"`
		Fragment string `json:"fragment"`

		Proto string `json:"proto"`

		Header http.Header `json:"header"`

		Body []byte `json:"body"`
	}

	responseEntity struct {
		StatusCode int         `json:"statusCode"`
		Header     http.Header `json:"header"`
		Body       []byte      `json:"body"`
	}
)

// Init initializes RemoteFilter.
func (rf *RemoteFilter) Init(filterSpec *pipeline.FilterSpec) {
	rf.filterSpec, rf.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	rf.reload()
}

// Inherit inherits previous generation of RemoteFilter.
func (rf *RemoteFilter) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	rf.Init(filterSpec)
}

func (rf *RemoteFilter) reload() {
	var err error
	if rf.spec.Timeout != "" {
		rf.spec.timeout, err = time.ParseDuration(rf.spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse duration %s failed: %v", rf.spec.Timeout, err)
		}
	}
}

func (rf *RemoteFilter) limitRead(reader io.Reader, n int64) []byte {
	if reader == nil {
		return nil
	}

	buff := bytes.NewBuffer(nil)
	written, err := io.CopyN(buff, reader, n+1)
	if err == nil && written == n+1 {
		panic(fmt.Errorf("larger than %dB", n))
	}

	if err != nil && err != io.EOF {
		panic(err)
	}

	return buff.Bytes()
}

// Handle handles HTTPContext by calling remote service.
func (rf *RemoteFilter) Handle(ctx context.HTTPContext) (result string) {
	result = rf.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (rf *RemoteFilter) handle(ctx context.HTTPContext) (result string) {
	r, w := ctx.Request(), ctx.Response()

	var errPrefix string
	defer func() {
		if err := recover(); err != nil {
			w.SetStatusCode(http.StatusServiceUnavailable)
			// NOTE: We don't use stringtool.Cat because err needs
			// the internal reflection of fmt.Sprintf.
			ctx.AddTag(fmt.Sprintf("remoteFilterErr: %s: %v", errPrefix, err))
			result = resultFailed
		}
	}()

	errPrefix = "read request body"
	reqBody := rf.limitRead(r.Body(), maxBodyBytes)

	errPrefix = "read response body"
	respBody := rf.limitRead(w.Body(), maxBodyBytes)

	errPrefix = "marshal context"
	ctxBuff := rf.marshalHTTPContext(ctx, reqBody, respBody)

	var (
		req *http.Request
		err error
	)

	if rf.spec.timeout > 0 {
		timeoutCtx, cancelFunc := stdcontext.WithTimeout(stdcontext.Background(), rf.spec.timeout)
		defer cancelFunc()
		req, err = http.NewRequestWithContext(timeoutCtx, http.MethodPost, rf.spec.URL, bytes.NewReader(ctxBuff))
	} else {
		req, err = http.NewRequest(http.MethodPost, rf.spec.URL, bytes.NewReader(ctxBuff))
	}

	if err != nil {
		logger.Errorf("BUG: new request failed: %v", err)
		w.SetStatusCode(http.StatusInternalServerError)
		ctx.AddTag(stringtool.Cat("remoteFilterBug: ", err.Error()))
		return resultFailed
	}

	errPrefix = "do request"
	resp, err := globalClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		panic(fmt.Errorf("not 2xx status code: %d", resp.StatusCode))
	}

	errPrefix = "read remote body"
	ctxBuff = rf.limitRead(resp.Body, maxContextBytes)

	errPrefix = "unmarshal context"
	rf.unmarshalHTTPContext(ctxBuff, ctx)

	if resp.StatusCode == 205 {
		return resultResponseAlready
	}

	return ""
}

// Status returns status.
func (rf *RemoteFilter) Status() interface{} { return nil }

// Close closes RemoteFilter.
func (rf *RemoteFilter) Close() {}

func (rf *RemoteFilter) marshalHTTPContext(ctx context.HTTPContext, reqBody, respBody []byte) []byte {
	r, w := ctx.Request(), ctx.Response()
	ctxEntity := contextEntity{
		Request: &requestEntity{
			RealIP:   r.RealIP(),
			Method:   r.Method(),
			Scheme:   r.Scheme(),
			Host:     r.Host(),
			Path:     r.Path(),
			Query:    r.Query(),
			Fragment: r.Fragment(),
			Proto:    r.Proto(),
			Header:   r.Header().Std(),
			Body:     reqBody,
		},
		Response: &responseEntity{
			StatusCode: w.StatusCode(),
			Header:     w.Header().Std(),
			Body:       respBody,
		},
	}

	buff, err := json.Marshal(ctxEntity)
	if err != nil {
		panic(err)
	}

	return buff
}

func (rf *RemoteFilter) unmarshalHTTPContext(buff []byte, ctx context.HTTPContext) {
	ctxEntity := &contextEntity{}

	err := json.Unmarshal(buff, ctxEntity)
	if err != nil {
		panic(err)
	}

	r, w := ctx.Request(), ctx.Response()
	re, we := ctxEntity.Request, ctxEntity.Response

	r.SetMethod(re.Method)
	r.SetPath(re.Path)
	r.SetQuery(re.Query)
	r.Header().Reset(re.Header)
	r.SetBody(bytes.NewReader(re.Body), true)

	if we == nil {
		return
	}

	if we.StatusCode < 200 || we.StatusCode >= 600 {
		panic(fmt.Errorf("invalid status code: %d", we.StatusCode))
	}

	w.SetStatusCode(we.StatusCode)
	w.Header().Reset(we.Header)
	w.SetBody(bytes.NewReader(we.Body))
}

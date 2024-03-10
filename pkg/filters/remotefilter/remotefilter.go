/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package remotefilter implements the RemoteFilter filter to invokes remote apis.
package remotefilter

import (
	"bytes"
	stdcontext "context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
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

var kind = &filters.Kind{
	Name:        Kind,
	Description: "RemoteFilter invokes remote apis.",
	Results:     []string{resultFailed, resultResponseAlready},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &RemoteFilter{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
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

// Name returns the name of the RemoteFilter filter instance.
func (rf *RemoteFilter) Name() string {
	return rf.spec.Name()
}

// Kind returns the kind of RemoteFilter.
func (rf *RemoteFilter) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the RemoteFilter
func (rf *RemoteFilter) Spec() filters.Spec {
	return rf.spec
}

type (
	// RemoteFilter is the filter making remote service acting like internal filter.
	RemoteFilter struct {
		spec *Spec
	}

	// Spec describes RemoteFilter.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		URL     string `json:"url" jsonschema:"required,format=uri"`
		Timeout string `json:"timeout,omitempty" jsonschema:"format=duration"`

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
func (rf *RemoteFilter) Init() {
	rf.reload()
}

// Inherit inherits previous generation of RemoteFilter.
func (rf *RemoteFilter) Inherit(previousGeneration filters.Filter) {
	rf.Init()
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

// Handle handles Context by calling remote service.
func (rf *RemoteFilter) Handle(ctx *context.Context) (result string) {
	r := ctx.GetInputRequest().(*httpprot.Request)

	var w *httpprot.Response
	response := ctx.GetOutputResponse()
	if response == nil {
		w, _ = httpprot.NewResponse(nil)
		ctx.SetOutputResponse(w)
	} else {
		w = response.(*httpprot.Response)
	}

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
	reqBody := rf.limitRead(r.GetPayload(), maxBodyBytes)

	errPrefix = "read response body"
	respBody := rf.limitRead(w.GetPayload(), maxBodyBytes)

	errPrefix = "marshal context"
	ctxBuff := rf.marshalHTTPContext(r, w, reqBody, respBody)

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
	rf.unmarshalHTTPContext(r, w, ctxBuff)

	if resp.StatusCode == 205 {
		return resultResponseAlready
	}

	return ""
}

// Status returns status.
func (rf *RemoteFilter) Status() interface{} { return nil }

// Close closes RemoteFilter.
func (rf *RemoteFilter) Close() {}

func (rf *RemoteFilter) marshalHTTPContext(r *httpprot.Request, w *httpprot.Response, reqBody, respBody []byte) []byte {
	ctxEntity := contextEntity{
		Request: &requestEntity{
			RealIP:   r.RealIP(),
			Method:   r.Method(),
			Scheme:   r.Scheme(),
			Host:     r.Host(),
			Path:     r.Path(),
			Query:    r.URL().RawQuery,
			Fragment: r.URL().Fragment,
			Proto:    r.Proto(),
			Header:   r.Std().Header,
			Body:     reqBody,
		},
		Response: &responseEntity{
			StatusCode: w.StatusCode(),
			Header:     w.Std().Header,
			Body:       respBody,
		},
	}

	buff, err := codectool.MarshalJSON(ctxEntity)
	if err != nil {
		panic(err)
	}

	return buff
}

func (rf *RemoteFilter) unmarshalHTTPContext(r *httpprot.Request, w *httpprot.Response, buff []byte) {
	ctxEntity := &contextEntity{}

	err := codectool.Unmarshal(buff, ctxEntity)
	if err != nil {
		panic(err)
	}

	re, we := ctxEntity.Request, ctxEntity.Response

	r.SetMethod(re.Method)
	r.SetPath(re.Path)
	r.URL().RawQuery = re.Query
	r.Header().Walk(func(key string, values interface{}) bool {
		r.Header().Del(key)
		return true
	})
	for k, vs := range re.Header {
		for _, v := range vs {
			r.Header().Add(k, v)
		}
	}

	if r.IsStream() {
		if c, ok := r.GetPayload().(io.Closer); ok {
			c.Close()
		}
	}
	r.SetPayload(re.Body)

	if we == nil {
		return
	}

	if we.StatusCode < 200 || we.StatusCode >= 600 {
		panic(fmt.Errorf("invalid status code: %d", we.StatusCode))
	}

	w.SetStatusCode(we.StatusCode)
	for k, vs := range we.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	if w.IsStream() {
		if c, ok := w.GetPayload().(io.Closer); ok {
			c.Close()
		}
	}
	w.SetPayload(we.Body)
}

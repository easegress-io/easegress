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
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	httpstat "github.com/tcnksm/go-httpstat"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/stretchr/testify/assert"
)

func TestRequest(t *testing.T) {
	assert := assert.New(t)
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedPath = func() string {
		return "/abc"
	}
	ctx.MockedRequest.MockedQuery = func() string {
		return "state=1"
	}
	ctx.MockedRequest.MockedMethod = func() string {
		return http.MethodGet
	}
	ctx.MockedRequest.MockedHost = func() string {
		return "megaease.com"
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(http.Header{})
	}

	server := Server{
		URL: "http://192.168.1.2",
	}

	p := pool{}
	sr := strings.NewReader("this is the raw body")
	req, _ := p.newRequest(ctx, &server, sr, requestPool, httpStatResultPool)
	defer requestPool.Put(req) // recycle request

	req.start()
	tm := req.startTime()

	time.Sleep(time.Millisecond)
	req.start()
	assert.Equal(tm, req.startTime())

	req.total()

	time.Sleep(time.Millisecond)
	req.finish()
	tm = req.endTime()
	time.Sleep(time.Millisecond)
	req.finish()

	assert.Equal(tm, req.endTime())

	req.total()
	assert.NotEqual("", req.detail())

	ctx.MockedRequest.MockedMethod = func() string {
		return "ééé" // not tokenable, should fail
	}
	req, err := p.newRequest(ctx, &server, nil, requestPool, httpStatResultPool)
	assert.Nil(req)
	assert.NotNil(err)
}

func TestResultState(t *testing.T) {
	rs := &resultState{buff: &bytes.Buffer{}}
	if n, b := rs.Width(); n != 0 || b {
		t.Error("implementation changed, this case should be updated")
	}
	if n, b := rs.Precision(); n != 0 || b {
		t.Error("implementation changed, this case should be updated")
	}
	if rs.Flag(1) {
		t.Error("implementation changed, this case should be updated")
	}
}

func TestRequestStatus(t *testing.T) {
	statResult := httpStatResultPool.Get().(*httpstat.Result)
	req := requestPool.Get().(*request)
	req.createTime = fasttime.Now()
	req._startTime = time.Time{}
	req._endTime = time.Time{}
	req.statResult = statResult

	if !req.createTime.Equal(req.startTime()) {
		t.Error("starttime should be createtime before start()")
	}
	time.Sleep(time.Millisecond)
	req.start()

	if req.createTime.Equal(req.startTime()) {
		t.Error("starttime should not be createtime after start()")
	}

	if req.endTime().Equal(req._endTime) {
		t.Error("endtime should be now before finish()")
	}
	time.Sleep(time.Millisecond)
	req.finish()

	if !req.endTime().Equal(req._endTime) {
		t.Error("endtime should be _endtime after finish()")
	}

	httpStatResultPool.Put(req.statResult)
	requestPool.Put(req)
}

func TestAddB3PropagationHeaders(t *testing.T) {
	assert := assert.New(t)
	ctx := &contexttest.MockedHTTPContext{}
	traceID := "463ac35c9f6413ad48485a3953bb6124"
	spanID := "a2fb4a1d1a96d312"
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(http.Header{
			"X-B3-TraceId":      []string{traceID},
			"X-B3-ParentSpanId": []string{"parentspanid"},
			"X-B3-SpanId":       []string{spanID},
			"X-B3-Sampled":      []string{"sampled"},
		})
	}

	server := Server{
		URL: "http://192.168.1.2",
	}

	p := pool{}
	sr := strings.NewReader("this is the raw body")
	req, _ := p.newRequest(ctx, &server, sr, requestPool, httpStatResultPool)
	defer requestPool.Put(req) // recycle request

	header := req.std.Header
	assert.Equal(1, len(header["X-B3-TraceId"]))
	assert.Equal(traceID, header["X-B3-TraceId"][0])
	assert.Equal(1, len(header["X-B3-Sampled"]))
	assert.Equal("sampled", header["X-B3-Sampled"][0])
}

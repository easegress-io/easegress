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

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

func TestRequest(t *testing.T) {
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
	req, _ := p.newRequest(ctx, &server, sr)

	req.start()
	tm := req.startTime()

	time.Sleep(time.Millisecond)
	req.start()
	if tm != req.startTime() {
		t.Error("start time should not change")
	}

	req.total()

	time.Sleep(time.Millisecond)
	req.finish()
	tm = req.endTime()
	time.Sleep(time.Millisecond)
	req.finish()

	if tm != req.endTime() {
		t.Error("end time should not change")
	}

	req.total()
	if req.detail() == "" {
		t.Error("detail should not be empty")
	}
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

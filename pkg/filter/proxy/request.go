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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	httpstat "github.com/tcnksm/go-httpstat"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

type (
	request struct {
		server     *Server
		std        *http.Request
		statResult *httpstat.Result
		createTime time.Time
		_startTime time.Time
		_endTime   time.Time
	}

	resultState struct {
		buff *bytes.Buffer
	}
)

func (p *pool) newRequest(
	ctx context.HTTPContext,
	server *Server,
	reqBody io.Reader,
	requestPool *sync.Pool,
	httpStatResultPool *sync.Pool) (*request, error) {
	statResult := httpStatResultPool.Get().(*httpstat.Result)
	req := requestPool.Get().(*request)
	req.createTime = fasttime.Now()
	req.server = server
	req.statResult = statResult
	req._startTime = time.Time{}
	req._endTime = time.Time{}

	r := ctx.Request()

	url := server.URL + r.Path()
	if r.Query() != "" {
		url += "?" + r.Query()
	}

	newCtx := httpstat.WithHTTPStat(ctx, req.statResult)
	stdr, err := http.NewRequestWithContext(newCtx, r.Method(), url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("BUG: new request failed: %v", err)
	}

	stdr.Header = r.Header().Std()
	// only set host when server address is not host name OR server is explicitly told to keep the host of the request.
	if !server.addrIsHostName || server.KeepHost {
		stdr.Host = r.Host()
	}

	// Based on Golang std lib comment:
	// https://github.com/golang/go/blob/master/src/net/http/request.go#L857
	// If body is of type *bytes.Buffer, *bytes.Reader, or
	// *strings.Reader, the returned request's ContentLength is set to its
	// exact value (instead of -1), GetBody is populated (so 307 and 308
	// redirects can replay the body), and Body is set to NoBody if the
	// ContentLength is 0.
	//
	// Here we set ContextLength when "Context-Length" header is not empty.
	if val := stdr.Header.Get(httpheader.KeyContentLength); val != "" {
		l, err := strconv.Atoi(val)
		if err == nil {
			stdr.ContentLength = int64(l)
		}
	}

	req.std = stdr

	return req, nil
}

func (r *request) start() {
	if !time.Time.IsZero(r._startTime) {
		logger.Errorf("BUG: started already")
		return
	}

	r._startTime = fasttime.Now()
}

func (r *request) startTime() time.Time {
	if time.Time.IsZero(r._startTime) {
		return r.createTime
	}

	return r._startTime
}

func (r *request) endTime() time.Time {
	if time.Time.IsZero(r._endTime) {
		return fasttime.Now()
	}

	return r._endTime
}

func (r *request) finish() {
	if !time.Time.IsZero(r._endTime) {
		logger.Errorf("BUG: finished already")
		return
	}

	now := fasttime.Now()
	r.statResult.End(now)
	r._endTime = now
}

func (r *request) total() time.Duration {
	if time.Time.IsZero(r._endTime) {
		logger.Errorf("BUG: call total before finish")
		return r.statResult.Total(fasttime.Now())
	}

	return r.statResult.Total(r._endTime)
}

func (r *request) detail() string {
	rs := &resultState{buff: bytes.NewBuffer(nil)}
	r.statResult.Format(rs, 's')
	return rs.buff.String()
}

func (rs *resultState) Write(p []byte) (int, error) { return rs.buff.Write(p) }
func (rs *resultState) Width() (int, bool)          { return 0, false }
func (rs *resultState) Precision() (int, bool)      { return 0, false }
func (rs *resultState) Flag(c int) bool             { return false }

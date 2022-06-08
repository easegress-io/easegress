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
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	httpstat "github.com/tcnksm/go-httpstat"
	"golang.org/x/net/http/httpguts"

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

// removeConnectionHeaders removes hop-by-hop headers listed in the "Connection" header of h.
// See RFC 7230, section 6.1
func removeConnectionHeaders(h http.Header) {
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = textproto.TrimString(sf); sf != "" {
				h.Del(sf)
			}
		}
	}
}

// https://github.com/golang/go/blob/95b68e1e02fa713719f02f6c59fb1532bd05e824/src/net/http/httputil/reverseproxy.go#L171-L186
// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

// info: https://github.com/golang/go/blob/95b68e1e02fa713719f02f6c59fb1532bd05e824/src/net/http/httputil/reverseproxy.go#L214
func processHopHeaders(inReq, outReq *http.Request) {
	removeConnectionHeaders(outReq.Header)

	for _, h := range hopHeaders {
		outReq.Header.Del(h)
	}

	if httpguts.HeaderValuesContainsToken(inReq.Header["Te"], "trailers") {
		outReq.Header.Set("Te", "trailers")
	}
}

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

	stdr.Header = r.Header().Std().Clone()
	// only set host when server address is not host name OR server is explicitly told to keep the host of the request.
	if !server.addrIsHostName || server.KeepHost {
		stdr.Host = r.Host()
	}

	// Based on comments in proxy.Handle, we update stdr.ContentLength here.
	if val := stdr.Header.Get(httpheader.KeyContentLength); val != "" {
		l, err := strconv.Atoi(val)
		if err == nil {
			stdr.ContentLength = int64(l)
		}
	}
	processHopHeaders(r.Std(), stdr)

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

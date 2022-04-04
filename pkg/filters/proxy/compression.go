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
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/klauspost/compress/gzip"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

// TODO: Expose more options: compression level, mime types.

var bodyFlushSize = 8 * int64(os.Getpagesize())

type (
	gzipBody struct {
		body     io.Reader
		buff     *bytes.Buffer
		gw       *gzip.Writer
		complete bool
	}

	// compression is filter compression.
	compression struct {
		spec *CompressionSpec
	}

	// CompressionSpec describes the compression.
	CompressionSpec struct {
		MinLength uint32 `yaml:"minLength"`
	}
)

func newCompression(spec *CompressionSpec) *compression {
	return &compression{
		spec: spec,
	}
}

func (c *compression) compress(ctx context.Context) {
	req := ctx.Request().(*httpprot.Request)
	resp := ctx.Response().(*httpprot.Response)

	if !c.acceptGzip(req) {
		return
	}

	if c.alreadyGziped(resp) {
		return
	}

	cl := c.parseContentLength(resp)
	if cl != -1 && cl < int(c.spec.MinLength) {
		return
	}

	resp.HTTPHeader().Del(httpprot.KeyContentLength)
	resp.HTTPHeader().Set(httpprot.KeyContentEncoding, "gzip")
	resp.HTTPHeader().Add(httpprot.KeyVary, httpprot.KeyContentEncoding)

	ctx.AddTag("gzip")

	// TODO: which response to use??
	resp.SetPayload(newGzipBody(w.Body()))
}

func (c *compression) alreadyGziped(resp *httpprot.Response) bool {
	for _, ce := range resp.HTTPHeader().Values(httpprot.KeyContentEncoding) {
		if strings.Contains(ce, "gzip") {
			return true
		}
	}

	return false
}

func (c *compression) acceptGzip(req *httpprot.Request) bool {
	acceptEncodings := req.HTTPHeader().Values(httpprot.KeyAcceptEncoding)

	// NOTE: Easegress does not support parsing qvalue for performance.
	// Reference: https://tools.ietf.org/html/rfc2616#section-14.3
	if len(acceptEncodings) > 0 {
		for _, ae := range acceptEncodings {
			if strings.Contains(ae, "*/*") ||
				strings.Contains(ae, "gzip") {
				return true
			}
		}
		return false
	}

	return true
}

func (c *compression) parseContentLength(resp *httpprot.Response) int {
	contentLength := resp.HTTPHeader().Get(httpprot.KeyContentLength)
	if contentLength == "" {
		return -1
	}

	cl, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return -1
	}

	return int(cl)
}

func newGzipBody(body io.Reader) *gzipBody {
	buff := bytes.NewBuffer(nil)
	return &gzipBody{
		body: body,
		buff: buff,
		gw:   gzip.NewWriter(buff),
	}
}

// body -> gw -> p
func (gb *gzipBody) Read(p []byte) (int, error) {
	if gb.complete {
		return 0, io.EOF
	}

	if len(gb.buff.Bytes()) < len(p) {
		gb.pull()
	}

	n, err := gb.buff.Read(p)
	if err == io.EOF && !gb.complete {
		err = nil
	}

	return n, err
}

func (gb *gzipBody) pull() {
	_, err := io.CopyN(gb.gw, gb.body, bodyFlushSize)
	switch err {
	case nil:
		// Nothing to do.
	case io.EOF:
		err := gb.gw.Close()
		if err != nil {
			logger.Errorf("BUG: close gzip failed: %v", err)
		}
		gb.complete = true
	default:
		gb.complete = true
		logger.Errorf("BUG: copy body to gzip failed: %v", err)
	}
}

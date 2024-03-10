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

package httpprot

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/util/readers"
)

// Response wraps http.Response.
//
// The payload of the response can be replaced with a new one, but it will
// never replace the body of the original http.Response.
//
// Code should always use payload functions of this response to read the
// body of the original response, and never use the Body of the original
// response directly.
type Response struct {
	// TODO: we only need StatusCode, Header and Body, that's can avoid
	// using the big http.Response object.
	*http.Response
	stream  *readers.ByteCountReader
	payload []byte
}

// ErrResponseEntityTooLarge means the request entity is too large.
var ErrResponseEntityTooLarge = fmt.Errorf("response entity too large, you may need to increase 'serverMaxBodySize' or set it to -1")

var _ protocols.Response = (*Response)(nil)

// NewResponse creates a new response from a standard response. If stdr is not
// nil, FetchPayload must be called before any read of the response body.
func NewResponse(stdr *http.Response) (*Response, error) {
	if stdr == nil {
		stdr := &http.Response{
			Body:          http.NoBody,
			StatusCode:    http.StatusOK,
			Header:        http.Header{},
			ContentLength: -1,
		}
		return &Response{Response: stdr}, nil
	}

	return &Response{Response: stdr}, nil
}

// IsStream returns whether the payload of the response is a stream.
func (r *Response) IsStream() bool {
	return r.stream != nil
}

// Trailer returns the trailer of the response in type protocols.Trailer.
func (r *Response) Trailer() protocols.Trailer {
	return newHeader(r.Std().Trailer)
}

// FetchPayload reads the body of the underlying http.Response and initializes
// the payload.
//
// if maxPayloadSize is a negative number, the payload is treated as a stream.
// if maxPayloadSize is zero, DefaultMaxPayloadSize is used.
func (r *Response) FetchPayload(maxPayloadSize int64) error {
	if maxPayloadSize == 0 {
		maxPayloadSize = DefaultMaxPayloadSize
	}

	if maxPayloadSize < 0 {
		r.SetPayload(r.Response.Body)
		return nil
	}

	stdr := r.Response

	if stdr.ContentLength > maxPayloadSize {
		return ErrResponseEntityTooLarge
	}

	if stdr.ContentLength > 0 {
		payload := make([]byte, stdr.ContentLength)
		n, err := io.ReadFull(stdr.Body, payload)
		payload = payload[:n]
		r.SetPayload(payload)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	if stdr.ContentLength == 0 {
		r.SetPayload(nil)
		return nil
	}

	payload, err := io.ReadAll(io.LimitReader(stdr.Body, maxPayloadSize))
	r.SetPayload(payload)
	if err != nil {
		return err
	}

	if len(payload) < int(maxPayloadSize) {
		return nil
	}

	// try read extra bytes to check if the payload is too large.
	n, err := io.Copy(io.Discard, stdr.Body)
	if n > 0 {
		return ErrResponseEntityTooLarge
	}

	return err
}

// SetPayload set the payload of the response to payload. The payload
// could be a string, a byte slice, or an io.Reader, and if it is an
// io.Reader, it will be treated as a stream, if this is not desired,
// please read the data to a byte slice, and set the byte slice as
// the payload.
func (r *Response) SetPayload(payload interface{}) {
	r.stream = nil
	r.payload = nil

	if payload == nil {
		return
	}

	switch p := payload.(type) {
	case []byte:
		r.payload = p
	case string:
		r.payload = []byte(p)
	case io.Reader:
		if bcr, ok := p.(*readers.ByteCountReader); ok {
			r.stream = bcr
		} else {
			r.stream = readers.NewByteCountReader(p)
		}
	default:
		panic("unknown payload type")
	}
}

// GetPayload returns a payload reader. For non-stream payload, the
// returned reader is always a new one, which contains the full data.
// For stream payload, the function always returns the same reader.
func (r *Response) GetPayload() io.Reader {
	if r.stream != nil {
		return r.stream
	}
	if len(r.payload) == 0 {
		return http.NoBody
	}
	return bytes.NewReader(r.payload)
}

// RawPayload returns the payload in []byte, the caller should not
// modify its content. The function panic if the payload is a stream.
func (r *Response) RawPayload() []byte {
	if r.stream == nil {
		return r.payload
	}
	panic("the payload is a large one")
}

// PayloadSize returns the size of the payload. If the payload is a
// stream, it returns the bytes count that have been currently read
// out.
func (r *Response) PayloadSize() int64 {
	if r.stream == nil {
		return int64(len(r.payload))
	}
	return int64(r.stream.BytesRead())
}

// ToBuilderResponse wraps the response and returns the wrapper, the
// return value can be used in the template of the Builder filters.
func (r *Response) ToBuilderResponse(name string) interface{} {
	var rawBody []byte

	if r.IsStream() {
		rawBody = []byte(fmt.Sprintf("the body of response %q is a stream", name))
	} else {
		rawBody = r.RawPayload()
	}

	return &builderResponse{
		Response: r.Std(),
		rawBody:  rawBody,
	}
}

// Std returns the underlying http.Response.
func (r *Response) Std() *http.Response {
	return r.Response
}

// MetaSize returns the meta data size of the response.
func (r *Response) MetaSize() int64 {
	stdr := r.Std()
	text := http.StatusText(stdr.StatusCode)
	if text == "" {
		text = "status code " + strconv.Itoa(stdr.StatusCode)
	}

	// meta length is the length of:
	// resp.Proto + " "
	// + strconv.Itoa(resp.StatusCode) + " "
	// + text + "\r\n",
	// + resp.Header().Dump() + "\r\n\r\n"
	//
	// but to improve performance, we won't build this string

	size := len(stdr.Proto) + 1
	if stdr.StatusCode >= 100 && stdr.StatusCode < 1000 {
		size += 3 + 1
	} else {
		size += len(strconv.Itoa(stdr.StatusCode)) + 1
	}
	size += len(text) + 2

	lines := 0
	for key, values := range stdr.Header {
		for _, value := range values {
			lines++
			size += len(key) + len(value)
		}
	}

	size += lines * 2 // ": "
	if lines > 1 {
		size += (lines - 1) * 2 // "\r\n"
	}

	return int64(size)
}

// StatusCode returns the status code of the response.
func (r *Response) StatusCode() int {
	return r.Std().StatusCode
}

// SetStatusCode sets the status code of the response.
func (r *Response) SetStatusCode(code int) {
	r.Std().StatusCode = code
}

// SetCookie adds a Set-Cookie header to the response's headers.
func (r *Response) SetCookie(cookie *http.Cookie) {
	if v := cookie.String(); v != "" {
		r.HTTPHeader().Add("Set-Cookie", v)
	}
}

// HTTPHeader returns the header of the response in type http.Header.
func (r *Response) HTTPHeader() http.Header {
	return r.Std().Header
}

// Header returns the header of the response in type protocols.Header.
func (r *Response) Header() protocols.Header {
	return newHeader(r.HTTPHeader())
}

// Close closes the response.
func (r *Response) Close() {
	if r.stream != nil {
		r.stream.Close()
	}
	r.Std().Body.Close()
}

// builderResponse is a wrapper of http.Response which can be used in the
// template of the Builder filters.
type builderResponse struct {
	*http.Response
	rawBody    []byte
	parsedBody interface{}
}

// RawBody returns the body as raw bytes.
func (r *builderResponse) RawBody() []byte {
	return r.rawBody
}

// Body returns the body as a string.
func (r *builderResponse) Body() string {
	return string(r.rawBody)
}

// JSONBody parses the body as a JSON object and returns the result.
// The function only parses the body if it is not already parsed.
func (r *builderResponse) JSONBody() (interface{}, error) {
	if r.parsedBody != nil {
		return r.parsedBody, nil
	}

	v, err := parseJSONBody(r.rawBody)
	r.parsedBody = v
	return v, err
}

// YAMLBody parses the body as a YAML object and returns the result.
// The function only parses the body if it is not already parsed.
// The function can only handle simple YAML objects, that is, the keys
// of the YAML object must be strings.
func (r *builderResponse) YAMLBody() (interface{}, error) {
	if r.parsedBody != nil {
		return r.parsedBody, nil
	}

	v, err := parseYAMLBody(r.rawBody)
	r.parsedBody = v
	return v, err
}

// responseInfo stores the information of a response.
type responseInfo struct {
	StatusCode int                 `json:"statusCode,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string              `json:"body,omitempty"`
}

// NewResponseInfo returns a new responseInfo.
func (p *Protocol) NewResponseInfo() interface{} {
	return &responseInfo{}
}

// BuildResponse builds and returns a response according to the given respInfo.
func (p *Protocol) BuildResponse(respInfo interface{}) (protocols.Response, error) {
	ri, ok := respInfo.(*responseInfo)
	if !ok {
		return nil, fmt.Errorf("invalid response info type: %T", respInfo)
	}

	if ri.StatusCode == 0 {
		ri.StatusCode = http.StatusOK
	} else if ri.StatusCode < 200 || ri.StatusCode >= 600 {
		return nil, fmt.Errorf("invalid status code: %d", ri.StatusCode)
	}

	stdResp := &http.Response{Header: http.Header{}, Body: http.NoBody}
	stdResp.StatusCode = ri.StatusCode

	for k, vs := range ri.Headers {
		for _, v := range vs {
			stdResp.Header.Add(k, v)
		}
	}

	// build body
	resp, _ := NewResponse(stdResp)
	resp.SetPayload([]byte(ri.Body))

	return resp, nil
}

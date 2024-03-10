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

// Package protocols defines the common interface of protocols used in Easegress.
package protocols

import (
	"io"
	"strings"
)

var registry = map[string]Protocol{}

// Register registers a new protocol with name.
func Register(name string, p Protocol) {
	registry[strings.ToUpper(name)] = p
}

// Get returns protocol by name.
func Get(name string) Protocol {
	return registry[strings.ToUpper(name)]
}

// Request is the protocol independent interface of a request.
type Request interface {
	// Header returns the header of the request.
	Header() Header

	// RealIP returns the real IP of the request.
	RealIP() string

	// IsStream returns whether the payload is a stream, which cannot be
	// read for more than once.
	IsStream() bool

	// SetPayload set the payload of the request to payload. The payload
	// could be a string, a byte slice, or an io.Reader, and if it is an
	// io.Reader, it will be treated as a stream, if this is not desired,
	// please read the data to a byte slice, and set the byte slice as
	// the payload.
	//
	// If the previous payload is a stream, it is the caller's responsibility
	// to close it, if required (that's, the previous payload is also an
	// io.Closer).
	SetPayload(payload interface{})

	// GetPayload returns a payload reader. For non-stream payload, the
	// returned reader is always a new one, which contains the full data.
	// For stream payload, the function always returns the same reader.
	GetPayload() io.Reader

	// RawPayload returns the payload in []byte, the caller should not
	// modify its content. The function panic if the payload is a stream.
	RawPayload() []byte

	// PayloadSize returns the size of the payload. If the payload is a
	// stream, it returns the bytes count that have been currently read
	// out.
	PayloadSize() int64

	// ToBuilderRequest wraps the request and returns the wrapper, the
	// return value can be used in the template of the Builder filters.
	ToBuilderRequest(name string) interface{}

	// Close closes the request.
	Close()
}

// Response is the protocol independent interface of a response.
type Response interface {
	// Header returns the header of the response.
	Header() Header

	// Trailer returns the trailer of the response
	Trailer() Trailer

	// IsStream returns whether the payload is a stream, which cannot be
	// read for more than once.
	IsStream() bool

	// SetPayload set the payload of the response to payload. The payload
	// could be a string, a byte slice, or an io.Reader, and if it is an
	// io.Reader, it will be treated as a stream, if this is not desired,
	// please read the data to a byte slice, and set the byte slice as
	// the payload.
	//
	// If the previous payload is a stream, it is the caller's responsibility
	// to close it, if required (that's, the previouse payload is also an
	// io.Closer).
	SetPayload(payload interface{})

	// GetPayload returns a payload reader. For non-stream payload, the
	// returned reader is always a new one, which contains the full data.
	// For stream payload, the function always returns the same reader.
	GetPayload() io.Reader

	// RawPayload returns the payload in []byte, the caller should not
	// modify its content. The function panic if the payload is a stream.
	RawPayload() []byte

	// PayloadSize returns the size of the payload. If the payload is a
	// stream, it returns the bytes count that have been currently read
	// out.
	PayloadSize() int64

	// ToBuilderResponse wraps the response and returns the wrapper, the
	// return value can be used in the template of the Builder filters.
	ToBuilderResponse(name string) interface{}

	// Close closes the response.
	Close()
}

// Header is the headers of a request or response.
type Header interface {
	Add(key string, value interface{})
	Set(key string, value interface{})
	Get(key string) interface{}
	Del(key string)
	// Walk walks all header items, and stops if fn returns false.
	Walk(fn func(key string, value interface{}) bool)
	Clone() Header
}

// Trailer is the trailers of a request or response.
type Trailer = Header

// Protocol is the interface of a protocol.
type Protocol interface {
	CreateRequest(req interface{}) (Request, error)
	CreateResponse(resp interface{}) (Response, error)

	NewRequestInfo() interface{}
	BuildRequest(reqInfo interface{}) (Request, error)

	NewResponseInfo() interface{}
	BuildResponse(respInfo interface{}) (Response, error)
}

// Route is the interface of a route.
// Filters could assert the real type according to the protocol.
type Route interface {
	// Protocol returns the canonical name of the protocol in lower case, such as http, grpc.
	Protocol() string
}

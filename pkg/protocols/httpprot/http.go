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

// Package httpprot implements the HTTP protocol.
package httpprot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

// DefaultMaxPayloadSize is the default max allowed payload size.
const DefaultMaxPayloadSize = 4 * 1024 * 1024

func init() {
	protocols.Register("http", &Protocol{})
}

// Header wraps the http header.
type Header struct {
	http.Header
}

func newHeader(h http.Header) *Header {
	return &Header{Header: h}
}

// Add adds the key value pair to the header.
func (h *Header) Add(key string, value interface{}) {
	h.Header.Add(key, value.(string))
}

// Set sets the header entries associated with key to value.
func (h *Header) Set(key string, value interface{}) {
	h.Header.Set(key, value.(string))
}

// Get gets the first value associated with the given key.
func (h *Header) Get(key string) interface{} {
	return h.Header.Get(key)
}

// Del deletes the values associated with key.
func (h *Header) Del(key string) {
	h.Header.Del(key)
}

// Clone returns a copy of h.
func (h *Header) Clone() protocols.Header {
	return &Header{Header: h.Header.Clone()}
}

// Walk calls fn for each key value pair of the header.
func (h *Header) Walk(fn func(key string, value interface{}) bool) {
	for k, v := range h.Header {
		if !fn(k, v) {
			break
		}
	}
}

// Protocol implements protocols.Protocol for HTTP.
type Protocol struct{}

var _ protocols.Protocol = (*Protocol)(nil)

// CreateRequest creates a new request. The input argument should be a
// *http.Request or nil.
//
// The caller need to handle the close of the body of the input request,
// if it need to be closed. Particularly, the body of the input request
// may be replaced, so the caller must save a reference of the original
// body and close it when it is no longer needed.
func (p *Protocol) CreateRequest(req interface{}) (protocols.Request, error) {
	r, _ := req.(*http.Request)
	return NewRequest(r)
}

// CreateResponse creates a new response. The input argument should be a
// *http.Response or nil.
//
// The caller need to handle the close of the body of the input response,
// if it need to be closed. Particularly, the body of the input response
// may be replaced, so the caller must save a reference of the original
// body and close it when it is no longer needed.
func (p *Protocol) CreateResponse(resp interface{}) (protocols.Response, error) {
	r, _ := resp.(*http.Response)
	return NewResponse(r)
}

func parseJSONBody(body []byte) (interface{}, error) {
	var v interface{}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// This function can only handle simple YAML objects, that is, the keys
// of the YAML object must be strings.
func parseYAMLBody(body []byte) (interface{}, error) {
	var v interface{}
	err := codectool.UnmarshalYAML(body, &v)
	if err != nil {
		return nil, err
	}

	var convert func(o interface{}) (interface{}, error)
	convert = func(o interface{}) (interface{}, error) {
		switch x := o.(type) {
		case []interface{}:
			for i, v := range x {
				v, err := convert(v)
				if err != nil {
					return nil, err
				}
				x[i] = v
			}
			return x, nil
		case map[interface{}]interface{}:
			x2 := make(map[string]interface{}, len(x))
			for k, v := range x {
				ks, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected key type %T", k)
				}
				v, err := convert(v)
				if err != nil {
					return nil, err
				}
				x2[ks] = v
			}
			return x2, nil
		}
		return o, nil
	}

	return convert(v)
}

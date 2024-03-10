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

package grpcprot

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/protocols"
	"google.golang.org/grpc/metadata"
)

type (
	// Header wrapper metadata.MD
	Header struct {
		md metadata.MD
	}
	// Trailer wrapper metadata.MD. used for grpc response in server-side
	Trailer = Header
)

// NewHeader returns a new Header
func NewHeader(md metadata.MD) *Header {
	return &Header{
		md: md.Copy(),
	}
}

// NewTrailer returns a new Trailer
func NewTrailer(md metadata.MD) *Trailer {
	return &Trailer{
		md: md.Copy(),
	}
}

// GetMD returns the metadata.MD
func (h *Header) GetMD() metadata.MD {
	return h.md.Copy()
}

// RawAdd adds the key, value pair to the header.
func (h *Header) RawAdd(key string, values ...string) {
	h.md.Append(key, values...)
}

// Add adds the key, value pair to the header.
func (h *Header) Add(key string, value interface{}) {
	switch v := value.(type) {
	case string:
		h.md.Append(key, v)
	case []string:
		h.md.Append(key, v...)
	default:
		panic(fmt.Sprintf("append grpc header value type %T is not string or []string", value))
	}
}

// RawSet sets the header entries associated with key to the given value.
func (h *Header) RawSet(key string, values ...string) {
	h.md.Set(key, values...)
}

// Set sets the header entries associated with key to the given value.
func (h *Header) Set(key string, value interface{}) {
	switch v := value.(type) {
	case string:
		h.md.Set(key, v)
	case []string:
		h.md.Set(key, v...)
	default:
		panic(fmt.Sprintf("set grpc header value type %T is not string or []string", value))
	}
}

// RawGet gets the values associated with the given key.
func (h *Header) RawGet(key string) []string {
	return h.md.Get(key)
}

// Get gets the values associated with the given key.
func (h *Header) Get(key string) interface{} {
	return h.md.Get(key)
}

// Del deletes the values associated with key.
func (h *Header) Del(key string) {
	h.md.Delete(key)
}

// GetFirst returns the first value for a given key.
// k is converted to lowercase before searching in md.
func (h *Header) GetFirst(k string) string {
	v := h.md.Get(k)
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// Merge all the provided metadata will be merged
func (h *Header) Merge(header *Header) {
	if header == nil {
		return
	}
	h.md = metadata.Join(h.md, header.md)
}

// Walk for each key exec fn()
func (h *Header) Walk(fn func(key string, value interface{}) bool) {
	for k, v := range h.md {
		if !fn(k, v) {
			break
		}
	}
}

// Clone returns a copy of the header.
func (h *Header) Clone() protocols.Header {
	return &Header{
		md: h.md.Copy(),
	}
}

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

package grpcprot

import (
	"fmt"
	"github.com/megaease/easegress/pkg/protocols"
	"google.golang.org/grpc/metadata"
	"sync"
)

type Header struct {
	md   metadata.MD
	lock *sync.Mutex
}

func NewHeader(md metadata.MD) *Header {
	return &Header{
		md:   md.Copy(),
		lock: &sync.Mutex{},
	}
}

func (h *Header) GetMD() metadata.MD {
	return h.md.Copy()
}

func (h *Header) RawAdd(key string, values ...string) {
	h.md.Append(key, values...)
}

func (h *Header) Add(key string, value interface{}) {
	switch value.(type) {
	case string:
		h.md.Append(key, value.(string))
	case []string:
		h.md.Append(key, value.([]string)...)
	default:
		panic(fmt.Sprintf("append grpc header value type %T is not string or []string", value))
	}
}

func (h *Header) RawSet(key string, values ...string) {
	h.md.Set(key, values...)
}

func (h *Header) Set(key string, value interface{}) {
	switch value.(type) {
	case string:
		h.md.Set(key, value.(string))
	case []string:
		h.md.Set(key, value.([]string)...)
	default:
		panic(fmt.Sprintf("set grpc header value type %T is not string or []string", value))
	}
}

func (h *Header) RawGet(key string) []string {
	return h.md.Get(key)
}

func (h *Header) Get(key string) interface{} {
	return h.md.Get(key)
}

func (h *Header) Del(key string) {
	h.md.Delete(key)
}

// MergeHeader all the provided metadata will be merged
func (h *Header) MergeHeader(header *Header) {
	if header == nil {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()
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

func (h *Header) Clone() protocols.Header {
	return &Header{
		md:   h.md.Copy(),
		lock: &sync.Mutex{},
	}
}

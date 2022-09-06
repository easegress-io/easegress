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
	"github.com/megaease/easegress/pkg/logger"
	"google.golang.org/grpc/metadata"
	"sync"
)

// Trailer wrapper metadata.MD. used for grpc response in server-side
type Trailer struct {
	lock *sync.Mutex
	md   metadata.MD
}

func NewTrailer(md metadata.MD) *Trailer {
	return &Trailer{
		lock: &sync.Mutex{},
		md:   md.Copy(),
	}
}

func (h *Trailer) GetMD() metadata.MD {
	return h.md
}

func (h *Trailer) Add(key string, value interface{}) {
	switch value.(type) {
	case string:
		h.md.Append(key, value.(string))
	case []string:
		h.md.Append(key, value.([]string)...)
	default:
		logger.Debugf("append grpc header value type %T is not string or []string", value)
	}
}

func (h *Trailer) Set(key string, value interface{}) {
	switch value.(type) {
	case string:
		h.md.Set(key, value.(string))
	case []string:
		h.md.Set(key, value.([]string)...)
	default:
		logger.Debugf("set grpc header value type %T is not string or []string", value)
	}
}

func (h *Trailer) Get(key string) interface{} {
	return h.md.Get(key)
}

func (h *Trailer) Del(key string) {
	h.md.Delete(key)
}

func (h *Trailer) Walk(fn func(key string, value interface{}) bool) {
	for k, v := range h.md {
		if !fn(k, v) {
			break
		}
	}
}

func (h *Trailer) Clone() *Trailer {
	return &Trailer{
		md:   h.md.Copy(),
		lock: &sync.Mutex{},
	}
}

func (h *Trailer) MergeTrailer(trailer Trailer) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.md = metadata.Join(h.md, trailer.md)
}

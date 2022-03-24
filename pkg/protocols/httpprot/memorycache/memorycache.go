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

package memorycache

import (
	"bytes"
	"strings"
	"time"

	cache "github.com/patrickmn/go-cache"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	cleanupIntervalFactor = 2
	cleanupIntervalMin    = 1 * time.Minute
)

type (
	// MemoryCache is an utility MemoryCache.
	MemoryCache struct {
		spec *Spec

		cache *cache.Cache
	}

	// Spec describes the MemoryCache.
	Spec struct {
		Expiration    string   `yaml:"expiration" jsonschema:"required,format=duration"`
		MaxEntryBytes uint32   `yaml:"maxEntryBytes" jsonschema:"required,minimum=1"`
		Codes         []int    `yaml:"codes" jsonschema:"required,minItems=1,uniqueItems=true,format=httpcode-array"`
		Methods       []string `yaml:"methods" jsonschema:"required,minItems=1,uniqueItems=true,format=httpmethod-array"`
	}

	cacheEntry struct {
		statusCode int
		header     protocols.Header
		body       []byte
	}
)

// New creates a MemoryCache.
func New(spec *Spec) *MemoryCache {
	expiration, err := time.ParseDuration(spec.Expiration)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v", spec.Expiration, err)
		expiration = 10 * time.Second
	}

	cleanupInterval := expiration * cleanupIntervalFactor
	if cleanupInterval < cleanupIntervalMin {
		cleanupInterval = cleanupIntervalMin
	}
	cache := cache.New(expiration, cleanupInterval)

	return &MemoryCache{
		spec:  spec,
		cache: cache,
	}
}

func (mc *MemoryCache) key(r *httpprot.Request) string {
	return stringtool.Cat(r.Scheme(), r.Host(), r.Path(), r.Method())
}

// Load tries to load cache for HTTPContext.
func (mc *MemoryCache) Load(r *httpprot.Request, w *httpprot.Response) (loaded bool) {
	// Reference: https://tools.ietf.org/html/rfc7234#section-5.2

	matchMethod := false
	for _, method := range mc.spec.Methods {
		if r.Method() == method {
			matchMethod = true
			break
		}
	}
	if !matchMethod {
		return false
	}

	for _, value := range r.Header().Values(httpprot.KeyCacheControl) {
		if strings.Contains(value, "no-cache") {
			return false
		}
	}

	v, ok := mc.cache.Get(mc.key(r))
	if ok {
		entry := v.(*cacheEntry)
		w.SetStatusCode(entry.statusCode)
		entry.header.Iter(func(key string, values []string) {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		})
		w.Payload().SetReader(bytes.NewReader(entry.body), true)
	}

	return ok
}

// Store tries to store cache for HTTPContext.
func (mc *MemoryCache) Store(r *httpprot.Request, w *httpprot.Response) {

	matchMethod := false
	for _, method := range mc.spec.Methods {
		if r.Method() == method {
			matchMethod = true
			break
		}
	}
	if !matchMethod {
		return
	}

	matchCode := false
	for _, code := range mc.spec.Codes {
		if w.StatusCode() == code {
			matchCode = true
			break
		}
	}
	if !matchCode {
		return
	}

	for _, value := range r.Header().Values(httpprot.KeyCacheControl) {
		if strings.Contains(value, "no-store") ||
			strings.Contains(value, "no-cache") {
			return
		}
	}
	for _, value := range w.Header().Values(httpprot.KeyCacheControl) {
		if strings.Contains(value, "no-store") ||
			strings.Contains(value, "no-cache") ||
			strings.Contains(value, "must-revalidate") {
			return
		}
	}

	key := mc.key(r)
	entry := &cacheEntry{
		statusCode: w.StatusCode(),
		header:     w.Header().Clone(),
	}
	bodyLength := 0
	w.OnFlushBody(func(body []byte, complete bool) []byte {
		bodyLength += len(body)
		if bodyLength > int(mc.spec.MaxEntryBytes) {
			return body
		}

		entry.body = append(entry.body, body...)
		if complete {
			mc.cache.SetDefault(key, entry)
		}

		return body
	})
}

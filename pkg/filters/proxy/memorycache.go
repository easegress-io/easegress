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
	"net/http"
	"strings"
	"time"

	cache "github.com/patrickmn/go-cache"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	minCleanupInterval = time.Minute
	keyCacheControl    = "Cache-Control"
)

type (
	// MemoryCache is an utility MemoryCache.
	MemoryCache struct {
		spec *MemoryCacheSpec

		cache *cache.Cache
	}

	// MemoryCacheSpec describes the MemoryCache.
	MemoryCacheSpec struct {
		Expiration    string   `yaml:"expiration" jsonschema:"required,format=duration"`
		MaxEntryBytes uint32   `yaml:"maxEntryBytes" jsonschema:"required,minimum=1"`
		Codes         []int    `yaml:"codes" jsonschema:"required,minItems=1,uniqueItems=true,format=httpcode-array"`
		Methods       []string `yaml:"methods" jsonschema:"required,minItems=1,uniqueItems=true,format=httpmethod-array"`
	}

	// CacheEntry is an item of the memory cache.
	CacheEntry struct {
		StatusCode int
		Header     http.Header
		Body       []byte
	}
)

// NewMemoryCache creates a MemoryCache.
func NewMemoryCache(spec *MemoryCacheSpec) *MemoryCache {
	expiration, err := time.ParseDuration(spec.Expiration)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v", spec.Expiration, err)
		expiration = 10 * time.Second
	}

	cleanupInterval := expiration * 2
	if cleanupInterval < minCleanupInterval {
		cleanupInterval = minCleanupInterval
	}
	cache := cache.New(expiration, cleanupInterval)

	return &MemoryCache{
		spec:  spec,
		cache: cache,
	}
}

func (mc *MemoryCache) key(req *http.Request) string {
	return stringtool.Cat(httpprot.RequestScheme(req), req.Host, req.URL.Path, req.Method)
}

// Load tries to load cache for HTTPContext.
func (mc *MemoryCache) Load(req *http.Request) *CacheEntry {
	// Reference: https://tools.ietf.org/html/rfc7234#section-5.2

	matched := false

	for _, method := range mc.spec.Methods {
		if req.Method == method {
			matched = true
			break
		}
	}
	if !matched {
		return nil
	}

	for _, value := range req.Header.Values(keyCacheControl) {
		if strings.Contains(value, "no-cache") {
			return nil
		}
	}

	if v, ok := mc.cache.Get(mc.key(req)); ok {
		return v.(*CacheEntry)
	}

	return nil
}

// NeedStore returns whether the response need to be stored.
func (mc *MemoryCache) NeedStore(req *http.Request, resp *http.Response) bool {
	matched := false
	for _, method := range mc.spec.Methods {
		if req.Method == method {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	matched = false
	for _, code := range mc.spec.Codes {
		if resp.StatusCode == code {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	for _, value := range req.Header.Values(keyCacheControl) {
		if strings.Contains(value, "no-store") ||
			strings.Contains(value, "no-cache") {
			return false
		}
	}
	for _, value := range resp.Header.Values(keyCacheControl) {
		if strings.Contains(value, "no-store") ||
			strings.Contains(value, "no-cache") ||
			strings.Contains(value, "must-revalidate") {
			return false
		}
	}

	return true
}

/*
	key := mc.key(r)
	entry := &CacheEntry{
		StatusCode: w.StatusCode(),
		Header:     w.HTTPHeader().Clone(),
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
*/

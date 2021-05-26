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

package httpserver

import (
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/ipfilter"

	lru "github.com/hashicorp/golang-lru"
)

type (
	cache struct {
		arc *lru.ARCCache
	}

	cacheItem struct {
		cached bool

		ipFilterChan     *ipfilter.IPFilters
		notFound         bool
		methodNotAllowed bool
		path             *muxPath
	}
)

func newCache(size uint32) *cache {
	arc, err := lru.NewARC(int(size))
	if err != nil {
		logger.Errorf("BUG: new arc cache failed: %v", err)
		return nil
	}

	return &cache{
		arc: arc,
	}
}

func (c *cache) get(key string) *cacheItem {
	value, ok := c.arc.Get(key)
	if !ok {
		return nil
	}

	return value.(*cacheItem)
}

func (c *cache) put(key string, ci *cacheItem) {
	c.arc.Add(key, ci)
}

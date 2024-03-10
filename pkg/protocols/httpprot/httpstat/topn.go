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

package httpstat

import (
	"sort"
	"sync"

	"github.com/megaease/easegress/v2/pkg/util/urlclusteranalyzer"
)

type (
	// TopN is the statistics tool for HTTP traffic.
	TopN struct {
		m   sync.Map
		n   int
		uca *urlclusteranalyzer.URLClusterAnalyzer
	}

	// Item is the item of status.
	Item struct {
		Path string `json:"path"`
		*Status
	}
)

// NewTopN creates a TopN.
func NewTopN(n int) *TopN {
	return &TopN{
		n:   n,
		m:   sync.Map{},
		uca: urlclusteranalyzer.New(),
	}
}

// Stat stats the ctx.
func (t *TopN) Stat(path string) *HTTPStat {
	pattern := t.uca.GetPattern(path)

	var httpStat *HTTPStat
	if v, loaded := t.m.Load(pattern); loaded {
		httpStat = v.(*HTTPStat)
	} else {
		httpStat = New()
		v, loaded = t.m.LoadOrStore(pattern, httpStat)
		if loaded {
			httpStat = v.(*HTTPStat)
		}
	}

	return httpStat
}

// Status returns TopN Status, and resets all metrics.
func (t *TopN) Status() []*Item {
	status := make([]*Item, 0)
	t.m.Range(func(key, value interface{}) bool {
		status = append(status, &Item{
			Path:   key.(string),
			Status: value.(*HTTPStat).Status(),
		})
		return true
	})

	sort.Slice(status, func(i, j int) bool {
		return status[i].Count > status[j].Count
	})
	n := len(status)
	if n > t.n {
		n = t.n
	}

	return status[0:n]
}

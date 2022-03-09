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

package pathsearch

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	// SearchResult is returned by SearchPath
	SearchResult string

	// MuxRuleInterface is interface for muxRule
	MuxRuleInterface interface {
		Pass(ctx context.HTTPContext) bool
		Match(ctx context.HTTPContext) bool
		Paths() []MuxPathInterface
	}

	// MuxPathInterface is interface for muxPath
	MuxPathInterface interface {
		Pass(ctx context.HTTPContext) bool
		MatchPath(ctx context.HTTPContext) bool
		MatchMethod(ctx context.HTTPContext) bool
		HasHeaders() bool
		MatchHeaders(ctx context.HTTPContext) bool
		GetIPFilterChain() *ipfilter.IPFilters
	}
)

const (
	// NotFound means no path found
	NotFound SearchResult = "not-found"
	// IPNotAllowed means context IP is not allowd
	IPNotAllowed SearchResult = "ip-not-allowed"
	// MethodNotAllowed means context method is not allowd
	MethodNotAllowed SearchResult = "method-not-allowed"
	// Found path
	Found SearchResult = "found"
	// FoundSkipCache means found path but skip caching result
	FoundSkipCache SearchResult = "found-skip-cache"
)

// SearchPath searches path among list of mux rules
func SearchPath(ctx context.HTTPContext, rulesToCheck []MuxRuleInterface) (SearchResult, MuxPathInterface) {
	methodAllowed := false
	pathFound := false
	for _, host := range rulesToCheck {
		if !host.Match(ctx) {
			continue
		}

		if !host.Pass(ctx) {
			return IPNotAllowed, nil
		}

		for _, path := range host.Paths() {
			if !path.MatchPath(ctx) {
				continue
			}

			// at least one path matches
			pathFound = true

			if !path.MatchMethod(ctx) {
				continue
			}
			// at least one path has correct method
			methodAllowed = true

			if !path.Pass(ctx) {
				return IPNotAllowed, path
			}

			if !path.HasHeaders() {
				return Found, path
			}

			if path.MatchHeaders(ctx) {
				return FoundSkipCache, path
			}
		}
	}
	if !methodAllowed && pathFound {
		return MethodNotAllowed, nil
	}
	return NotFound, nil
}

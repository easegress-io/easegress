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

package httpfilter

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

func TestFilterHeader(t *testing.T) {
	r := &http.Request{
		URL: &url.URL{
			Path: "/",
		},
		Header: map[string][]string{
			"X-Easemesh-Servicecanary": {},
			"Content-Type":             {"application/json"},
		},
	}

	req, _ := httpprot.NewRequest(r)

	filter := New(&Spec{
		MatchAllHeaders: true,
		Headers: map[string]*urlrule.StringMatch{
			"Content-Type": {
				Exact: "application/json",
			},
			"X-Easemesh-Servicecanary": {
				Empty: true,
			},
		},
	})

	if !filter.Filter(req) {
		t.Fatalf("filter failed")
	}

	filter.spec.MatchAllHeaders = false
	if !filter.Filter(req) {
		t.Fatalf("filter failed")
	}
}

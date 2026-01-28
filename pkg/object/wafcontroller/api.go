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

package wafcontroller

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/metrics"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// APIGroupName is the name of the WAF API group.
	APIGroupName = "waf"
	// APIPrefix is the prefix for WAF API endpoints.
	APIPrefix = "/waf"
)

type (
	StatsResponse struct {
		Stats []*metrics.MetricStats `json:"stats"`
	}
)

func (waf *WAFController) registerAPIs() {
	group := &api.Group{
		Group: APIGroupName,
		Entries: []*api.Entry{
			{
				Path:    APIPrefix + "/metrics",
				Method:  "GET",
				Handler: waf.metrics,
			},
		},
	}

	api.RegisterAPIs(group)
}

func (waf *WAFController) unregisterAPIs() {
	api.UnregisterAPIs(APIGroupName)
}

func (waf *WAFController) metrics(w http.ResponseWriter, r *http.Request) {
	stats := waf.metricHub.GetStats()
	response := StatsResponse{
		Stats: stats,
	}

	w.Write(codectool.MustMarshalJSON(response))
}

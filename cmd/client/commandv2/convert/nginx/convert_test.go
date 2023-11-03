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

package nginx

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/builder"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestGetRequestAdaptor(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("GET", "http://example.com", strings.NewReader("test"))
	req.Header.Set("Content-Length", "4")
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth("user", "pass")
	req.RemoteAddr = "localhost:8080"
	req.RequestURI = "/apis/v1"
	assert.Nil(err)
	ctx := newContext(t, req)
	info := &ProxyInfo{
		SetHeaders: map[string]string{
			"X-Host":         "$host",
			"X-Hostname":     "$hostname",
			"X-Content":      "$content_length",
			"X-Content-Type": "$content_type",
			"X-Remote-Addr":  "$remote_addr",
			"X-Remote-User":  "$remote_user",
			"X-Request-Body": "$request_body",
			"X-Method":       "$request_method",
			"X-Request-URI":  "$request_uri",
			"X-Scheme":       "$scheme",
		},
	}
	spec := getRequestAdaptor(info)
	ra := filters.GetKind(builder.RequestAdaptorKind).CreateInstance(spec)
	ra.Init()
	ra.Handle(ctx)
	h := ctx.GetInputRequest().(*httpprot.Request).Header()
	expected := map[string]string{
		"X-Host":         "example.com",
		"X-Hostname":     "example.com",
		"X-Content":      "4",
		"X-Content-Type": "text/plain",
		"X-Remote-Addr":  "localhost:8080",
		"X-Remote-User":  "user",
		"X-Request-Body": "test",
		"X-Method":       "GET",
		"X-Request-URI":  "/apis/v1",
		"X-Scheme":       "http",
	}
	for k, v := range expected {
		assert.Equal(v, h.Get(k), fmt.Sprintf("header %s", k))
	}
}

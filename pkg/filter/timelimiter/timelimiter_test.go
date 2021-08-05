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

package timelimiter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestTimeLimiter(t *testing.T) {
	const yamlSpec = `
kind: TimeLimiter
name: timelimiter
defaultTimeoutDuration: 456ms
urls:
  - timeoutDuration: 10ms
    methods: [GET]
    url:
      exact: /timelimit
  - methods: [POST]
    url:
      prefix: /customer
`
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := httppipeline.NewFilterSpec(rawSpec, nil)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	tl := &TimeLimiter{}
	tl.Init(spec)

	if tl.spec.defaultTimeout != 456*time.Millisecond {
		t.Error("default timeout duration is not the value in spec")
	}

	if len(tl.spec.URLs) != 2 {
		t.Error("the length of 'urls' should be 2")
	} else if tl.spec.URLs[0].timeout != 10*time.Millisecond {
		t.Error("timeout duration is not the value in spec")
	}

	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedMethod = func() string {
		return http.MethodGet
	}
	ctx.MockedRequest.MockedPath = func() string {
		return "/timelimit"
	}
	ctx.MockedResponse.MockedStd = func() http.ResponseWriter {
		return &httptest.ResponseRecorder{}
	}
	ctx.MockedCallNextHandler = func(lastResult string) string {
		return ""
	}

	result := tl.Handle(ctx)
	if result == resultTimeout {
		t.Error("timeout should not happen")
	}

	ctx.MockedCallNextHandler = func(lastResult string) string {
		time.Sleep(20 * time.Millisecond)
		return ""
	}

	result = tl.Handle(ctx)
	if result != resultTimeout {
		t.Error("expected timeout didn't happen")
	}

	ctx.MockedRequest.MockedPath = func() string {
		return "/notimelimit"
	}

	result = tl.Handle(ctx)
	if result == resultTimeout {
		t.Error("request path doesn't match, timeout should not happen")
	}

	if tl.Status() != nil {
		t.Error("behavior changed, please update this case")
	}
	tl.Description()

	newTl := &TimeLimiter{}
	spec, _ = httppipeline.NewFilterSpec(rawSpec, nil)
	newTl.Inherit(spec, tl)
	tl.Close()
	result = newTl.Handle(ctx)
	if result == resultTimeout {
		t.Error("request path doesn't match, timeout should not happen")
	}
}

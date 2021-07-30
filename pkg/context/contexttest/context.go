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

package contexttest

import (
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/httpstat"
	"github.com/megaease/easegress/pkg/util/texttemplate"
)

type MockedHTTPContext struct {
	MockedLock               func()
	MockedUnlock             func()
	MockedSpan               func() tracing.Span
	MockedRequest            MockedHTTPRequest
	MockedResponse           MockedHTTPResponse
	MockedDeadline           func() (time.Time, bool)
	MockedDone               func() <-chan struct{}
	MockedErr                func() error
	MockedValue              func(key interface{}) interface{}
	MockedCancel             func(err error)
	MockedCancelled          func() bool
	MockedClientDisconnected func() bool
	MockedDuration           func() time.Duration
	MockedOnFinish           func(func())
	MockedAddTag             func(tag string)
	MockedStatMetric         func() *httpstat.Metric
	MockedLog                func() string
	MockedFinish             func()
	MockedTemplate           func() texttemplate.TemplateEngine
	MockedSetTemplate        func(ht *context.HTTPTemplate)
	MockedSaveReqToTemplate  func(filterName string) error
	MockedSaveRspToTemplate  func(filterName string) error
	MockedCallNextHandler    func(lastResult string) string
	MockedSetHandlerCaller   func(caller context.HandlerCaller)
}

func (c *MockedHTTPContext) Lock() {
	if c.MockedLock != nil {
		c.MockedLock()
	}
}

func (c *MockedHTTPContext) Unlock() {
	if c.MockedUnlock != nil {
		c.MockedUnlock()
	}
}

func (c *MockedHTTPContext) Span() tracing.Span {
	if c.MockedSpan != nil {
		return c.MockedSpan()
	}
	return nil
}

func (c *MockedHTTPContext) Request() context.HTTPRequest {
	return &c.MockedRequest
}

func (c *MockedHTTPContext) Response() context.HTTPResponse {
	return &c.MockedResponse
}

func (c *MockedHTTPContext) Deadline() (deadline time.Time, ok bool) {
	if c.MockedDeadline != nil {
		return c.MockedDeadline()
	}
	return time.Now(), false
}

func (c *MockedHTTPContext) Done() <-chan struct{} {
	if c.MockedDone != nil {
		return c.MockedDone()
	}
	return nil
}

func (c *MockedHTTPContext) Err() error {
	if c.MockedErr != nil {
		return c.MockedErr()
	}
	return nil
}

func (c *MockedHTTPContext) Value(key interface{}) interface{} {
	if c.MockedValue != nil {
		return c.MockedValue(key)
	}
	return nil
}

func (c *MockedHTTPContext) Cancel(err error) {
	if c.MockedCancel != nil {
		c.MockedCancel(err)
	}
}

func (c *MockedHTTPContext) Cancelled() bool {
	if c.MockedCancelled != nil {
		return c.MockedCancelled()
	}
	return false
}

func (c *MockedHTTPContext) ClientDisconnected() bool {
	if c.MockedClientDisconnected != nil {
		return c.MockedClientDisconnected()
	}
	return false
}

func (c *MockedHTTPContext) Duration() time.Duration {
	if c.MockedDuration != nil {
		return c.MockedDuration()
	}
	return 0
}

func (c *MockedHTTPContext) OnFinish(fn func()) {
	if c.MockedFinish != nil {
		c.MockedOnFinish(fn)
	}
}

func (c *MockedHTTPContext) AddTag(tag string) {
	if c.MockedAddTag != nil {
		c.MockedAddTag(tag)
	}
}

func (c *MockedHTTPContext) StatMetric() *httpstat.Metric {
	if c.MockedStatMetric != nil {
		return c.MockedStatMetric()
	}
	return nil
}

func (c *MockedHTTPContext) Log() string {
	if c.MockedLog != nil {
		return c.MockedLog()
	}
	return ""
}

func (c *MockedHTTPContext) Finish() {
	if c.MockedFinish != nil {
		c.MockedFinish()
	}
}

func (c *MockedHTTPContext) Template() texttemplate.TemplateEngine {
	if c.MockedTemplate != nil {
		return c.MockedTemplate()
	}
	return nil
}

func (c *MockedHTTPContext) SetTemplate(ht *context.HTTPTemplate) {
	if c.MockedSetTemplate != nil {
		c.MockedSetTemplate(ht)
	}
}

func (c *MockedHTTPContext) SaveReqToTemplate(filterName string) error {
	if c.MockedSaveReqToTemplate != nil {
		return c.MockedSaveReqToTemplate(filterName)
	}
	return nil
}

func (c *MockedHTTPContext) SaveRspToTemplate(filterName string) error {
	if c.MockedSaveRspToTemplate != nil {
		return c.MockedSaveRspToTemplate(filterName)
	}
	return nil
}

func (c *MockedHTTPContext) CallNextHandler(lastResult string) string {
	if c.MockedCallNextHandler != nil {
		return c.MockedCallNextHandler(lastResult)
	}
	return lastResult
}

func (c *MockedHTTPContext) SetHandlerCaller(caller context.HandlerCaller) {
	if c.MockedSetHandlerCaller != nil {
		c.SetHandlerCaller(caller)
	}
}

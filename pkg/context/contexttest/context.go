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
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/httpstat"
	"github.com/megaease/easegress/pkg/util/texttemplate"
)

// MockedHTTPContext is the mocked HTTP context
type MockedHTTPContext struct {
	lock                     sync.Mutex
	finishFuncs              []func()
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

// Lock mocks the Lock function of HTTPContext
func (c *MockedHTTPContext) Lock() {
	if c.MockedLock != nil {
		c.MockedLock()
	}
}

// Unlock mocks the Unlock function of HTTPContext
func (c *MockedHTTPContext) Unlock() {
	if c.MockedUnlock != nil {
		c.MockedUnlock()
	}
}

// Span mocks the Span function of HTTPContext
func (c *MockedHTTPContext) Span() tracing.Span {
	if c.MockedSpan != nil {
		return c.MockedSpan()
	}
	return tracing.NewSpan(tracing.NoopTracing, "mocked")
}

// Request mocks the Request function of HTTPContext
func (c *MockedHTTPContext) Request() context.HTTPRequest {
	return &c.MockedRequest
}

// Response mocks the Response function of HTTPContext
func (c *MockedHTTPContext) Response() context.HTTPResponse {
	return &c.MockedResponse
}

// Deadline mocks the Deadline function of HTTPContext
func (c *MockedHTTPContext) Deadline() (deadline time.Time, ok bool) {
	if c.MockedDeadline != nil {
		return c.MockedDeadline()
	}
	return time.Now(), false
}

// Done mocks the Done function of HTTPContext
func (c *MockedHTTPContext) Done() <-chan struct{} {
	if c.MockedDone != nil {
		return c.MockedDone()
	}
	return nil
}

// Err mocks the Err function of HTTPContext
func (c *MockedHTTPContext) Err() error {
	if c.MockedErr != nil {
		return c.MockedErr()
	}
	return nil
}

// Value mocks the Value function of HTTPContext
func (c *MockedHTTPContext) Value(key interface{}) interface{} {
	if c.MockedValue != nil {
		return c.MockedValue(key)
	}
	return nil
}

// Cancel mocks the Cancel function of HTTPContext
func (c *MockedHTTPContext) Cancel(err error) {
	if c.MockedCancel != nil {
		c.MockedCancel(err)
	}
}

// Cancelled mocks the Cancelled function of HTTPContext
func (c *MockedHTTPContext) Cancelled() bool {
	if c.MockedCancelled != nil {
		return c.MockedCancelled()
	}
	return false
}

// ClientDisconnected mocks the ClientDisconnected function of HTTPContext
func (c *MockedHTTPContext) ClientDisconnected() bool {
	if c.MockedClientDisconnected != nil {
		return c.MockedClientDisconnected()
	}
	return false
}

// Duration mocks the Duration function of HTTPContext
func (c *MockedHTTPContext) Duration() time.Duration {
	if c.MockedDuration != nil {
		return c.MockedDuration()
	}
	return 0
}

// OnFinish mocks the OnFinish function of HTTPContext
func (c *MockedHTTPContext) OnFinish(fn func()) {
	if c.MockedFinish != nil {
		c.MockedOnFinish(fn)
	}
	c.lock.Lock()
	c.finishFuncs = append(c.finishFuncs, fn)
	c.lock.Unlock()
}

// AddTag mocks the AddTag function of HTTPContext
func (c *MockedHTTPContext) AddTag(tag string) {
	if c.MockedAddTag != nil {
		c.MockedAddTag(tag)
	}
}

// StatMetric mocks the StatMetric function of HTTPContext
func (c *MockedHTTPContext) StatMetric() *httpstat.Metric {
	if c.MockedStatMetric != nil {
		return c.MockedStatMetric()
	}
	return nil
}

// Log mocks the Log function of HTTPContext
func (c *MockedHTTPContext) Log() string {
	if c.MockedLog != nil {
		return c.MockedLog()
	}
	return ""
}

// Finish mocks the Finish function of HTTPContext
func (c *MockedHTTPContext) Finish() {
	if c.MockedFinish != nil {
		c.MockedFinish()
		return
	}

	c.lock.Lock()
	for _, fn := range c.finishFuncs {
		func() {
			defer func() {
				if err := recover(); err != nil {
				}
			}()
			fn()
		}()
	}
	c.lock.Unlock()
}

// Template mocks the Template function of HTTPContext
func (c *MockedHTTPContext) Template() texttemplate.TemplateEngine {
	if c.MockedTemplate != nil {
		return c.MockedTemplate()
	}
	return nil
}

// SetTemplate mocks the SetTemplate function of HTTPContext
func (c *MockedHTTPContext) SetTemplate(ht *context.HTTPTemplate) {
	if c.MockedSetTemplate != nil {
		c.MockedSetTemplate(ht)
	}
}

// SaveReqToTemplate mocks the SaveReqToTemplate function of HTTPContext
func (c *MockedHTTPContext) SaveReqToTemplate(filterName string) error {
	if c.MockedSaveReqToTemplate != nil {
		return c.MockedSaveReqToTemplate(filterName)
	}
	return nil
}

// SaveRspToTemplate mocks the SaveRspToTemplate function of HTTPContext
func (c *MockedHTTPContext) SaveRspToTemplate(filterName string) error {
	if c.MockedSaveRspToTemplate != nil {
		return c.MockedSaveRspToTemplate(filterName)
	}
	return nil
}

// CallNextHandler mocks the CallNextHandler function of HTTPContext
func (c *MockedHTTPContext) CallNextHandler(lastResult string) string {
	if c.MockedCallNextHandler != nil {
		return c.MockedCallNextHandler(lastResult)
	}
	return lastResult
}

// SetHandlerCaller mocks the SetHandlerCaller function of HTTPContext
func (c *MockedHTTPContext) SetHandlerCaller(caller context.HandlerCaller) {
	if c.MockedSetHandlerCaller != nil {
		c.SetHandlerCaller(caller)
	}
}

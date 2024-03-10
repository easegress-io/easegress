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

// Package contexttest provides utilities for testing context.
package contexttest

import "github.com/megaease/easegress/v2/pkg/context"

// MockedMuxMapper is a mux mapper for mocking in testing.
type MockedMuxMapper struct {
	MockedGetHandler func(name string) (context.Handler, bool)
}

// GetHandler implements context.MuxMapper.
func (mmm *MockedMuxMapper) GetHandler(name string) (context.Handler, bool) {
	if mmm.MockedGetHandler == nil {
		return nil, false
	}
	return mmm.MockedGetHandler(name)
}

// MockedHandler is a mocked handler for mocking in testing.
type MockedHandler struct {
	MockedHandle func(ctx *context.Context) string
}

// Handle implements context.Handler.
func (mh *MockedHandler) Handle(ctx *context.Context) string {
	if mh.MockedHandle == nil {
		return ""
	}
	return mh.MockedHandle(ctx)
}

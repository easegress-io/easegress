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

package mqttproxy

import (
	"testing"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/openzipkin/zipkin-go/model"
)

func init() {
	logger.InitMock()
}

func TestSpanLog(t *testing.T) {
	context := &model.SpanContext{
		TraceID: model.TraceID{
			High: 100,
			Low:  200,
		},
		ID: 300,
	}
	spanDebugf(context, "test span log, %v", "debug with context")
	spanDebugf(nil, "test span log, %v", "debug without context")
	spanErrorf(context, "test span log, %v", "error with context")
	spanDebugf(nil, "test span log, %v", "error without context")
}

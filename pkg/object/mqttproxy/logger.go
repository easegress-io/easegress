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
	"github.com/megaease/easegress/pkg/logger"
	"github.com/openzipkin/zipkin-go/model"
)

func spanDebugf(context *model.SpanContext, template string, args ...interface{}) {
	temp := "tid=%v sid=%v " + template
	if context == nil {
		logger.Debugf(temp, nil, nil, args)
	} else {
		logger.Debugf(temp, context.TraceID, context.ID, args)
	}
}

func spanErrorf(context *model.SpanContext, template string, args ...interface{}) {
	temp := "tid=%v sid=%v " + template
	if context == nil {
		logger.Errorf(temp, nil, nil, args)
	} else {
		logger.Errorf(temp, context.TraceID, context.ID, args)
	}
}

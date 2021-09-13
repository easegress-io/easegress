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

package protocol

import (
	"github.com/megaease/easegress/pkg/context"
)

type (
	// Layer4Handler is the common handler for the all backends
	// which handle the traffic from layer4(tcp/udp) server.
	Layer4Handler interface {

		// InboundHandler filter handle inbound stream from client via ctx
		// put handle result to object and pass to next filter
		InboundHandler(ctx context.Layer4Context, object interface{})

		// OutboundHandler filter handle inbound stream from upstream via ctx
		// put handle result to object and pass to next filter
		OutboundHandler(ctx context.Layer4Context, object interface{})
	}

	// Layer4MuxMapper gets layer4 handler pipeline with mutex
	Layer4MuxMapper interface {
		GetHandler(name string) (Layer4Handler, bool)
	}
)

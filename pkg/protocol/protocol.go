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

// Package protocol gathers protocol-specific stuff,
// which eliminate the coupling of framework and implementation.
package protocol

import (
	"github.com/megaease/easegress/pkg/context"
)

type (
	// HTTPHandler is the common handler for the all backends
	// which handle the traffic from HTTPServer.
	HTTPHandler interface {
		Handle(ctx context.HTTPContext)
	}

	// Layer4Handler is the common handler for the all backends
	// which handle the traffic from layer4(tcp/udp) server.
	Layer4Handler interface {
		// Handle read buffer from context, and set write buffer to context,
		// its filter's response to release read buffer in context
		// and its filter's response to determine which time to flush buffer to client or upstream
		Handle(ctx context.Layer4Context)
	}

	// MuxMapper gets HTTP handler pipeline with mutex
	MuxMapper interface {
		// GetHTTPHandler get http handler from mux
		GetHTTPHandler(name string) (HTTPHandler, bool)

		// GetLayer4Handler get layer4 handler from mux
		GetLayer4Handler(name string) (Layer4Handler, bool)
	}
)

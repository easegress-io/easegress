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

// Package protocols gathers protocol-specific stuff,
// which eliminate the coupling of framework and implementation.
package protocols

import (
	"github.com/megaease/easegress/pkg/context"
)

type (
	// HTTPHandler is the common handler for the all backends
	// which handle the traffic from HTTPServer.
	HTTPHandler interface {
		Handle(ctx context.HTTPContext) string
	}

	// MuxMapper gets HTTP handler pipeline with mutex
	MuxMapper interface {
		GetHandler(name string) (HTTPHandler, bool)
	}
)

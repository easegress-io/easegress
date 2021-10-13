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

package bridge

import (
	"net/http"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/protocol"
)

const (
	// Kind is the kind of Bridge.
	Kind = "Bridge"

	// Description is the Description of Bridge.
	Description = `# Bridge Filter

A Bridge Filter route requests to from one pipeline to other pipelines or http proxies under a http server.

1. The upstream filter set the target pipeline/proxy to the http header,  'X-Easegress-Bridge-Dest'.
2. Bridge will extract the value from 'X-Easegress-Bridge-Dest' and try to match in the configuration.
   It will send the request if a dest matched. abort the process if no match.
3. Bridge will select the first dest from the filter configuration if there's no header named 'X-Easegress-Bridge-Dest'`

	resultDestinationNotFound     = "destinationNotFound"
	resultInvokeDestinationFailed = "invokeDestinationFailed"

	bridgeDestHeader = "X-Easegress-Bridge-Dest"
)

var results = []string{resultDestinationNotFound, resultInvokeDestinationFailed}

func init() {
	// FIXME: Bridge is a temporary product for some historical reason.
	// I(@xxx7xxxx) think we should not empower filter to cross pipelines.

	// httppipeline.Register(&Bridge{})
}

type (
	// Bridge is filter Bridge.
	Bridge struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		muxMapper protocol.MuxMapper
	}

	// Spec describes the Mock.
	Spec struct {
		Destinations []string `yaml:"destinations" jsonschema:"required,pattern=^[^ \t]+$"`
	}
)

// Kind returns the kind of Bridge.
func (b *Bridge) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Bridge.
func (b *Bridge) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Bridge.
func (b *Bridge) Description() string {
	return Description
}

// Results returns the results of Bridge.
func (b *Bridge) Results() []string {
	return results
}

// Init initializes Bridge.
func (b *Bridge) Init(filterSpec *httppipeline.FilterSpec) {
	b.filterSpec, b.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	b.reload()
}

// Inherit inherits previous generation of Bridge.
func (b *Bridge) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	b.Init(filterSpec)
}

func (b *Bridge) reload() {
	if len(b.spec.Destinations) <= 0 {
		logger.Errorf("not any destination defined")
	}
}

// Handle builds a bridge for pipeline.
func (b *Bridge) Handle(ctx context.HTTPContext) (result string) {
	result = b.handle(ctx)
	return ctx.CallNextHandler(result)
}

// InjectMuxMapper injects mux mapper into Bridge.
func (b *Bridge) InjectMuxMapper(mapper protocol.MuxMapper) {
	b.muxMapper = mapper
}

func (b *Bridge) handle(ctx context.HTTPContext) (result string) {
	if len(b.spec.Destinations) <= 0 {
		panic("not any destination defined")
	}

	r := ctx.Request()
	dest := r.Header().Get(bridgeDestHeader)
	found := false
	if dest == "" {
		logger.Warnf("destination not defined, will choose the first dest: %s", b.spec.Destinations[0])
		dest = b.spec.Destinations[0]
		found = true
	} else {
		for _, d := range b.spec.Destinations {
			if d == dest {
				r.Header().Del(bridgeDestHeader)
				found = true
				break
			}
		}
	}

	if !found {
		logger.Errorf("dest not found: %s", dest)
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		return resultDestinationNotFound
	}

	handler, exists := b.muxMapper.GetHandler(dest)

	if !exists {
		logger.Errorf("failed to get running object %s", b.spec.Destinations[0])
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		return resultDestinationNotFound
	}

	handler.Handle(ctx)

	return ""
}

// Status returns status.
func (b *Bridge) Status() interface{} {
	return nil
}

// Close closes Bridge.
func (b *Bridge) Close() {}

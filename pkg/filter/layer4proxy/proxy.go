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

package layer4proxy

import (
	"fmt"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/layer4pipeline"
	"github.com/megaease/easegress/pkg/util/fallback"
)

const (
	// Kind is the kind of Proxy.
	Kind = "Proxy"

	resultInternalError = "internalError"
	resultClientError   = "clientError"
	resultServerError   = "serverError"
)

var results = []string{
	resultInternalError,
	resultClientError,
	resultServerError,
}

func init() {
	layer4pipeline.Register(&Proxy{})
}

type (
	// Proxy is the filter Proxy.
	Proxy struct {
		filterSpec *layer4pipeline.FilterSpec
		spec       *Spec

		fallback *fallback.Fallback

		mainPool       *pool
		candidatePools []*pool
		mirrorPool     *pool
	}

	// Spec describes the Proxy.
	Spec struct {
		Fallback       *FallbackSpec `yaml:"fallback,omitempty" jsonschema:"omitempty"`
		MainPool       *PoolSpec     `yaml:"mainPool" jsonschema:"required"`
		CandidatePools []*PoolSpec   `yaml:"candidatePools,omitempty" jsonschema:"omitempty"`
		MirrorPool     *PoolSpec     `yaml:"mirrorPool,omitempty" jsonschema:"omitempty"`
		FailureCodes   []int         `yaml:"failureCodes" jsonschema:"omitempty,uniqueItems=true,format=httpcode-array"`
	}

	// FallbackSpec describes the fallback policy.
	FallbackSpec struct {
		ForCodes      bool `yaml:"forCodes"`
		fallback.Spec `yaml:",inline"`
	}

	// Status is the status of Proxy.
	Status struct {
		MainPool       *PoolStatus   `yaml:"mainPool"`
		CandidatePools []*PoolStatus `yaml:"candidatePools,omitempty"`
		MirrorPool     *PoolStatus   `yaml:"mirrorPool,omitempty"`
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	// NOTE: The tag of v parent may be behind mainPool.
	if s.MainPool == nil {
		return fmt.Errorf("mainPool is required")
	}

	if s.MainPool.Filter != nil {
		return fmt.Errorf("filter must be empty in mainPool")
	}

	if len(s.CandidatePools) > 0 {
		for _, v := range s.CandidatePools {
			if v.Filter == nil {
				return fmt.Errorf("filter of candidatePool is required")
			}
		}
	}

	if s.MirrorPool != nil {
		if s.MirrorPool.Filter == nil {
			return fmt.Errorf("filter of mirrorPool is required")
		}
	}

	if len(s.FailureCodes) == 0 {
		if s.Fallback != nil {
			return fmt.Errorf("fallback needs failureCodes")
		}
	}

	return nil
}

// Kind returns the kind of Proxy.
func (b *Proxy) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Proxy.
func (b *Proxy) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Proxy.
func (b *Proxy) Description() string {
	return "Proxy sets the proxy of proxy servers"
}

// Results returns the results of Proxy.
func (b *Proxy) Results() []string {
	return results
}

// Init initializes Proxy.
func (b *Proxy) Init(filterSpec *layer4pipeline.FilterSpec) {
	b.filterSpec, b.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	b.reload()
}

// Inherit inherits previous generation of Proxy.
func (b *Proxy) Inherit(filterSpec *layer4pipeline.FilterSpec, previousGeneration layer4pipeline.Filter) {
	previousGeneration.Close()
	b.Init(filterSpec)
}

func (b *Proxy) reload() {
	super := b.filterSpec.Super()

	b.mainPool = newPool(super, b.spec.MainPool, "proxy#main")

	if b.spec.Fallback != nil {
		b.fallback = fallback.New(&b.spec.Fallback.Spec)
	}

	if len(b.spec.CandidatePools) > 0 {
		var candidatePools []*pool
		for k := range b.spec.CandidatePools {
			candidatePools = append(candidatePools,
				newPool(super, b.spec.CandidatePools[k], fmt.Sprintf("proxy#candidate#%d", k)))
		}
		b.candidatePools = candidatePools
	}
	if b.spec.MirrorPool != nil {
		b.mirrorPool = newPool(super, b.spec.MirrorPool, "proxy#mirror")
	}
}

// Status returns Proxy status.
func (b *Proxy) Status() interface{} {
	s := &Status{
		MainPool: b.mainPool.status(),
	}
	if b.candidatePools != nil {
		for k := range b.candidatePools {
			s.CandidatePools = append(s.CandidatePools, b.candidatePools[k].status())
		}
	}
	if b.mirrorPool != nil {
		s.MirrorPool = b.mirrorPool.status()
	}
	return s
}

// Close closes Proxy.
func (b *Proxy) Close() {
	b.mainPool.close()

	if b.candidatePools != nil {
		for _, v := range b.candidatePools {
			v.close()
		}
	}

	if b.mirrorPool != nil {
		b.mirrorPool.close()
	}
}

func (b *Proxy) fallbackForCodes(ctx context.HTTPContext) bool {
	if b.fallback != nil && b.spec.Fallback.ForCodes {
		for _, code := range b.spec.FailureCodes {
			if ctx.Response().StatusCode() == code {
				b.fallback.Fallback(ctx)
				return true
			}
		}
	}
	return false
}

// Handle handles HTTPContext.
func (b *Proxy) Handle(ctx context.Layer4Context) (result string) {
	result = b.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (b *Proxy) handle(ctx context.Layer4Context) (result string) {
	// TODO add mirror pool proxy

	var p *pool
	if len(b.candidatePools) > 0 {
		for k, v := range b.candidatePools {
			if v.filter.Filter(ctx) {
				p = b.candidatePools[k]
				break
			}
		}
	}

	if p == nil {
		p = b.mainPool
	}
	return p.handle(ctx)
}

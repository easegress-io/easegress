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

package layer4filter

import (
	"math/rand"
	"net"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/util/hashtool"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	policyIPHash string = "ipHash"
	policyRandom        = "random"
)

type (
	// Spec describes Layer4filter.
	Spec struct {
		Probability *Probability `yaml:"probability,omitempty" jsonschema:"omitempty"`
	}

	// Layer4filter filters layer4 traffic.
	Layer4filter struct {
		spec *Spec
	}

	// Probability filters layer4 traffic by probability.
	Probability struct {
		PerMill uint32 `yaml:"perMill" jsonschema:"required,minimum=1,maximum=1000"`
		Policy  string `yaml:"policy" jsonschema:"required,enum=ipHash,enum=headerHash,enum=random"`
	}
)

// New creates an HTTPFilter.
func New(spec *Spec) *Layer4filter {
	hf := &Layer4filter{
		spec: spec,
	}
	return hf
}

// Filter filters Layer4Context.
func (hf *Layer4filter) Filter(ctx context.Layer4Context) bool {
	return hf.filterProbability(ctx)
}

func (hf *Layer4filter) filterProbability(ctx context.Layer4Context) bool {
	prob := hf.spec.Probability

	var result uint32
	switch prob.Policy {
	case policyRandom:
		result = uint32(rand.Int31n(1000))
	case policyIPHash:
	default:
		host, _, _ := net.SplitHostPort(ctx.RemoteAddr().String())
		result = hashtool.Hash32(host)
	}

	if result%1000 < prob.PerMill {
		return true
	}
	return false
}

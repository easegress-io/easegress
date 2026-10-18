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

package tcpproxy

import (
	"sync"
	"sync/atomic"
)

// LoadBalancer selects a backend from the list of available backends.
type LoadBalancer interface {
	// Next returns the next backend address.
	Next() string
}

// RoundRobin implements simple round-robin load balancing.
type RoundRobin struct {
	backends []*Backend
	index    uint64
}

// NewRoundRobin creates a new round-robin load balancer.
func NewRoundRobin(backends []*Backend) *RoundRobin {
	return &RoundRobin{
		backends: backends,
		index:    0,
	}
}

// Next returns the next backend using round-robin strategy.
func (rr *RoundRobin) Next() string {
	if len(rr.backends) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&rr.index, 1)
	return rr.backends[(idx-1)%uint64(len(rr.backends))].Address
}

// WeightedRoundRobin implements weighted round-robin load balancing.
type WeightedRoundRobin struct {
	mu          sync.Mutex
	backends    []*weightedBackend
	totalWeight uint32
}

type weightedBackend struct {
	address       string
	weight        uint32
	currentWeight int64
}

// NewWeightedRoundRobin creates a new weighted round-robin load balancer.
func NewWeightedRoundRobin(backends []*Backend) *WeightedRoundRobin {
	wbs := make([]*weightedBackend, 0, len(backends))
	var totalWeight uint32
	for _, b := range backends {
		w := b.Weight
		if w == 0 {
			w = 1
		}
		wbs = append(wbs, &weightedBackend{
			address: b.Address,
			weight:  w,
		})
		totalWeight += w
	}

	return &WeightedRoundRobin{
		backends:    wbs,
		totalWeight: totalWeight,
	}
}

// Next returns the next backend using weighted round-robin strategy.
// Uses the smooth weighted round-robin algorithm.
func (wrr *WeightedRoundRobin) Next() string {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	if len(wrr.backends) == 0 {
		return ""
	}

	// Increase each backend's currentWeight by its weight.
	for _, b := range wrr.backends {
		b.currentWeight += int64(b.weight)
	}

	// Find the backend with the highest currentWeight.
	// When there's a tie, prefer the one after the previously selected
	// to ensure smooth distribution.
	var best *weightedBackend
	var bestWeight int64 = -1

	for _, b := range wrr.backends {
		if b.currentWeight > bestWeight {
			best = b
			bestWeight = b.currentWeight
		}
	}

	if best == nil {
		return wrr.backends[0].address
	}

	// Decrease the selected backend's currentWeight by the totalWeight.
	best.currentWeight -= int64(wrr.totalWeight)

	return best.address
}

// NewLoadBalancer creates a load balancer based on the given strategy.
func NewLoadBalancer(strategy string, backends []*Backend) LoadBalancer {
	switch strategy {
	case LoadBalanceWeightedRoundRobin:
		return NewWeightedRoundRobin(backends)
	default:
		return NewRoundRobin(backends)
	}
}

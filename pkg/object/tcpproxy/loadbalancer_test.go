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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobin(t *testing.T) {
	assertions := assert.New(t)

	backends := []*Backend{
		{Address: "127.0.0.1:8081"},
		{Address: "127.0.0.1:8082"},
		{Address: "127.0.0.1:8083"},
	}

	rr := NewRoundRobin(backends)

	// Test round-robin distribution.
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		addr := rr.Next()
		counts[addr]++
	}

	assertions.Equal(34, counts["127.0.0.1:8081"])
	assertions.Equal(33, counts["127.0.0.1:8082"])
	assertions.Equal(33, counts["127.0.0.1:8083"])
}

func TestRoundRobinSingleBackend(t *testing.T) {
	assertions := assert.New(t)

	backends := []*Backend{
		{Address: "127.0.0.1:8081"},
	}

	rr := NewRoundRobin(backends)

	for i := 0; i < 10; i++ {
		addr := rr.Next()
		assertions.Equal("127.0.0.1:8081", addr)
	}
}

func TestRoundRobinEmptyBackends(t *testing.T) {
	assertions := assert.New(t)

	rr := NewRoundRobin([]*Backend{})
	assertions.Equal("", rr.Next())
}

func TestWeightedRoundRobin(t *testing.T) {
	assertions := assert.New(t)

	backends := []*Backend{
		{Address: "127.0.0.1:8081", Weight: 3},
		{Address: "127.0.0.1:8082", Weight: 1},
	}

	wrr := NewWeightedRoundRobin(backends)

	// With smooth weighted round-robin, distribution should be ~3:1.
	counts := make(map[string]int)
	total := 1000
	for i := 0; i < total; i++ {
		addr := wrr.Next()
		counts[addr]++
	}

	// Allow some variance.
	assertions.InDelta(750, counts["127.0.0.1:8081"], 50)
	assertions.InDelta(250, counts["127.0.0.1:8082"], 50)
}

func TestWeightedRoundRobinEqualWeights(t *testing.T) {
	assertions := assert.New(t)

	backends := []*Backend{
		{Address: "127.0.0.1:8081", Weight: 1},
		{Address: "127.0.0.1:8082", Weight: 1},
		{Address: "127.0.0.1:8083", Weight: 1},
	}

	wrr := NewWeightedRoundRobin(backends)

	counts := make(map[string]int)
	total := 900
	for i := 0; i < total; i++ {
		addr := wrr.Next()
		counts[addr]++
	}

	assertions.InDelta(300, counts["127.0.0.1:8081"], 30)
	assertions.InDelta(300, counts["127.0.0.1:8082"], 30)
	assertions.InDelta(300, counts["127.0.0.1:8083"], 30)
}

func TestWeightedRoundRobinEmptyBackends(t *testing.T) {
	assertions := assert.New(t)

	wrr := NewWeightedRoundRobin([]*Backend{})
	assertions.Equal("", wrr.Next())
}

func TestNewLoadBalancer(t *testing.T) {
	assertions := assert.New(t)

	backends := []*Backend{
		{Address: "127.0.0.1:8081", Weight: 2},
		{Address: "127.0.0.1:8082", Weight: 1},
	}

	// Test round robin strategy.
	lb := NewLoadBalancer(LoadBalanceRoundRobin, backends)
	assertions.NotNil(lb)
	_, isRR := lb.(*RoundRobin)
	assertions.True(isRR)

	// Test weighted round robin strategy.
	lb = NewLoadBalancer(LoadBalanceWeightedRoundRobin, backends)
	assertions.NotNil(lb)
	_, isWRR := lb.(*WeightedRoundRobin)
	assertions.True(isWRR)

	// Test default (empty string) -> round robin.
	lb = NewLoadBalancer("", backends)
	assertions.NotNil(lb)
	_, isRR = lb.(*RoundRobin)
	assertions.True(isRR)
}

func TestWeightedRoundRobinZeroWeight(t *testing.T) {
	assertions := assert.New(t)

	backends := []*Backend{
		{Address: "127.0.0.1:8081", Weight: 0},
		{Address: "127.0.0.1:8082", Weight: 0},
	}

	// Zero weights should default to 1.
	wrr := NewWeightedRoundRobin(backends)

	counts := make(map[string]int)
	total := 200
	for i := 0; i < total; i++ {
		addr := wrr.Next()
		counts[addr]++
	}

	assertions.InDelta(100, counts["127.0.0.1:8081"], 20)
	assertions.InDelta(100, counts["127.0.0.1:8082"], 20)
}

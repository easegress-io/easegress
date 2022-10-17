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

package proxy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertEven(t *testing.T, svrs []*Server, slots []*Server) {
	total := 0
	m := map[string]int{}
	for _, s := range slots {
		m[s.ID()] = m[s.ID()] + 1
	}

	avg := Total / len(svrs)
	for _, s := range svrs {
		ls := m[s.ID()]
		assert.GreaterOrEqual(t, ls, avg)
		total += ls
	}
	assert.Equal(t, Total, total)
}

func assertConsistent(t *testing.T, oldSvrs []*Server, oldSlots []*Server, newSvrs []*Server, newSlots []*Server) {

	lo, ln := len(oldSvrs), len(newSvrs)
	size := lo
	if lo < ln {
		size = ln
	}

	om := map[string]bool{}
	for _, s := range oldSvrs {
		om[s.ID()] = true
	}

	m := map[string]int{}
	for i, s := range newSlots {
		if s.ID() == oldSlots[i].ID() {
			m[s.ID()] = m[s.ID()] + 1
		}
	}

	for _, s := range newSvrs {
		if om[s.ID()] {
			assert.GreaterOrEqual(t, m[s.ID()], Total/size)
		}
	}
}

func create(ids ...int) []*Server {
	svrs := make([]*Server, len(ids))
	for i, id := range ids {
		svrs[i] = &Server{URL: fmt.Sprintf("192.168.1.%d", id)}
	}
	return svrs
}

func increase(from []*Server, ids ...int) []*Server {
	to := make([]*Server, len(from))
	for i, s := range from {
		to[i] = s
	}
	for _, id := range ids {
		to = append(to, &Server{URL: fmt.Sprintf("192.168.1.%d", id)})
	}
	return to
}

func copy(from []*Server) []*Server {
	to := make([]*Server, len(from))
	for i, s := range from {
		to[i] = s
	}
	return to
}

func replace(from []*Server, ids ...int) []*Server {
	to := copy(from)
	for i, id := range ids {
		to[i] = &Server{URL: fmt.Sprintf("192.168.1.%d", id)}
	}
	return to
}

func TestInitServer(t *testing.T) {
	newSvrs := create(1, 2, 3)
	newSlots := createSlots(nil, newSvrs)
	assertEven(t, newSvrs, newSlots)

	newSvrs = create(1, 2, 3, 4, 5)
	newSlots = createSlots(nil, newSvrs)
	assertEven(t, newSvrs, newSlots)
}

func TestIncreaseServer(t *testing.T) {
	oldSvrs := create(1, 2, 3)
	oldSlots := createSlots(nil, oldSvrs)
	newSvrs := increase(oldSvrs, 4)
	newSlots := createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)

	oldSvrs, oldSlots = copy(newSvrs), copy(newSlots)
	newSvrs = increase(oldSvrs, 5, 6)
	newSlots = createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)
}

func TestReduceServer(t *testing.T) {
	oldSvrs := create(1, 2, 3, 4, 5)
	oldSlots := createSlots(nil, oldSvrs)
	newSvrs := oldSvrs[1:]
	newSlots := createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)

	oldSvrs, oldSlots = copy(newSvrs), copy(newSlots)
	newSvrs = oldSvrs[2:]
	newSlots = createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)
}

func TestReplaceServer(t *testing.T) {
	oldSvrs := create(1, 2, 3)
	oldSlots := createSlots(nil, oldSvrs)
	newSvrs := replace(oldSvrs, 4)
	newSlots := createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)

	oldSvrs, oldSlots = copy(newSvrs), copy(newSlots)
	newSvrs = replace(oldSvrs, 5, 6)
	newSlots = createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)
}

func TestReorderServer(t *testing.T) {
	oldSvrs := create(1, 2, 3)
	oldSlots := createSlots(nil, oldSvrs)
	newSvrs := replace(oldSvrs, 1, 3, 2)
	newSlots := createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)

	oldSvrs, oldSlots = copy(newSvrs), copy(newSlots)
	newSvrs = replace(oldSvrs, 2, 3, 1)
	newSlots = createSlots(oldSlots, newSvrs)
	assertEven(t, newSvrs, newSlots)
	assertConsistent(t, oldSvrs, oldSlots, newSvrs, newSlots)
}

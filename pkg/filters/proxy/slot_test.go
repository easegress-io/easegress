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

func assertEven(t *testing.T, svrs []*Server) {
	total := 0
	for i, s := range svrs {
		size := avg(len(svrs), i)
		ls := len(s.slots)
		assert.GreaterOrEqual(t, ls, size)
		total += ls
	}
	assert.Equal(t, Total, total)
}

func assertConsistent(t *testing.T, from []*Server, to []*Server) {
	m := map[string]*Server{}
	for _, s := range to {
		m[s.ID()] = s
	}

	// compute consistent size
	lf, lt := len(from), len(to)
	lc := Total / lf
	if lt > lf {
		lc = Total / lt
	}

	for _, s := range from {
		if ns := m[s.ID()]; ns != nil {
			c := 0
			for i, p := range ns.slots {
				if p == s.slots[i] {
					c++
				}
			}
			assert.GreaterOrEqual(t, c, lc)
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
	to := create(1, 2, 3)
	to = hashSlots(nil, to)
	assertEven(t, to)

	to = create(1, 2, 3, 4, 5)
	to = hashSlots(nil, to)
	assertEven(t, to)
}

func TestIncreaseServer(t *testing.T) {
	from := hashSlots(nil, create(1, 2, 3))
	to := increase(from, 4)
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)

	from = copy(to)
	to = increase(from, 5, 6)
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)
}

func TestReduceServer(t *testing.T) {
	from := hashSlots(nil, create(1, 2, 3, 45))
	to := from[1:]
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)

	from = copy(to)
	to = to[2:]
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)
}

func TestReplaceServer(t *testing.T) {
	from := hashSlots(nil, create(1, 2, 3))
	to := replace(from, 4)
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)

	from = copy(to)
	to = replace(to, 5, 6)
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)
}

func TestReorderServer(t *testing.T) {
	from := hashSlots(nil, create(1, 2, 3))
	to := replace(from, 1, 3, 2)
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)

	from = copy(to)
	to = replace(to, 2, 3, 1)
	to = hashSlots(from, to)
	assertEven(t, to)
	assertConsistent(t, from, to)
}

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

func assertBalance(t *testing.T, svrs []*Server) {

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

func TestHashSlots(t *testing.T) {
	to := make([]*Server, 0, 3)
	for i := 0; i < 3; i++ {
		to = append(to, &Server{URL: fmt.Sprintf("192.168.1.%d", i+1)})
	}
	to = hashSlots(nil, to)
	assertBalance(t, to)

	from := make([]*Server, len(to))
	copy(from, to)
	to = append(from, &Server{URL: fmt.Sprintf("192.168.1.%d", 4)})
	to = hashSlots(to, from)
	assertBalance(t, to)
	assertConsistent(t, from, to)

	from = make([]*Server, len(to))
	copy(from, to)
	to = append(from, &Server{URL: fmt.Sprintf("192.168.1.%d", 5)})
	to = append(to, &Server{URL: fmt.Sprintf("192.168.1.%d", 6)})
	to = hashSlots(from, to)
	assertBalance(t, to)
	assertConsistent(t, from, to)

	from = make([]*Server, len(to))
	copy(from, to)
	to = to[:len(to)-1]
	to = append(to, &Server{URL: fmt.Sprintf("192.168.1.%d", 7)})
	to = hashSlots(from, to)
	assertBalance(t, to)
	assertConsistent(t, from, to)

	from = make([]*Server, len(to))
	copy(from, to)
	to = to[1:]
	to = hashSlots(from, to)
	assertBalance(t, to)
	assertConsistent(t, from, to)

	from = make([]*Server, len(to))
	copy(from, to)
	to = to[2:]
	to = hashSlots(from, to)
	assertBalance(t, to)
	assertConsistent(t, from, to)
}

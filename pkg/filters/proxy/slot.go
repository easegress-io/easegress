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

const (
	// Total is the total size of slots
	Total = 1024
)

// avg counts averages size of slots
func avg(size, pos int) int {
	avg := Total / size
	// servers ahead can get one more
	if pos <= Total%size-1 {
		avg++
	}
	return avg
}

// hashSlots repartitions slots for changed servers
func hashSlots(from []*Server, to []*Server) []*Server {
	// distribute evenly for first time
	lf, lt := len(from), len(to)
	if lf == 0 {
		// init slots
		all := make([]int, Total)
		for i := range all {
			all[i] = i
		}

		// distribute evenly
		s := 0
		for i, svr := range to {
			size := avg(lt, i)
			svr.slots = all[s : s+size]
			s += size
		}

		return to
	}

	// use map for compare
	m := make(map[string]*Server, lt)
	for _, s := range to {
		m[s.ID()] = s
	}

	// handle slots
	c := make([]int, 0)
	svrs := make([]*Server, lt)
	pos := 0
	for _, s := range from {
		ns := m[s.ID()]
		if ns == nil {
			// collect slots from lost server
			c = append(c, s.slots...)
			continue
		}

		// collect exceeding slots from existing server
		size := avg(lt, pos)
		if len(s.slots) > size {
			c = append(c, s.slots[size:]...)
		}

		// copy slots from existing server in order
		ns.slots = s.slots
		svrs[pos] = ns
		delete(m, s.ID())
		pos++
	}

	// copy new servers
	for _, s := range m {
		svrs[pos] = s
		pos++
	}

	// check slots size
	s := 0
	for i, svr := range svrs {
		size := avg(len(svrs), i)
		add := size - len(svr.slots)
		if add > 0 {
			// fill up lacking slots
			svr.slots = append(svr.slots, c[s:s+add]...)
			s += add
		} else if add < 0 {
			// prune exceeding slots
			svr.slots = svr.slots[:size]
		}
	}

	return svrs
}

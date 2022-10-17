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

// slotSize counts slot size of server at specific pos
func slotSize(svrSize, pos int) int {
	s := Total / svrSize
	// servers ahead can get one more
	if pos <= Total%svrSize-1 {
		s++
	}
	return s
}

// createSlots creates slots for servers using old slots
func createSlots(oldSlots []*Server, servers []*Server) []*Server {
	if len(servers) == 0 {
		return nil
	}

	newSlots := make([]*Server, Total)
	svrSize := len(servers)

	// distribute evenly first time
	if len(oldSlots) == 0 {
		p := 0
		for i, svr := range servers {
			size := slotSize(svrSize, i)
			for j := 0; j < size; j++ {
				newSlots[p+j] = svr
			}
			p += size
		}
		return newSlots
	}

	// use server map to compare
	svrm := map[string]*Server{}
	for _, s := range servers {
		svrm[s.ID()] = s
	}

	// handle slots
	p := 0
	sizem := map[string]int{}
	slotsm := map[string][]int{}
	c := make([]int, 0)
	for i, svr := range oldSlots {
		// collect slots from lost server
		nsvr := svrm[svr.ID()]
		if nsvr == nil {
			c = append(c, i)
			continue
		}

		// collect exceeding slots from existing server
		slots := slotsm[svr.ID()]
		size := sizem[svr.ID()]
		if size != 0 && len(slots) >= size {
			c = append(c, i)
			continue
		}

		// keep normal slots for existing server
		newSlots[i] = svr

		// count slots for existing server in original order
		if slots == nil {
			slots = make([]int, 0)
			sizem[svr.ID()] = slotSize(svrSize, p)
			p++
		}
		slotsm[svr.ID()] = append(slots, i)
	}

	// count slots size for new server
	for id := range svrm {
		if sizem[id] == 0 {
			sizem[id] = slotSize(svrSize, p)
			p++
		}
	}

	// fill up slots for lacking server
	for _, s := range servers {
		slots := slotsm[s.ID()]
		if lack := sizem[s.ID()] - len(slots); lack > 0 {
			for j := 0; j < lack; j++ {
				newSlots[c[j]] = s
			}
			c = c[lack:]
		}
	}
	return newSlots
}

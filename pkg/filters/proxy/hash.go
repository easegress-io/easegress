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
	// SlotSize is the size of slots
	SlotSize = 1024
)

// SlotHash define a slot hash
type SlotHash struct {
	slots   *[]string
	servers []*Server
	slotses map[string][]int
	sizes   map[string]int
}

// calucate calucates slots map and size map for servers
func (s *SlotHash) calucate() {
	if *s.slots == nil {
		*s.slots = make([]string, SlotSize)
	}
	slotL, svrL := len(*s.slots), len(s.servers)
	s.slotses, s.sizes = make(map[string][]int, svrL), make(map[string]int, svrL)
	for i, svr := range s.servers {
		id := svr.ID()
		s.slotses[id] = make([]int, 0, slotL/svrL+1)
		divisor, remainder := slotL/svrL, slotL%svrL
		if remainder >= (i + 1) {
			divisor++
		}
		s.sizes[id] = divisor
	}
}

// remainAndFree remain and free slots
func (s *SlotHash) remainAndFree() {
	for pos, id := range *s.slots {
		slots := s.slotses[id]
		if slots != nil && len(slots) < s.sizes[id] {
			// remain consistent slot
			s.slotses[id] = append(slots, pos)
		} else {
			// free slot for lost and redundant server
			(*s.slots)[pos] = ""
		}
	}
}

// allocate allocates slots to lacks
func (s *SlotHash) allocate() {
	pos, slotL := 0, len(*s.slots)
	for _, svr := range s.servers {
		id := svr.ID()
		slots := s.slotses[id]
		for i := len(slots); i < s.sizes[id]; i++ {
			for ; ; pos++ {
				if pos >= slotL {
					break
				}
				if (*s.slots)[pos] == "" {
					(*s.slots)[pos] = id
					slots = append(slots, pos)
					pos++
					break
				}
			}
		}
		svrSlots := make(map[int]bool, len(slots))
		for _, slot := range slots {
			svrSlots[slot] = true
		}
		svr.slots = svrSlots
	}
}

// hash distributes slots using consistent hash for servers
func (s *SlotHash) hash() {
	s.calucate()
	s.remainAndFree()
	s.allocate()
}

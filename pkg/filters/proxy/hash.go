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
	slots    *[]string
	servers  []*Server
	slotses  map[string][]int
	sizes    map[string]int
	recycles []int
}

// size counts size for total and group with pos
func size(total, group, pos int) int {
	size := total / group
	if pos <= total%group {
		size++
	}
	return size
}

// init inits the slots map and counts size for servers
func (s *SlotHash) init() bool {
	if len(s.servers) == 0 {
		return false
	}

	if *s.slots == nil {
		*s.slots = make([]string, SlotSize)
	}
	l := len(s.servers)
	s.slotses = make(map[string][]int, l)
	s.sizes = make(map[string]int, l)
	for i, svr := range s.servers {
		id := svr.ID()
		s.slotses[id] = make([]int, 0)
		s.sizes[id] = size(SlotSize, l, i+1)
	}
	return true
}

// recycle recycles slots for empty, lost and overflow
func (s *SlotHash) recycle() bool {
	for pos, id := range *s.slots {
		if id == "" || s.slotses[id] == nil || len(s.slotses[id]) >= s.sizes[id] {
			s.recycles = append(s.recycles, pos)
		} else {
			s.slotses[id] = append(s.slotses[id], pos)
		}
	}
	return len(s.recycles) > 0
}

// fillup fills up slots for lacking servers
func (s *SlotHash) fillup() {
	for _, svr := range s.servers {
		id := svr.ID()
		slots := s.slotses[id]
		fill := s.sizes[id] - len(slots)
		if fill == 0 {
			continue
		}

		// fill slots from recycles
		for i := 0; i < fill; i++ {
			slots = append(slots, s.recycles[i])
			(*s.slots)[s.recycles[i]] = id
		}
		s.recycles = s.recycles[fill:]

		// update slots for servers
		svrSlots := make(map[int]bool, len(slots))
		for _, slot := range slots {
			svrSlots[slot] = true
		}
		svr.slots = svrSlots
	}
}

// hash distributes slots using consistent hash for servers
func (s *SlotHash) hash() {
	if !s.init() {
		return
	}
	if !s.recycle() {
		return
	}
	s.fillup()
}

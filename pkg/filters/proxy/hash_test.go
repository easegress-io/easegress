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

func assertEven(t *testing.T, svrs []*Server, slots []string) {
	assert.Greater(t, len(slots), 0)

	counter := make(map[string]int, len(svrs))
	for _, svr := range svrs {
		counter[svr.ID()] = 0
	}
	for _, id := range slots {
		counter[id]++
	}
	for _, c := range counter {
		assert.GreaterOrEqual(t, c, len(slots)/len(svrs))
	}
}

func assertConsistentAfterRemove(t *testing.T, preSvrs []*Server, preSlots []string, oldSvrs []*Server, slots []string) {
	assert.Greater(t, len(preSlots), 0)
	assert.Greater(t, len(slots), 0)

	oldIds := make(map[string]bool)
	for _, svr := range oldSvrs {
		oldIds[svr.ID()] = true
	}
	for i, id := range preSlots {
		if !oldIds[id] {
			assert.Equal(t, slots[i], id)
		}
	}
}

func assertConsistentAfterAdd(t *testing.T, preSvrs []*Server, preSlots []string, newSvrs []*Server, slots []string) {
	assert.Greater(t, len(preSlots), 0)
	assert.Greater(t, len(slots), 0)

	counter := 0
	for i, id := range preSlots {
		if id == slots[i] {
			counter++
		}
	}
	svrL, newSvrL, slotL := len(preSvrs), len(newSvrs), len(preSlots)
	assert.GreaterOrEqual(t, counter, slotL/(svrL+newSvrL)*svrL)
}

func TestSlotHash(t *testing.T) {
	sp := NewServerPool(&Proxy{}, &ServerPoolSpec{}, "test")
	svrs := make([]*Server, 0, 3)
	for i := 0; i < 3; i++ {
		svrs = append(svrs, &Server{URL: fmt.Sprintf("192.168.1.%d", i+1)})
	}
	slotHash := &SlotHash{slots: &sp.slots, servers: svrs}
	slotHash.hash()
	assertEven(t, svrs, sp.slots)

	preSvrs := svrs
	preSlots := sp.slots
	newSvrs := make([]*Server, 0)
	newSvrs = append(newSvrs, &Server{URL: fmt.Sprintf("192.168.1.%d", 4)})
	for _, svr := range newSvrs {
		svrs = append(svrs, svr)
	}
	slotHash = &SlotHash{slots: &sp.slots, servers: svrs}
	slotHash.hash()
	assertEven(t, svrs, sp.slots)
	assertConsistentAfterAdd(t, preSvrs, preSlots, newSvrs, sp.slots)

	preSvrs = svrs
	preSlots = sp.slots
	newSvrs = make([]*Server, 0)
	newSvrs = append(newSvrs, &Server{URL: fmt.Sprintf("192.168.1.%d", 5)})
	newSvrs = append(newSvrs, &Server{URL: fmt.Sprintf("192.168.1.%d", 6)})
	for _, svr := range newSvrs {
		svrs = append(svrs, svr)
	}
	slotHash = &SlotHash{slots: &sp.slots, servers: svrs}
	slotHash.hash()
	assertEven(t, svrs, sp.slots)
	assertConsistentAfterAdd(t, preSvrs, preSlots, newSvrs, sp.slots)

	preSvrs = svrs
	preSlots = sp.slots
	oldSvrs := svrs[len(svrs)-1:]
	svrs = svrs[:len(svrs)-1]
	slotHash = &SlotHash{slots: &sp.slots, servers: svrs}
	slotHash.hash()
	assertEven(t, svrs, sp.slots)
	assertConsistentAfterRemove(t, preSvrs, preSlots, oldSvrs, sp.slots)

	preSvrs = svrs
	preSlots = sp.slots
	oldSvrs = svrs[len(svrs)-2:]
	svrs = svrs[:len(svrs)-2]
	slotHash = &SlotHash{slots: &sp.slots, servers: svrs}
	slotHash.hash()
	assertEven(t, svrs, sp.slots)
	assertConsistentAfterRemove(t, preSvrs, preSlots, oldSvrs, sp.slots)
}

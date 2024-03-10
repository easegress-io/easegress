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

package mqttproxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func finalEqual(t *testing.T, expect any, got func() any) {
	for i := 0; i < 20; i++ {
		if expect == got() {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	assert.Equal(t, expect, got())
}

func TestSessionCacheManager(t *testing.T) {
	EGName := "eg1"
	OtherEGName := "eg2"

	spec := &Spec{
		TopicCacheSize: 1000,
		EGName:         EGName,
	}
	topicMgr := newTopicManager(spec)
	sessCacheMgr := newSessionCacheManager(spec, topicMgr)
	sessCacheMgr.sync(map[string]*SessionInfo{
		"client1": {
			Topics:   map[string]int{"topic1": 1},
			EGName:   EGName,
			ClientID: "client1",
		},
		"client3": {
			Topics:   map[string]int{"topic1": 1},
			EGName:   EGName,
			ClientID: "client3",
		},
		"client2": {
			Topics:   map[string]int{"topic1": 0},
			EGName:   OtherEGName,
			ClientID: "client2",
		},
	})
	finalEqual(t, "eg1", func() any { return sessCacheMgr.getEGName("client1") })
	finalEqual(t, "eg2", func() any { return sessCacheMgr.getEGName("client2") })

	sessCacheMgr.update(map[string]*SessionInfo{
		"client2": {
			Topics:   map[string]int{"topic2": 1},
			EGName:   OtherEGName,
			ClientID: "client2",
		},
	})
	finalEqual(t, byte(1), func() any {
		subscribers, err := topicMgr.findSubscribers("topic2")
		assert.Nil(t, err)
		return subscribers["client2"]
	})

	// after delete, getEGName should return its own name for client2
	sessCacheMgr.delete("client2")
	finalEqual(t, EGName, func() any { return sessCacheMgr.getEGName("client2") })

	sessCacheMgr.sync(map[string]*SessionInfo{
		"client2": {
			Topics:   map[string]int{"topic1": 0},
			EGName:   OtherEGName,
			ClientID: "client2",
		},
	})
	finalEqual(t, OtherEGName, func() any { return sessCacheMgr.getEGName("client2") })
}

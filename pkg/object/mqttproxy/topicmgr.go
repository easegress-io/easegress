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
	"fmt"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

// TopicManager is the interface of a topic manager.
type TopicManager interface {
	subscribe(topics []string, qoss []byte, clientID string) error
	unsubscribe(topics []string, clientID string) error
	disconnect(topics []string, clientID string) error
	findSubscribers(topic string) (map[string]byte, error)
	close()
}

func newTopicManager(spec *Spec) TopicManager {
	if spec.BrokerMode {
		return newCachedTopicManager(spec.TopicCacheSize)
	}
	return newNoCacheTopicManager(spec.TopicCacheSize)
}

// levelTopicManager to manage topic subscribe and unsubscribe in MQTT
type levelTopicManager struct {
	sync.RWMutex
	root *topicNode
}

func newLevelTopicManager() *levelTopicManager {
	return &levelTopicManager{
		root: newNode(),
	}
}

func (mgr *levelTopicManager) subscribe(allLevels [][]string, qoss []byte, clientID string) {
	mgr.Lock()
	defer mgr.Unlock()

	for i, l := range allLevels {
		mgr.insert(l, qoss[i], clientID)
	}
}

func (mgr *levelTopicManager) unsubscribe(allLevels [][]string, clientID string) {
	mgr.Lock()
	defer mgr.Unlock()

	for _, l := range allLevels {
		mgr.remove(l, clientID)
	}
}

// findSubscribers is used to find all clients that subscribe a certain topic directly or use wildcard.
// for example, topic "loc/device/event" will find clients that subscribe topic "+/+/+" or "loc/+/event" or "loc/device/event"
// so, clients subscribe topics that contain or not contain wildcard, and this function will find all subscribed topics that match
// the given topic.
func (mgr *levelTopicManager) findSubscribers(levels []string) map[string]byte {
	mgr.RLock()
	defer mgr.RUnlock()

	output := make(map[string]byte)

	currentLevelNodes := []*topicNode{mgr.root}
	for _, topicLevel := range levels {
		nextLevelNodes := []*topicNode{}
		for _, node := range currentLevelNodes {
			for nodeLevel, nextNode := range node.nodes {
				if nodeLevel == "#" {
					nextNode.addClients(output)

				} else if nodeLevel == "+" || nodeLevel == topicLevel {
					nextLevelNodes = append(nextLevelNodes, nextNode)
				}
			}
		}
		currentLevelNodes = nextLevelNodes
		if len(currentLevelNodes) == 0 {
			return output
		}
	}
	for _, n := range currentLevelNodes {
		n.addClients(output)
		// in MQTT version 3.1.1 section 4.7.1.2, topic "sport/tennis/player1/#" would receive msg from "sport/tennis/player1"
		// which means when we reach end of topic level, we need check one more level for wildcard #
		if val, ok := n.nodes["#"]; ok {
			val.addClients(output)
		}
	}
	return output
}

func (mgr *levelTopicManager) insert(levels []string, qos byte, clientID string) {
	node := mgr.root
	var nextNode *topicNode
	var ok bool
	for _, l := range levels {
		nextNode, ok = node.nodes[l]
		if !ok {
			nextNode = newNode()
			node.nodes[l] = nextNode
		}
		node = nextNode
	}
	node.clients[clientID] = qos
}

func (mgr *levelTopicManager) remove(levels []string, clientID string) {
	node := mgr.root
	var nextNode *topicNode
	var ok bool
	prevNodes := []*topicNode{}
	for _, l := range levels {
		if nextNode, ok = node.nodes[l]; !ok {
			return
		}
		prevNodes = append(prevNodes, node)
		node = nextNode
	}
	delete(node.clients, clientID)

	// clear memory
	for i := len(prevNodes) - 1; i >= 0; i-- {
		node = prevNodes[i].nodes[levels[i]]
		if len(node.clients) == 0 && len(node.nodes) == 0 {
			delete(prevNodes[i].nodes, levels[i])
		} else {
			return
		}
	}
}

type topicNode struct {
	// client with their qos
	clients map[string]byte
	nodes   map[string]*topicNode
}

func newNode() *topicNode {
	return &topicNode{
		clients: make(map[string]byte),
		nodes:   make(map[string]*topicNode),
	}
}

func (node *topicNode) addClients(ans map[string]byte) {
	for client, qos := range node.clients {
		ans[client] = qos
	}
}

type topicLevelCache struct {
	data *lru.Cache
}

func newTopicLevelCache(cacheSize int) *topicLevelCache {
	// here we promise cacheSize greater than 0, so we ignore this error.
	cache, _ := lru.New(cacheSize)
	return &topicLevelCache{
		data: cache,
	}
}

func (t *topicLevelCache) get(topic string) ([]string, error) {
	if val, ok := t.data.Get(topic); ok {
		return val.([]string), nil
	}
	levels, valid := splitTopic(topic)
	if valid {
		t.data.Add(topic, levels)
		return levels, nil
	}
	return nil, fmt.Errorf("topic %v is invalid", topic)
}

func (t *topicLevelCache) getAll(topics []string) ([][]string, error) {
	res := make([][]string, len(topics))
	for i, topic := range topics {
		levels, err := t.get(topic)
		if err != nil {
			return nil, err
		}
		res[i] = levels
	}
	return res, nil
}

func splitTopic(topic string) ([]string, bool) {
	levels := make([]string, strings.Count(topic, "/")+1)
	levelsLoc := 0

	levelStart := 0
	wildCardFlag := false
	for i, char := range topic {
		if char == '/' {
			level := topic[levelStart:i]
			if len(level) > 1 && wildCardFlag {
				return nil, false
			}
			levels[levelsLoc] = level
			levelsLoc++
			levelStart = i + 1
			wildCardFlag = false

		} else if char == '+' {
			wildCardFlag = true

		} else if char == '#' {
			wildCardFlag = true
			if i != len(topic)-1 {
				return nil, false
			}
		}
	}

	level := topic[levelStart:]
	if len(level) > 1 && wildCardFlag {
		return nil, false
	}
	levels[levelsLoc] = level
	return levels, true
}

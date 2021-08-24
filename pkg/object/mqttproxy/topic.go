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

package mqttproxy

import (
	"fmt"
	"strings"
	"sync"
)

type (
	// TopicManager use topic to find corresponding
	TopicManager struct {
		sync.RWMutex

		// map topic (string) to subscribers (client ids)
		subscriptions map[string]map[string]struct{}
	}

	topicMapFunc func(mqttTopic string) (topic string, headers map[string]string, err error)
)

func newTopicManager() *TopicManager {
	t := &TopicManager{
		subscriptions: make(map[string]map[string]struct{}),
	}
	return t
}

func (t *TopicManager) subscribe(topics []string, clientID string) {
	t.Lock()
	defer t.Unlock()
	for _, topic := range topics {
		if val, ok := t.subscriptions[topic]; ok {
			val[clientID] = struct{}{}
		} else {
			t.subscriptions[topic] = make(map[string]struct{})
			t.subscriptions[topic][clientID] = struct{}{}
		}
	}
}

func (t *TopicManager) unsubscribe(topics []string, clientID string) {
	t.Lock()
	defer t.Unlock()
	for _, topic := range topics {
		if val, ok := t.subscriptions[topic]; ok {
			delete(val, clientID)
			if len(val) == 0 {
				delete(t.subscriptions, topic)
			}
		}
	}
}

func (t *TopicManager) findSubscribers(topic string) map[string]struct{} {
	t.RLock()
	defer t.RUnlock()

	if val, ok := t.subscriptions[topic]; ok {
		return val
	}
	return nil
}

func getTopicMapFunc(topicMapper *TopicMapper) topicMapFunc {
	if topicMapper == nil {
		return nil
	}
	idx := topicMapper.TopicIndex

	f := func(mqttTopic string) (string, map[string]string, error) {
		levels := strings.Split(mqttTopic, "/")
		if levels[0] == "" {
			levels = levels[1:]
		}
		if len(levels) <= idx {
			return "", nil, fmt.Errorf("levels in mqtt topic <%s> is less than topic index <%d>", mqttTopic, idx)
		}

		topic := levels[idx]
		headers := make(map[string]string)
		for k, v := range topicMapper.Headers {
			if k >= len(levels) {
				continue
			}
			headers[v] = levels[k]
		}
		return topic, headers, nil
	}
	return f
}

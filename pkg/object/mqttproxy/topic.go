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

	"github.com/megaease/easegress/pkg/object/function/storage"
	etcderror "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"gopkg.in/yaml.v2"
)

type (
	// TopicManager use topic to find corresponding
	TopicManager struct {
		store storage.Storage
	}

	Topic struct {
		Set map[string]struct{} `yaml:"set"`
	}

	topicMapFunc func(mqttTopic string) (topic string, headers map[string]string, err error)
)

func newTopicManager(store storage.Storage) *TopicManager {
	t := &TopicManager{
		store: store,
	}
	return t
}

func (t *TopicManager) subscribe(topics []string, clientID string) error {
	err := t.store.Lock()
	if err != nil {
		return err
	}
	defer func() {
		err = t.store.Unlock()
	}()

	for _, topic := range topics {
		key := topicStoreKey(topic)
		value, err := t.store.Get(key)
		if err != nil && err != etcderror.ErrKeyNotFound {
			return err
		}

		data := Topic{}
		if value == nil {
			data.Set = make(map[string]struct{})
		} else {
			err = yaml.Unmarshal([]byte(*value), &data)
			if err != nil {
				return err
			}
		}
		data.Set[clientID] = struct{}{}
		bs, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		err = t.store.Put(key, string(bs))
		if err != nil {
			return err
		}
	}
	return err
}

func (t *TopicManager) unsubscribe(topics []string, clientID string) error {
	err := t.store.Lock()
	if err != nil {
		return err
	}
	defer func() {
		err = t.store.Unlock()
	}()

	for _, topic := range topics {
		key := topicStoreKey(topic)
		value, err := t.store.Get(key)
		if err != nil && err != etcderror.ErrKeyNotFound {
			return err
		}

		data := Topic{}
		if value == nil {
			data.Set = make(map[string]struct{})
		} else {
			err = yaml.Unmarshal([]byte(*value), &data)
			if err != nil {
				return err
			}
		}
		delete(data.Set, clientID)
		bs, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		err = t.store.Put(key, string(bs))
		if err != nil {
			return err
		}
	}
	return err
}

func (t *TopicManager) findSubscribers(topic string) (map[string]struct{}, error) {
	err := t.store.Lock()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = t.store.Unlock()
	}()

	key := topicStoreKey(topic)
	value, err := t.store.Get(key)
	if err != nil {
		if err == etcderror.ErrKeyNotFound {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}

	data := Topic{}
	err = yaml.Unmarshal([]byte(*value), &data)
	if err != nil {
		return nil, err
	}
	return data.Set, nil
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

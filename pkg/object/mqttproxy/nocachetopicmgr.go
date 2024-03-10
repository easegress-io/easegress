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

type noCacheTopicManager struct {
	topicMgr   *levelTopicManager
	topicCache *topicLevelCache
}

var _ TopicManager = (*noCacheTopicManager)(nil)

func newNoCacheTopicManager(cacheSize int) *noCacheTopicManager {
	return &noCacheTopicManager{
		topicMgr:   newLevelTopicManager(),
		topicCache: newTopicLevelCache(cacheSize),
	}
}

func (mgr *noCacheTopicManager) findSubscribers(topic string) (map[string]byte, error) {
	levels, err := mgr.topicCache.get(topic)
	if err != nil {
		return nil, err
	}
	return mgr.topicMgr.findSubscribers(levels), nil
}

func (mgr *noCacheTopicManager) subscribe(topics []string, qoss []byte, clientID string) error {
	allLevels, err := mgr.topicCache.getAll(topics)
	if err != nil {
		return err
	}
	mgr.topicMgr.subscribe(allLevels, qoss, clientID)
	return nil
}

func (mgr *noCacheTopicManager) unsubscribe(topics []string, clientID string) error {
	allLevels, err := mgr.topicCache.getAll(topics)
	if err != nil {
		return err
	}
	mgr.topicMgr.unsubscribe(allLevels, clientID)
	return nil
}

func (mgr *noCacheTopicManager) disconnect(topics []string, clientID string) error {
	return mgr.unsubscribe(topics, clientID)
}

func (mgr *noCacheTopicManager) close() {
}

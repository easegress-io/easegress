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

import "sync"

type cachedTopicManager struct {
	levelCache *topicLevelCache
	topicMgr   *levelTopicManager

	// topicMapper map[topic]map[clientID]qos
	// sync.Map with value of sync.Map
	topicMapper sync.Map

	// clientMapper map[clientID]map[topic]qos
	// sync.Map with value of sync.Map
	clientMapper sync.Map

	writeCh chan *topicOp
	closeCh chan struct{}
}

var _ TopicManager = (*cachedTopicManager)(nil)

func newCachedTopicManager(cacheSize int) *cachedTopicManager {
	mgr := &cachedTopicManager{
		topicMgr:   newLevelTopicManager(),
		levelCache: newTopicLevelCache(cacheSize),
		writeCh:    make(chan *topicOp, 10000),
		closeCh:    make(chan struct{}),
	}
	go mgr.update()
	return mgr
}

func (mgr *cachedTopicManager) findSubscribers(topic string) (map[string]byte, error) {
	if value, ok := mgr.topicMapper.Load(topic); ok {
		res := make(map[string]byte)
		value.(*sync.Map).Range(func(key any, value any) bool {
			res[key.(string)] = value.(byte)
			return true
		})
		return res, nil
	}
	levels, err := mgr.levelCache.get(topic)
	if err != nil {
		return nil, err
	}
	mgr.writeCh <- &topicOp{
		opType: publish,
		publishOp: &publishOp{
			topic:  topic,
			levels: levels,
		},
	}
	return mgr.topicMgr.findSubscribers(levels), nil
}

func (mgr *cachedTopicManager) subscribe(topics []string, qoss []byte, clientID string) error {
	allLevels, err := mgr.levelCache.getAll(topics)
	if err != nil {
		return err
	}

	mgr.writeCh <- &topicOp{
		opType: subscribe,
		subscribeOp: &subscribeOp{
			allLevels: allLevels,
			qoss:      qoss,
			clientID:  clientID,
		},
	}
	return nil
}

func (mgr *cachedTopicManager) unsubscribe(topics []string, clientID string) error {
	allLevels, err := mgr.levelCache.getAll(topics)
	if err != nil {
		return err
	}

	mgr.writeCh <- &topicOp{
		opType: unsubscribe,
		unsubscribeOp: &unsubscribeOp{
			allLevels: allLevels,
			clientID:  clientID,
		},
	}
	return nil
}

func (mgr *cachedTopicManager) disconnect(topics []string, clientID string) error {
	allLevels, err := mgr.levelCache.getAll(topics)
	if err != nil {
		return err
	}
	mgr.writeCh <- &topicOp{
		opType: disconnect,
		disconnectOp: &disconnectOp{
			allLevels: allLevels,
			clientID:  clientID,
		},
	}
	return nil
}

func (mgr *cachedTopicManager) processSubscribe(op *subscribeOp) {
	mgr.topicMgr.subscribe(op.allLevels, op.qoss, op.clientID)
	matchTopics := []string{}

	mgr.topicMapper.Range(func(key any, value any) bool {
		topicLevels, _ := mgr.levelCache.get(key.(string))
		for i, levels := range op.allLevels {
			if isTopicMatch(topicLevels, levels) {
				matchTopics = append(matchTopics, key.(string))
				value.(*sync.Map).Store(op.clientID, op.qoss[i])
			}
		}
		return true
	})
	clientMap, _ := mgr.clientMapper.LoadOrStore(op.clientID, &sync.Map{})
	for _, topic := range matchTopics {
		clientMap.(*sync.Map).Store(topic, op.qoss)
	}
}

func (mgr *cachedTopicManager) processUnsubscribe(op *unsubscribeOp) {
	mgr.topicMgr.unsubscribe(op.allLevels, op.clientID)
	value, ok := mgr.clientMapper.Load(op.clientID)
	if !ok {
		return
	}

	matchTopics := []string{}
	value.(*sync.Map).Range(func(key any, value any) bool {
		topicLevels, _ := mgr.levelCache.get(key.(string))
		for _, levels := range op.allLevels {
			if isTopicMatch(topicLevels, levels) {
				matchTopics = append(matchTopics, key.(string))
			}
		}
		return true
	})
	for _, topic := range matchTopics {
		value.(*sync.Map).Delete(topic)
		topicMap, ok := mgr.topicMapper.Load(topic)
		if ok {
			topicMap.(*sync.Map).Delete(op.clientID)
		}
	}
}

func (mgr *cachedTopicManager) processPublish(op *publishOp) {
	clients := mgr.topicMgr.findSubscribers(op.levels)
	topicMap, _ := mgr.topicMapper.LoadOrStore(op.topic, &sync.Map{})

	for cid, qos := range clients {
		topicMap.(*sync.Map).Store(cid, qos)
		clientMap, _ := mgr.clientMapper.LoadOrStore(cid, &sync.Map{})
		clientMap.(*sync.Map).Store(op.topic, qos)
	}
}

func (mgr *cachedTopicManager) processDisconnect(op *disconnectOp) {
	mgr.topicMgr.unsubscribe(op.allLevels, op.clientID)
	value, ok := mgr.clientMapper.Load(op.clientID)
	if !ok {
		return
	}
	value.(*sync.Map).Range(func(key any, value any) bool {
		topicMap, ok := mgr.topicMapper.Load(key.(string))
		if ok {
			topicMap.(*sync.Map).Delete(op.clientID)
		}
		return true
	})
	mgr.clientMapper.Delete(op.clientID)
}

func (mgr *cachedTopicManager) update() {
	for {
		select {
		case op := <-mgr.writeCh:
			switch op.opType {
			case subscribe:
				mgr.processSubscribe(op.subscribeOp)
			case unsubscribe:
				mgr.processUnsubscribe(op.unsubscribeOp)
			case publish:
				mgr.processPublish(op.publishOp)
			case disconnect:
				mgr.processDisconnect(op.disconnectOp)
			}
		case <-mgr.closeCh:
			return
		}
	}
}

func (mgr *cachedTopicManager) close() {
	close(mgr.closeCh)
}

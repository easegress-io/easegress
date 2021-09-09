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
	"regexp"
	"strings"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
)

type topicMapFunc func(mqttTopic string) (topic string, headers map[string]string, err error)

type TopicManager struct {
	sync.RWMutex
	root *topicNode
}

func newTopicManager() *TopicManager {
	return &TopicManager{
		root: newNode(),
	}
}

func validTopic(topic string) bool {
	levels := strings.Split(topic, "/")
	for i, level := range levels {
		if strings.ContainsRune(level, '+') && len(level) != 1 {
			return false
		}
		if strings.ContainsRune(level, '#') {
			if len(level) != 1 || i != len(levels)-1 {
				return false
			}
		}
	}
	return true
}

func (mgr *TopicManager) subscribe(topics []string, qoss []byte, clientID string) error {
	mgr.Lock()
	defer mgr.Unlock()

	for i, t := range topics {
		if !validTopic(t) {
			return fmt.Errorf("topic not valid, topic:%s", t)
		}
		levels := strings.Split(t, "/")
		if err := mgr.root.insert(levels, qoss[i], clientID); err != nil {
			return err
		}
	}
	return nil
}

func (mgr *TopicManager) unsubscribe(topics []string, clientID string) error {
	mgr.Lock()
	defer mgr.Unlock()

	for _, t := range topics {
		if !validTopic(t) {
			return fmt.Errorf("topic not valid, topic:%s", t)
		}
		levels := strings.Split(t, "/")
		if err := mgr.root.remove(levels, clientID); err != nil {
			return err
		}
	}
	return nil
}

func (mgr *TopicManager) findSubscribers(topic string) (map[string]byte, error) {
	mgr.RLock()
	defer mgr.RUnlock()
	if !validTopic(topic) {
		return nil, fmt.Errorf("topic not valid, topic:%s", topic)
	}
	levels := strings.Split(topic, "/")
	ans := make(map[string]byte)
	mgr.root.match(levels, ans)
	return ans, nil
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

func (node *topicNode) insert(levels []string, qos byte, clientID string) error {
	if len(levels) == 0 {
		node.clients[clientID] = qos
		return nil
	}

	nextNode, ok := node.nodes[levels[0]]
	if !ok {
		nextNode = newNode()
		node.nodes[levels[0]] = nextNode
	}
	return nextNode.insert(levels[1:], qos, clientID)
}

func (node *topicNode) remove(levels []string, clientID string) error {
	if len(levels) == 0 {
		delete(node.clients, clientID)
		return nil
	}

	nextNode, ok := node.nodes[levels[0]]
	if !ok {
		return nil
	}
	if err := nextNode.remove(levels[1:], clientID); err != nil {
		return err
	}
	if len(nextNode.clients) == 0 && len(nextNode.nodes) == 0 {
		delete(node.nodes, levels[0])
	}
	return nil
}

func (node *topicNode) match(levels []string, ans map[string]byte) {
	if len(levels) == 0 {
		node.addClients(ans)
		if n, ok := node.nodes["#"]; ok {
			n.addClients(ans)
		}
		return
	}

	for l, nextNode := range node.nodes {
		if l == "#" {
			nextNode.addClients(ans)
		}
		if l == "+" || l == levels[0] {
			nextNode.match(levels[1:], ans)
		}
	}
}

func (node *topicNode) addClients(ans map[string]byte) {
	for client, qos := range node.clients {
		ans[client] = qos
	}
}

func getPolicyRoute(routes []*PolicyRe) map[string]*regexp.Regexp {
	ans := make(map[string]*regexp.Regexp)
	for _, route := range routes {
		r, err := regexp.Compile(route.MatchExpr)
		if err != nil {
			logger.Errorf("topicMapper policy <%s> match expr <%s> compile failed, err:%v", route.Name, route.MatchExpr, err)
		} else {
			ans[route.Name] = r
		}
	}
	return ans
}

type topicRouteType map[string][]*regexp.Regexp

func getTopicRoute(p *Policy) topicRouteType {
	m := make(map[string][]*regexp.Regexp)
	for _, route := range p.Route {
		for _, expr := range route.Exprs {
			r, err := regexp.Compile(expr)
			if err != nil {
				logger.Errorf("topicMapper policy <%s> topic route expr <%s> compile failed, err:%v", p.Name, expr, err)
			} else {
				m[route.Topic] = append(m[route.Topic], r)
			}
		}
	}
	return m
}

func getPolicyMap(ps []*Policy) map[string]*Policy {
	ans := make(map[string]*Policy)
	for _, p := range ps {
		ans[p.Name] = p
	}
	return ans
}

func getTopicMapFunc(topicMapper *TopicMapper) topicMapFunc {
	if topicMapper == nil {
		return nil
	}
	policyMap := getPolicyMap(topicMapper.Policies)
	policyRoute := getPolicyRoute(topicMapper.Route)
	topicRoutes := make(map[string]topicRouteType)
	for _, p := range topicMapper.Policies {
		topicRoutes[p.Name] = getTopicRoute(p)
	}

	getPolicy := func(level string) *Policy {
		for name, r := range policyRoute {
			if r.MatchString(level) {
				return policyMap[name]
			}
		}
		return nil
	}

	getTopic := func(level string, p *Policy) (string, error) {
		topicRoute := topicRoutes[p.Name]
		for _, route := range p.Route {
			regexps := topicRoute[route.Topic]
			for _, regexp := range regexps {
				if regexp.MatchString(level) {
					return route.Topic, nil
				}
			}
		}
		return "", fmt.Errorf("no match topic for level <%s> with policy <%s>", level, p.Name)
	}

	f := func(mqttTopic string) (string, map[string]string, error) {
		levels := strings.Split(mqttTopic, "/")
		if levels[0] == "" {
			levels = levels[1:]
		}
		if len(levels) <= topicMapper.MatchIndex {
			return "", nil, fmt.Errorf("levels in mqtt topic <%s> is less than policy match index <%d>", mqttTopic, topicMapper.MatchIndex)
		}
		p := getPolicy(levels[topicMapper.MatchIndex])
		if p == nil {
			return "", nil, fmt.Errorf("no policy match mqtt topic <%s>", mqttTopic)
		}
		if len(levels) <= p.TopicIndex {
			return "", nil, fmt.Errorf("levels in mqtt topic <%s> is less then policy <%s> topic index <%d>", mqttTopic, p.Name, p.TopicIndex)
		}
		topic, err := getTopic(levels[p.TopicIndex], p)
		if err != nil {
			return "", nil, fmt.Errorf("mqttTopic %s get backend massage queue topic failed, err:%v", mqttTopic, err)
		}
		headers := make(map[string]string)
		for k, v := range p.Headers {
			if k >= len(levels) {
				continue
			}
			headers[v] = levels[k]
		}
		return topic, headers, nil
	}
	return f
}

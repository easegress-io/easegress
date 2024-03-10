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

package topicmapper

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/megaease/easegress/v2/pkg/logger"
)

type topicMapFunc func(mqttTopic string) (topic string, headers map[string]string, err error)

type policyRe struct {
	name string
	re   *regexp.Regexp
}

func getPolicyRoute(routes []*PolicyRe) []*policyRe {
	res := make([]*policyRe, 0, len(routes))
	for _, route := range routes {
		r, err := regexp.Compile(route.MatchExpr)
		if err != nil {
			logger.SpanErrorf(nil, "topicMapper policy <%s> match expr <%s> compile failed: %v", route.Name, route.MatchExpr, err)
		} else {
			res = append(res, &policyRe{name: route.Name, re: r})
		}
	}
	return res
}

type topicRouteType map[string][]*regexp.Regexp

func getTopicRoute(p *Policy) topicRouteType {
	m := make(map[string][]*regexp.Regexp)
	for _, route := range p.Route {
		for _, expr := range route.Exprs {
			r, err := regexp.Compile(expr)
			if err != nil {
				logger.SpanErrorf(nil, "topicMapper policy <%s> topic route expr <%s> compile failed: %v", p.Name, expr, err)
			} else {
				m[route.Topic] = append(m[route.Topic], r)
			}
		}
	}
	return m
}

func getPolicyMap(ps []*Policy) map[string]*Policy {
	res := make(map[string]*Policy)
	for _, p := range ps {
		res[p.Name] = p
	}
	return res
}

func getTopicMapFunc(topicMapper *Spec) topicMapFunc {
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
		for _, route := range policyRoute {
			if route.re.MatchString(level) {
				return policyMap[route.name]
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
			return "", nil, fmt.Errorf("mqttTopic %s get backend massage queue topic failed: %v", mqttTopic, err)
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

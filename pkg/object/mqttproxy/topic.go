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

type topicOpType int

const (
	subscribe topicOpType = iota
	unsubscribe
	publish
	disconnect
)

type topicOp struct {
	opType        topicOpType
	subscribeOp   *subscribeOp
	unsubscribeOp *unsubscribeOp
	publishOp     *publishOp
	disconnectOp  *disconnectOp
}

type subscribeOp struct {
	allLevels [][]string
	qoss      []byte
	clientID  string
}

type unsubscribeOp struct {
	allLevels [][]string
	clientID  string
}

type publishOp struct {
	topic  string
	levels []string
}

type disconnectOp struct {
	allLevels [][]string
	clientID  string
}

func isTopicMatch(topicLevels []string, subLevels []string) bool {
	if len(topicLevels) < len(subLevels) {
		return false
	}
	if (len(topicLevels) > len(subLevels)) && (subLevels[len(subLevels)-1] != "#") {
		return false
	}
	for i, subLevel := range subLevels {
		if subLevel == "#" {
			return true
		}
		if subLevel == "+" {
			continue
		}
		if subLevel != topicLevels[i] {
			return false
		}
	}
	return true
}

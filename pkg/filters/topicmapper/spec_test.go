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
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func TestSpec(t *testing.T) {
	topicYaml := `
matchIndex: 0
route:
  - name: g2s
    matchExpr: g2s
  - name: d2s
    matchExpr: d2s
policies:
  - name: d2s
    topicIndex: 1
    route:
      - topic: to_cloud
        exprs: ["bar", "foo"]
      - topic: to_raw
        exprs: [".*"]
    headers:
      0: d2s
      1: type
      2: device
  - name: g2s
    topicIndex: 4
    route:
      - topic: to_cloud
        exprs: ["bar", "foo"]
      - topic: to_raw
        exprs: [".*"]
    headers:
      0: g2s
      1: gateway
      2: info
      3: device
      4: type
`
	got := Spec{}
	err := codectool.Unmarshal([]byte(topicYaml), &got)
	assert.Nil(t, err)

	want := Spec{
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"g2s", "g2s"},
			{"d2s", "d2s"},
		},
		Policies: []*Policy{
			{
				Name:       "d2s",
				TopicIndex: 1,
				Route: []TopicRe{
					{"to_cloud", []string{"bar", "foo"}},
					{"to_raw", []string{".*"}},
				},
				Headers: map[int]string{0: "d2s", 1: "type", 2: "device"},
			},
			{
				Name:       "g2s",
				TopicIndex: 4,
				Route: []TopicRe{
					{"to_cloud", []string{"bar", "foo"}},
					{"to_raw", []string{".*"}},
				},
				Headers: map[int]string{0: "g2s", 1: "gateway", 2: "info", 3: "device", 4: "type"},
			},
		},
	}
	assert.Equal(t, want, got)
}

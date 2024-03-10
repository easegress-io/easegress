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

import "github.com/megaease/easegress/v2/pkg/filters"

type (
	// Spec is spec of Kafka
	Spec struct {
		filters.BaseSpec `json:",inline"`

		MatchIndex int         `json:"matchIndex" jsonschema:"required"`
		Route      []*PolicyRe `json:"route" jsonschema:"required"`
		Policies   []*Policy   `json:"policies" jsonschema:"required"`
		SetKV      *SetKV      `json:"setKV" jsonschema:"required"`
	}

	// SetKV set topic mapper result to MQTT context kv map
	SetKV struct {
		Topic   string `json:"topic" jsonschema:"topic"`
		Headers string `json:"headers" jsonschema:"headers"`
	}

	// PolicyRe to match right policy to do topic map
	PolicyRe struct {
		Name      string `json:"name" jsonschema:"required"`
		MatchExpr string `json:"matchExpr" jsonschema:"required"`
	}

	// Policy describes topic map between MQTT topic and Backend MQ topic
	Policy struct {
		Name       string         `json:"name" jsonschema:"required"`
		TopicIndex int            `json:"topicIndex" jsonschema:"required"`
		Route      []TopicRe      `json:"route" jsonschema:"required"`
		Headers    map[int]string `json:"headers" jsonschema:"required"`
	}

	// TopicRe to match right topic in given policy
	TopicRe struct {
		Topic string   `json:"topic" jsonschema:"required"`
		Exprs []string `json:"exprs" jsonschema:"required"`
	}
)

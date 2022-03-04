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

package topicmapper

type (
	// Spec is spec of Kafka
	Spec struct {
		MatchIndex int         `yaml:"matchIndex" jsonschema:"required"`
		Route      []*PolicyRe `yaml:"route" jsonschema:"required"`
		Policies   []*Policy   `yaml:"policies" jsonschema:"required"`
		SetKV      *SetKV      `yaml:"setKV" jsonschema:"required"`
	}

	// SetKV set topic mapper result to MQTT context kv map
	SetKV struct {
		Topic   string `yaml:"topic" jsonschema:"topic"`
		Headers string `yaml:"headers" jsonschema:"headers"`
	}

	// PolicyRe to match right policy to do topic map
	PolicyRe struct {
		Name      string `yaml:"name" jsonschema:"required"`
		MatchExpr string `yaml:"matchExpr" jsonschema:"required"`
	}

	// Policy describes topic map between MQTT topic and Backend MQ topic
	Policy struct {
		Name       string         `yaml:"name" jsonschema:"required"`
		TopicIndex int            `yaml:"topicIndex" jsonschema:"required"`
		Route      []TopicRe      `yaml:"route" jsonschema:"required"`
		Headers    map[int]string `yaml:"headers" jsonschema:"required"`
	}

	// TopicRe to match right topic in given policy
	TopicRe struct {
		Topic string   `yaml:"topic" jsonschema:"required"`
		Exprs []string `yaml:"exprs" jsonschema:"required"`
	}
)

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

package kafka

import "github.com/megaease/easegress/pkg/filters"

type (
	// Spec is spec of Kafka
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Backend []string `yaml:"backend" jsonschema:"required,uniqueItems=true"`
		Topic   *Topic   `yaml:"topic" jsonschema:"required"`
		KVMap   *KVMap   `yaml:"mqtt" jsonschema:"required"`
	}

	// Topic defined ways to get Kafka topic
	Topic struct {
		Default string `yaml:"default" jsonschema:"required"`
	}

	// KVMap defines ways to get kafka data from MQTTContext kv map
	KVMap struct {
		TopicKey   string `yaml:"topicKey" jsonschema:"required"`
		HeaderKey  string `yaml:"headerKey" jsonschema:"required"`
		PayloadKey string `yaml:"payloadKey" jsonschema:"required"`
	}
)

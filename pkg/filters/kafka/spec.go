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

package kafka

import "github.com/megaease/easegress/v2/pkg/filters"

type (
	// Spec is spec of Kafka
	Spec struct {
		filters.BaseSpec `json:",inline"`

		Backend []string `json:"backend" jsonschema:"required,uniqueItems=true"`
		Topic   *Topic   `json:"topic" jsonschema:"required"`
		KVMap   *KVMap   `json:"mqtt" jsonschema:"required"`
	}

	// Topic defined ways to get Kafka topic
	Topic struct {
		Default string `json:"default" jsonschema:"required"`
	}

	// KVMap defines ways to get kafka data from MQTTContext kv map
	KVMap struct {
		TopicKey   string `json:"topicKey" jsonschema:"required"`
		HeaderKey  string `json:"headerKey" jsonschema:"required"`
		PayloadKey string `json:"payloadKey" jsonschema:"required"`
	}
)

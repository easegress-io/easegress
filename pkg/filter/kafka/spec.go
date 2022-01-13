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

type (
	// Spec is spec of Kafka
	Spec struct {
		Backend []string `yaml:"backend" jsonschema:"required,uniqueItems=true"`
		KVMap   *KVMap   `yaml:"mqtt" jsonschema:"required"`
	}

	// KVMap defines ways to get kafka data from MQTTContext kv map
	KVMap struct {
		Topic   string `yaml:"topic" jsonschema:"required"`
		Headers string `yaml:"headers" jsonschema:"required"`
		Payload string `yaml:"payload" jsonschema:"required"`
	}
)

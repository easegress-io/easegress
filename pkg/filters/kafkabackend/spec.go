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
		filters.BaseSpec `json:",inline"`

		Backend []string `json:"backend" jsonschema:"required,uniqueItems=true"`
		Topic   *Topic   `json:"topic" jsonschema:"required"`
	}

	// Topic defined ways to get Kafka topic
	Topic struct {
		Default string   `json:"default" jsonschema:"required"`
		Dynamic *Dynamic `json:"dynamic" jsonschema:"omitempty"`
	}

	// Dynamic defines dynamic ways to get Kafka topic from http request
	Dynamic struct {
		Header string `json:"header" jsonschema:"omitempty"`
	}
)

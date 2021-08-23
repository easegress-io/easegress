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

type (
	// Spec describes the MQTTProxy.
	Spec struct {
		Name        string       `yaml:"-"`
		Port        uint16       `yaml:"port" jsonschema:"required"`
		BackendType string       `yaml:"backendType" jsonschema:"required"`
		Auth        []Auth       `yaml:"auth" jsonschema:"required"`
		TopicMapper *TopicMapper `yaml:"topicMapper" jsonschema:"omitempty"`
		Kafka       *KafkaSpec   `yaml:"kafkaBroker" jsonschema:"omitempty"`
	}

	// Auth describes username and password for MQTTProxy
	Auth struct {
		Username  string `yaml:"userName" jsonschema:"required"`
		B64Passwd string `yaml:"passBase64" jsonschema:"required"`
	}

	// TopicMapper describes topic map between MQTT topic and Backend MQ topic
	TopicMapper struct {
		TopicIndex int            `yaml:"topicIndex" jsonschema:"required"`
		Headers    map[int]string `yaml:"headers" jsonschema:"required"`
	}

	// KafkaSpec describes Kafka producer
	KafkaSpec struct {
		Backend []string `yaml:"backend" jsonschema:"required,uniqueItems=true"`
	}
)

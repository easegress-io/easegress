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

import (
	"crypto/tls"
	"fmt"
)

const (
	sessionPrefix = "/mqtt/sessionMgr/clientID/%s"
	topicPrefix   = "/mqtt/topicMgr/topic/%s"
	mqttAPIPrefix = "/mqttproxy/%s/topics/publish"
)

type (
	// Spec describes the MQTTProxy.
	Spec struct {
		EGName         string        `yaml:"-"`
		Name           string        `yaml:"-"`
		Port           uint16        `yaml:"port" jsonschema:"required"`
		BackendType    string        `yaml:"backendType" jsonschema:"required"`
		Auth           []Auth        `yaml:"auth" jsonschema:"required"`
		TopicMapper    *TopicMapper  `yaml:"topicMapper" jsonschema:"omitempty"`
		Kafka          *KafkaSpec    `yaml:"kafkaBroker" jsonschema:"omitempty"`
		UseTLS         bool          `yaml:"useTLS" jsonschema:"omitempty"`
		Certificate    []Certificate `yaml:"certificate" jsonschema:"omitempty"`
		TopicCacheSize int           `yaml:"topicCacheSize" jsonschema:"omitempty"`
	}

	// Certificate describes TLS certifications.
	Certificate struct {
		Name string `yaml:"name" jsonschema:"required"`
		Cert string `yaml:"cert" jsonschema:"required"`
		Key  string `yaml:"key" jsonschema:"required"`
	}

	// Auth describes username and password for MQTTProxy
	Auth struct {
		UserName   string `yaml:"userName" jsonschema:"required"`
		PassBase64 string `yaml:"passBase64" jsonschema:"required"`
	}
	// TopicMapper map MQTT multi-level topic to Kafka topic with headers
	TopicMapper struct {
		MatchIndex int         `yaml:"matchIndex" jsonschema:"required"`
		Route      []*PolicyRe `yaml:"route" jsonschema:"required"`
		Policies   []*Policy   `yaml:"policies" jsonschema:"required"`
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

	// KafkaSpec describes Kafka producer
	KafkaSpec struct {
		Backend []string `yaml:"backend" jsonschema:"required,uniqueItems=true"`
	}
)

func (spec *Spec) tlsConfig() (*tls.Config, error) {
	var certificates []tls.Certificate

	for _, c := range spec.Certificate {
		cert, err := tls.X509KeyPair([]byte(c.Cert), []byte(c.Key))
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair for %s failed: %s ", c.Name, err)
		}
		certificates = append(certificates, cert)
	}
	if len(certificates) == 0 {
		return nil, fmt.Errorf("none valid certs and secret")
	}

	return &tls.Config{Certificates: certificates}, nil
}

func sessionStoreKey(clientID string) string {
	return fmt.Sprintf(sessionPrefix, clientID)
}

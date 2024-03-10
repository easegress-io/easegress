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

import (
	"crypto/tls"
	"fmt"
)

const (
	sessionPrefix              = "/mqtt/sessionMgr/clientID/%s"
	topicPrefix                = "/mqtt/topicMgr/topic/%s"
	mqttAPITopicPublishPrefix  = "/mqttproxy/%s/topics/publish"
	mqttAPISessionQueryPrefix  = "/mqttproxy/%s/session/query"
	mqttAPISessionDeletePrefix = "/mqttproxy/%s/sessions"
)

// PacketType is mqtt packet type
type PacketType string

const (
	// Connect is connect type of MQTT packet
	Connect PacketType = "Connect"

	// Disconnect is disconnect type of MQTT packet
	Disconnect PacketType = "Disconnect"

	// Publish is publish type of MQTT packet
	Publish PacketType = "Publish"

	// Subscribe is subscribe type of MQTT packet
	Subscribe PacketType = "Subscribe"

	// Unsubscribe is unsubscribe type of MQTT packet
	Unsubscribe PacketType = "Unsubscribe"
)

var pipelinePacketTypes = map[PacketType]struct{}{
	Connect:     {},
	Disconnect:  {},
	Publish:     {},
	Subscribe:   {},
	Unsubscribe: {},
}

type (
	// Spec describes the MQTTProxy.
	Spec struct {
		EGName               string        `json:"-"`
		Name                 string        `json:"-"`
		Port                 uint16        `json:"port" jsonschema:"required"`
		UseTLS               bool          `json:"useTLS,omitempty"`
		Certificate          []Certificate `json:"certificate,omitempty"`
		TopicCacheSize       int           `json:"topicCacheSize,omitempty"`
		MaxAllowedConnection int           `json:"maxAllowedConnection,omitempty"`
		ConnectionLimit      *RateLimit    `json:"connectionLimit,omitempty"`
		ClientPublishLimit   *RateLimit    `json:"clientPublishLimit,omitempty"`
		Rules                []*Rule       `json:"rules,omitempty"`
		BrokerMode           bool          `json:"brokerMode,omitempty"`
		// unit is second, default is 30s
		RetryInterval int `yaml:"retryInterval,omitempty"`
	}

	// Rule used to route MQTT packets to different pipelines
	Rule struct {
		When     *When  `json:"when,omitempty"`
		Pipeline string `json:"pipeline,omitempty"`
	}

	// When is used to check if MQTT packet match this pipeline
	When struct {
		PacketType PacketType `json:"packetType,omitempty"`
	}

	// RateLimit describes rate limit for connection or publish.
	// requestRate: max allowed request in time period
	// timePeriod: max allowed bytes in time period
	// timePeriod: time of seconds to count requestRate and bytesRate, default 1 second
	RateLimit struct {
		RequestRate int `json:"requestRate,omitempty"`
		BytesRate   int `json:"bytesRate,omitempty"`
		TimePeriod  int `json:"timePeriod,omitempty"`
	}

	// Certificate describes TLS certifications.
	Certificate struct {
		Name string `json:"name" jsonschema:"required"`
		Cert string `json:"cert" jsonschema:"required"`
		Key  string `json:"key" jsonschema:"required"`
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

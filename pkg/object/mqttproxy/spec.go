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
		EGName               string        `yaml:"-"`
		Name                 string        `yaml:"-"`
		Port                 uint16        `yaml:"port" jsonschema:"required"`
		UseTLS               bool          `yaml:"useTLS" jsonschema:"omitempty"`
		Certificate          []Certificate `yaml:"certificate" jsonschema:"omitempty"`
		TopicCacheSize       int           `yaml:"topicCacheSize" jsonschema:"omitempty"`
		MaxAllowedConnection int           `yaml:"maxAllowedConnection" jsonschema:"omitempty"`
		ConnectionLimit      *RateLimit    `yaml:"connectionLimit" jsonschema:"omitempty"`
		ClientPublishLimit   *RateLimit    `yaml:"clientPublishLimit" jsonschema:"omitempty"`
		Rules                []*Rule       `yaml:"rules" jsonschema:"omitempty"`
	}

	// Rule used to route MQTT packets to different pipelines
	Rule struct {
		When     *When  `yaml:"when" jsonschema:"omitempty"`
		Pipeline string `yaml:"pipeline" jsonschema:"omitempty"`
	}

	// When is used to check if MQTT packet match this pipeline
	When struct {
		PacketType PacketType `yaml:"packetType" jsonschema:"omitempty"`
	}

	// RateLimit describes rate limit for connection or publish.
	// requestRate: max allowed request in time period
	// timePeriod: max allowed bytes in time period
	// timePeriod: time of seconds to count requestRate and bytesRate, default 1 second
	RateLimit struct {
		RequestRate int `yaml:"requestRate" jsonschema:"omitempty"`
		BytesRate   int `yaml:"bytesRate" jsonschema:"omitempty"`
		TimePeriod  int `yaml:"timePeriod" jsonschema:"omitempty"`
	}

	// Certificate describes TLS certifications.
	Certificate struct {
		Name string `yaml:"name" jsonschema:"required"`
		Cert string `yaml:"cert" jsonschema:"required"`
		Key  string `yaml:"key" jsonschema:"required"`
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

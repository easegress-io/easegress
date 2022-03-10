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

package mqttclientauth

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of MQTTClientAuth
	Kind = "MQTTClientAuth"

	resultAuthFail = "AuthFail"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Authentication can check MQTT client's username and password",
	Results:     []string{resultAuthFail},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func() filters.Filter {
		return &MQTTClientAuth{}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// MQTTClientAuth is used to check authentication for MQTT client
	MQTTClientAuth struct {
		spec    *Spec
		authMap map[string]string
		salt    string
	}

	// Spec is spec for MQTTClientAuth.
	// For security of password, passwords in yaml file should be salted SHA256 checksum.
	// password = sha256sum(connect.password + salt)
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Salt string  `yaml:"salt" jsonschema:"omitempty"`
		Auth []*Auth `yaml:"auth" jsonschema:"required"`
	}

	// Auth describes username and password for MQTTProxy
	Auth struct {
		Username         string `yaml:"username" jsonschema:"required"`
		SaltedSha256Pass string `yaml:"saltedSha256Pass" jsonschema:"required"`
	}
)

var _ filters.Filter = (*MQTTClientAuth)(nil)
var _ pipeline.MQTTFilter = (*MQTTClientAuth)(nil)

// Name returns the name of the MQTTClientAuth filter instance.
func (a *MQTTClientAuth) Name() string {
	return a.spec.Name()
}

// Kind return kind of MQTTClientAuth
func (a *MQTTClientAuth) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the MQTTClientAuth
func (a *MQTTClientAuth) Spec() filters.Spec {
	return a.spec
}

// Init init MQTTClientAuth
func (a *MQTTClientAuth) Init(spec filters.Spec) {
	if spec.Protocol() != context.MQTT {
		panic("filter ConnectControl only support MQTT protocol for now")
	}
	a.spec = spec.(*Spec)
	a.salt = a.spec.Salt
	a.authMap = make(map[string]string)

	for _, auth := range a.spec.Auth {
		a.authMap[auth.Username] = auth.SaltedSha256Pass
	}

	if len(a.authMap) == 0 {
		logger.Errorf("empty valid authentication for MQTT filter %v", spec.Name())
	}
}

// Inherit init MQTTClientAuth based on previous generation
func (a *MQTTClientAuth) Inherit(spec filters.Spec, previousGeneration filters.Filter) {
	previousGeneration.Close()
	a.Init(spec)
}

// Close close MQTTClientAuth
func (a *MQTTClientAuth) Close() {
}

// Status return status of MQTTClientAuth
func (a *MQTTClientAuth) Status() interface{} {
	return nil
}

func sha256Sum(data []byte) string {
	sha256Bytes := sha256.Sum256(data)
	return hex.EncodeToString(sha256Bytes[:])
}

func (a *MQTTClientAuth) checkAuth(connect *packets.ConnectPacket) string {
	if connect.ClientIdentifier == "" {
		return resultAuthFail
	}
	saltedSha256Pass, ok := a.authMap[connect.Username]
	if !ok {
		return resultAuthFail
	}
	if saltedSha256Pass != sha256Sum(append(connect.Password, []byte(a.salt)...)) {
		return resultAuthFail
	}
	return ""
}

// HandleMQTT handle MQTT context
func (a *MQTTClientAuth) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTConnect {
		return &context.MQTTResult{}
	}
	result := a.checkAuth(ctx.ConnectPacket())
	if result != "" {
		ctx.SetDisconnect()
		return &context.MQTTResult{ErrString: resultAuthFail}
	}
	return &context.MQTTResult{ErrString: ""}
}

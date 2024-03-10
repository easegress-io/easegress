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

// Package mqttclientauth implements authentication for MQTT clients.
package mqttclientauth

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
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
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &MQTTClientAuth{spec: spec.(*Spec)}
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
	// For security of password, passwords in json file should be salted SHA256 checksum.
	// password = sha256sum(connect.password + salt)
	Spec struct {
		filters.BaseSpec `json:",inline"`

		Salt string  `json:"salt,omitempty"`
		Auth []*Auth `json:"auth" jsonschema:"required"`
	}

	// Auth describes username and password for MQTTProxy
	Auth struct {
		Username         string `json:"username" jsonschema:"required"`
		SaltedSha256Pass string `json:"saltedSha256Pass" jsonschema:"required"`
	}
)

var _ filters.Filter = (*MQTTClientAuth)(nil)

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
func (a *MQTTClientAuth) Init() {
	a.salt = a.spec.Salt
	a.authMap = make(map[string]string)

	for _, auth := range a.spec.Auth {
		a.authMap[auth.Username] = auth.SaltedSha256Pass
	}

	if len(a.authMap) == 0 {
		logger.Errorf("empty valid authentication for MQTT filter %v", a.spec.Name())
	}
}

// Inherit init MQTTClientAuth based on previous generation
func (a *MQTTClientAuth) Inherit(previousGeneration filters.Filter) {
	a.Init()
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

// Handle handles context.
func (a *MQTTClientAuth) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*mqttprot.Request)
	resp := ctx.GetOutputResponse().(*mqttprot.Response)
	if req.PacketType() != mqttprot.ConnectType {
		return ""
	}
	result := a.checkAuth(req.ConnectPacket())
	if result != "" {
		resp.SetDisconnect()
		return resultAuthFail
	}
	return ""
}

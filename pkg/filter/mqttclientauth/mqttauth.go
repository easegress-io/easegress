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
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of MQTTClientAuth
	Kind = "MQTTClientAuth"

	resultAuthFail = "AuthFail"
)

var errAuthFail = errors.New(resultAuthFail)

func init() {
	pipeline.Register(&MQTTClientAuth{})
}

type (
	// MQTTClientAuth is used to check authentication for MQTT client
	MQTTClientAuth struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
		authMap    map[string]string
	}

	// Spec is spec for MQTTClientAuth
	Spec struct {
		Auth []Auth `yaml:"auth" jsonschema:"required"`
	}

	// Auth describes username and password for MQTTProxy
	// passSha256 make sure customer's password is safe.
	Auth struct {
		UserName   string `yaml:"userName" jsonschema:"required"`
		PassBase64 string `yaml:"passBase64" jsonschema:"required"`
	}
)

var _ pipeline.Filter = (*MQTTClientAuth)(nil)
var _ pipeline.MQTTFilter = (*MQTTClientAuth)(nil)

// Kind return kind of MQTTClientAuth
func (a *MQTTClientAuth) Kind() string {
	return Kind
}

// DefaultSpec return default spec of MQTTClientAuth
func (a *MQTTClientAuth) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of MQTTClientAuth
func (a *MQTTClientAuth) Description() string {
	return "Authentication can check MQTT client's username and password"
}

// Results return possible results of MQTTClientAuth
func (a *MQTTClientAuth) Results() []string {
	return []string{resultAuthFail}
}

// Init init MQTTClientAuth
func (a *MQTTClientAuth) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter ConnectControl only support MQTT protocol for now")
	}
	a.filterSpec, a.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	a.authMap = make(map[string]string)

	for _, auth := range a.spec.Auth {
		passwd, err := base64.StdEncoding.DecodeString(auth.PassBase64)
		if err != nil {
			logger.Errorf("auth with name %v, base64 password %v decode failed: %v", auth.UserName, auth.PassBase64, err)
			continue
		}
		a.authMap[auth.UserName] = sha256Sum(passwd)
	}
	if len(a.authMap) == 0 {
		logger.Errorf("empty valid authentication for MQTT filter %v", filterSpec.Name())
	}
}

// Inherit init MQTTClientAuth based on previous generation
func (a *MQTTClientAuth) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	a.Init(filterSpec)
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

func (a *MQTTClientAuth) checkAuth(connect *packets.ConnectPacket) error {
	if connect.ClientIdentifier == "" {
		return errAuthFail
	}
	pass, ok := a.authMap[connect.Username]
	if !ok {
		return errAuthFail
	}
	if pass != sha256Sum(connect.Password) {
		return errAuthFail
	}
	return nil
}

// HandleMQTT handle MQTT context
func (a *MQTTClientAuth) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTConnect {
		return &context.MQTTResult{}
	}
	err := a.checkAuth(ctx.ConnectPacket())
	if err != nil {
		ctx.SetDisconnect()
		return &context.MQTTResult{Err: errAuthFail}
	}
	return &context.MQTTResult{}
}
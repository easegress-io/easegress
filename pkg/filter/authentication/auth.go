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

package authentication

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
	// Kind is the kind of Authentication
	Kind = "Authentication"

	resultAuthFail = "AuthFail"
)

var errAuthFail = errors.New(resultAuthFail)

func init() {
	pipeline.Register(&Authentication{})
}

type (
	// Authentication is used to check authentication for MQTT client
	Authentication struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
		authMap    map[string]string
	}

	// Spec is spec for Authentication
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

var _ pipeline.Filter = (*Authentication)(nil)
var _ pipeline.MQTTFilter = (*Authentication)(nil)

// Kind return kind of Authentication
func (a *Authentication) Kind() string {
	return Kind
}

// DefaultSpec return default spec of Authentication
func (a *Authentication) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of Authentication
func (a *Authentication) Description() string {
	return "Authentication can check MQTT client's username and password"
}

// Results return possible results of Authentication
func (a *Authentication) Results() []string {
	return []string{resultAuthFail}
}

// Init init Authentication
func (a *Authentication) Init(filterSpec *pipeline.FilterSpec) {
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

// Inherit init Authentication based on previous generation
func (k *Authentication) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	k.Init(filterSpec)
}

// Close close Authentication
func (a *Authentication) Close() {
}

// Status return status of Authentication
func (a *Authentication) Status() interface{} {
	return nil
}

func sha256Sum(data []byte) string {
	sha256Bytes := sha256.Sum256(data)
	return hex.EncodeToString(sha256Bytes[:])
}

func (a *Authentication) checkAuth(connect *packets.ConnectPacket) error {
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
func (a *Authentication) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
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

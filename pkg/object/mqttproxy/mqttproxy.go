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

import "github.com/megaease/easegress/pkg/supervisor"

const (
	// Category is the category of MQTTProxy.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of MQTTProxy.
	Kind = "MQTTProxy"
)

func init() {
	supervisor.Register(&MQTTProxy{})
}

type (
	// MQTTProxy implements MQTT proxy in EG
	MQTTProxy struct {
		superSpec *supervisor.Spec
		spec      *Spec
		broker    *Broker
	}
)

// Category returns the category of MQTTProxy.
func (mp *MQTTProxy) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of MQTTProxy.
func (mp *MQTTProxy) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of MQTTProxy.
func (mp *MQTTProxy) DefaultSpec() interface{} {
	return &Spec{}
}

func (mp *MQTTProxy) Status() *supervisor.Status {
	return &supervisor.Status{}
}

// Init initializes Function.
func (mp *MQTTProxy) Init(superSpec *supervisor.Spec) {
	spec := superSpec.ObjectSpec().(*Spec)
	spec.Name = superSpec.Name()
	mp.superSpec, mp.spec = superSpec, spec
	mp.broker = newBroker(spec)
}

// Inherit inherits previous generation of WebSocketServer.
func (mp *MQTTProxy) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	mp.Init(superSpec)
}

// Close closes MQTTProxy.
func (mp *MQTTProxy) Close() {
	mp.broker.close()
}

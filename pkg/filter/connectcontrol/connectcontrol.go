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

package connectcontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of ConnectControl
	Kind = "ConnectControl"

	resultWrongPacket         = "wrongPacketError"
	resultBannedClientOrTopic = "bannedClientOrTopicError"
)

func init() {
	pipeline.Register(&ConnectControl{})
}

type (
	// ConnectControl is used to control MQTT clients connect status,
	// if MQTTContext ClientID in bannedClients, the connection will be closed,
	// if MQTTContext publish topic in bannedTopics, the connection will be closed.
	ConnectControl struct {
		filterSpec    *pipeline.FilterSpec
		spec          *Spec
		bannedClients sync.Map
		bannedTopics  sync.Map
	}

	// Spec describes the ConnectControl
	Spec struct {
		BannedClients []string `yaml:"bannedClients" jsonschema:"omitempty"`
		BannedTopics  []string `yaml:"bannedTopics" jsonschema:"omitempty"`
		EarlyStop     bool     `yaml:"earlyStop" jsonschema:"omitempty"`
	}

	Status struct {
	}

	HTTPJsonData struct {
		Clients []string `yaml:"clients" jsonschema:"omitempty"`
		Topics  []string `yaml:"topics" jsonschema:"omitempty"`
	}
)

var _ pipeline.Filter = (*ConnectControl)(nil)
var _ pipeline.MQTTFilter = (*ConnectControl)(nil)

// Kind return kind of ConnectControl
func (cc *ConnectControl) Kind() string {
	return Kind
}

// DefaultSpec return default spec of ConnectControl
func (cc *ConnectControl) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of ConnectControl
func (cc *ConnectControl) Description() string {
	return "ConnectControl control connections of MQTT clients"
}

// Results return results of ConnectControl
func (cc *ConnectControl) Results() []string {
	return nil
}

// Init init ConnectControl with pipeline filter spec
func (cc *ConnectControl) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter ConnectControl only support MQTT protocol for now")
	}
	cc.filterSpec, cc.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	cc.reload()
}

func (cc *ConnectControl) reload() {
	for _, c := range cc.spec.BannedClients {
		cc.bannedClients.Store(c, struct{}{})
	}
	for _, t := range cc.spec.BannedTopics {
		cc.bannedTopics.Store(t, struct{}{})
	}
}

// Inherit init ConnectControl with previous generation
func (cc *ConnectControl) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	cc.Init(filterSpec)
}

// Status return status of ConnectControl
func (cc *ConnectControl) Status() interface{} {
	return nil
}

// Close close ConnectControl gracefully
func (cc *ConnectControl) Close() {
}

// HandleMQTT handle MQTT request
func (cc *ConnectControl) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTPublish {
		return &context.MQTTResult{Err: errors.New(resultWrongPacket)}
	}

	cid := ctx.Client().ClientID()
	if _, ok := cc.bannedClients.Load(cid); !ok {
		topic := ctx.PublishPacket().TopicName
		if _, ok := cc.bannedTopics.Load(topic); !ok {
			return &context.MQTTResult{}
		}
	}

	ctx.SetDisconnect()
	if cc.spec.EarlyStop {
		ctx.SetEarlyStop()
	}
	return &context.MQTTResult{Err: errors.New(resultBannedClientOrTopic)}
}

func (cc *ConnectControl) APIs() []*pipeline.APIEntry {
	addName := func(path string) string {
		return fmt.Sprintf(path, cc.filterSpec.Name())
	}
	entries := []*pipeline.APIEntry{
		{Path: addName("connectcontrol/%s/banclient"), Method: http.MethodPost, Handler: cc.handleBanClient},
		{Path: addName("connectcontrol/%s/unbanclient"), Method: http.MethodPost, Handler: cc.handleUnbanClient},
		{Path: addName("connectcontrol/%s/info"), Method: http.MethodGet, Handler: cc.handleInfo},
	}
	return entries
}

func (cc *ConnectControl) handleBanClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose POST request but got %s", r.Method))
		return
	}
	var data HTTPJsonData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("invalid json data from request body"))
		return
	}
	for _, c := range data.Clients {
		cc.bannedClients.Store(c, struct{}{})
	}
	for _, t := range data.Topics {
		cc.bannedTopics.Store(t, struct{}{})
	}
}

func (cc *ConnectControl) handleUnbanClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose POST request but got %s", r.Method))
		return
	}
	var data HTTPJsonData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("invalid json data from request body"))
		return
	}
	for _, c := range data.Clients {
		cc.bannedClients.Delete(c)
	}
	for _, t := range data.Topics {
		cc.bannedTopics.Delete(t)
	}
}

func (cc *ConnectControl) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose POST request but got %s", r.Method))
		return
	}
	clients := []string{}
	cc.bannedClients.Range(func(key, value interface{}) bool {
		clients = append(clients, key.(string))
		return true
	})

	topics := []string{}
	cc.bannedTopics.Range(func(key, value interface{}) bool {
		topics = append(topics, key.(string))
		return true
	})

	data := HTTPJsonData{
		Clients: clients,
		Topics:  topics,
	}
	err := json.NewEncoder(w).Encode(&data)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("encode json data failed"))
		return
	}
}

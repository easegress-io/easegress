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
	stdcontext "context"
	"fmt"
	"sync"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func newContext(cid, username, password string) context.MQTTContext {
	client := &context.MockMQTTClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	packet.ClientIdentifier = cid
	packet.Username = username
	packet.Password = []byte(password)
	return context.NewMQTTContext(stdcontext.Background(), client, packet)
}

func defaultFilterSpec(spec *Spec) *pipeline.FilterSpec {
	meta := &pipeline.FilterMetaSpec{
		Name:     "connect-demo",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
		Protocol: context.MQTT,
	}
	filterSpec := pipeline.MockFilterSpec(nil, nil, "", meta, spec)
	return filterSpec
}

func TestAuth(t *testing.T) {
	assert := assert.New(t)
	spec := &Spec{}
	filterSpec := defaultFilterSpec(spec)
	auth := &MQTTClientAuth{}
	auth.Init(filterSpec)

	assert.Equal(Kind, auth.Kind())
	assert.Equal(&Spec{}, auth.DefaultSpec())
	assert.NotEmpty(auth.Description())
	assert.Equal(1, len(auth.Results()), "please update this case if add more results")
	assert.Nil(auth.Status(), "please update this case if return status")

	newAuth := &MQTTClientAuth{}
	newAuth.Inherit(filterSpec, auth)
	newAuth.Close()
}

func TestAuthFile(t *testing.T) {
	assert := assert.New(t)
	salt := "abcfdlfkjaslfkalfjslfskfjslf"
	spec := &Spec{
		Salt: salt,
		Auth: []*Auth{
			{Username: "test", SaltedSha256Pass: sha256Sum([]byte("test" + salt))},
			{Username: "admin", SaltedSha256Pass: sha256Sum([]byte("admin" + salt))},
		},
	}

	filterSpec := defaultFilterSpec(spec)
	auth := &MQTTClientAuth{}
	auth.Init(filterSpec)

	type testCase struct {
		cid        string
		name       string
		pass       string
		disconnect bool
	}
	tests := []testCase{
		{"client1", "test", "test", false},
		{"client2", "admin", "admin", false},
		{"client3", "fake", "test", true},
		{"client4", "test", "wrongPass", true},
		{"", "test", "test", true},
	}

	var wg sync.WaitGroup
	for _, test := range tests {
		wg.Add(1)
		go func(test testCase) {
			ctx := newContext(test.cid, test.name, test.pass)
			auth.HandleMQTT(ctx)
			assert.Equal(test.disconnect, ctx.Disconnect(), fmt.Errorf("test case %+v got wrong result", test))
			wg.Done()
		}(test)
	}
	wg.Wait()
}

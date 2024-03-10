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

package mqttclientauth

import (
	"fmt"
	"sync"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func newContext(cid, username, password string) *context.Context {
	ctx := context.New(nil)

	client := &mqttprot.MockClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	packet.ClientIdentifier = cid
	packet.Username = username
	packet.Password = []byte(password)

	req := mqttprot.NewRequest(packet, client)
	ctx.SetInputRequest(req)

	resp := mqttprot.NewResponse()
	ctx.SetOutputResponse(resp)

	return ctx
}

func TestAuth(t *testing.T) {
	assert := assert.New(t)
	spec := &Spec{}
	auth := kind.CreateInstance(spec)
	auth.Init()

	assert.Equal(Kind, auth.Kind().Name)
	assert.Equal(1, len(kind.Results), "please update this case if add more results")
	assert.Nil(auth.Status(), "please update this case if return status")

	newAuth := kind.CreateInstance(spec)
	newAuth.Inherit(auth)
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

	auth := kind.CreateInstance(spec)
	auth.Init()

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
			auth.Handle(ctx)
			resp := ctx.GetOutputResponse().(*mqttprot.Response)
			assert.Equal(test.disconnect, resp.Disconnect(), fmt.Errorf("test case %+v got wrong result", test))
			wg.Done()
		}(test)
	}
	wg.Wait()
}

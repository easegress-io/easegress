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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func base64Encode(text string) string {
	return base64.StdEncoding.EncodeToString([]byte(text))
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
	fileStr := `
- username: test
  passBase64: %s
- username: admin
  passBase64: %s
`
	fileStr = fmt.Sprintf(fileStr, base64Encode("test"), base64Encode("admin"))

	tmpFile, err := ioutil.TempFile(os.TempDir(), "prefix-")
	require.Nil(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte(fileStr))
	require.Nil(t, err)

	err = tmpFile.Close()
	require.Nil(t, err)

	assert := assert.New(t)
	spec := &Spec{
		AuthFile: tmpFile.Name(),
	}

	filterSpec := defaultFilterSpec(spec)
	auth := &MQTTClientAuth{}
	auth.Init(filterSpec)

	tests := []struct {
		cid        string
		name       string
		pass       string
		disconnect bool
	}{
		{"client1", "test", "test", false},
		{"client2", "admin", "admin", false},
		{"client3", "fake", "test", true},
		{"client4", "test", "wrongPass", true},
		{"", "test", "test", true},
	}

	for _, test := range tests {
		ctx := newContext(test.cid, test.name, test.pass)
		auth.HandleMQTT(ctx)
		assert.Equal(test.disconnect, ctx.Disconnect(), fmt.Errorf("test case %+v got wrong result", test))
	}
}

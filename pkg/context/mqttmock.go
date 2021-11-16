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

package context

import "sync"

// MockMQTTClient is mock client for MQTTContext
type MockMQTTClient struct {
	MockClientID string
	MockUserName string
	MockKVMap    sync.Map
}

var _ MQTTClient = (*MockMQTTClient)(nil)

// ClientID return client id of MockMQTTClient
func (m *MockMQTTClient) ClientID() string {
	return m.MockClientID
}

// UserName return username if MockMQTTClient
func (m *MockMQTTClient) UserName() string {
	return m.MockUserName
}

// Load load value keep in MockMQTTClient kv map
func (m *MockMQTTClient) Load(key interface{}) (value interface{}, ok bool) {
	return m.MockKVMap.Load(key)
}

// Store store kv pair into MockMQTTClient kv map
func (m *MockMQTTClient) Store(key interface{}, value interface{}) {
	m.MockKVMap.Store(key, value)
}

// Delete delete key-value pair in MockMQTTClient kv map
func (m *MockMQTTClient) Delete(key interface{}) {
	m.MockKVMap.Delete(key)
}

type MockMQTTMsg struct {
	Target  string
	Data    []byte
	Headers map[string]string
}

type MockMQTTBackend struct {
	Messages map[string]MockMQTTMsg
}

var _ MQTTBackend = (*MockMQTTBackend)(nil)

func (m *MockMQTTBackend) Publish(target string, data []byte, headers map[string]string) error {
	m.Messages[target] = MockMQTTMsg{
		Target:  target,
		Data:    data,
		Headers: headers,
	}
	return nil
}

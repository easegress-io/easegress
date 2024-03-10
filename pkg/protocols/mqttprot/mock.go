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

package mqttprot

import "sync"

// MockClient is mock client for MQTT protocol
type MockClient struct {
	MockClientID string
	MockUserName string
	MockKVMap    sync.Map
}

var _ Client = (*MockClient)(nil)

// ClientID return client id of MockClient
func (m *MockClient) ClientID() string {
	return m.MockClientID
}

// UserName return username if MockClient
func (m *MockClient) UserName() string {
	return m.MockUserName
}

// Load load value keep in MockClient kv map
func (m *MockClient) Load(key interface{}) (value interface{}, ok bool) {
	return m.MockKVMap.Load(key)
}

// Store store kv pair into MockClient kv map
func (m *MockClient) Store(key interface{}, value interface{}) {
	m.MockKVMap.Store(key, value)
}

// Delete delete key-value pair in MockClient kv map
func (m *MockClient) Delete(key interface{}) {
	m.MockKVMap.Delete(key)
}

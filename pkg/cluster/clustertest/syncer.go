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

package clustertest

import "go.etcd.io/etcd/api/v3/mvccpb"

// MockedSyncer is a mock for cluster.Syncer
type MockedSyncer struct {
	MockedSync          func(string) (<-chan *string, error)
	MockedSyncRaw       func(string) (<-chan *mvccpb.KeyValue, error)
	MockedSyncPrefix    func(string) (<-chan map[string]string, error)
	MockedSyncRawPrefix func(string) (<-chan map[string]*mvccpb.KeyValue, error)
	MockedClose         func()
}

// NewMockedSyncer return MockedSyncer
func NewMockedSyncer() *MockedSyncer {
	return &MockedSyncer{}
}

// Sync implements interface function Sync
func (s *MockedSyncer) Sync(key string) (<-chan *string, error) {
	if s.MockedSync != nil {
		return s.MockedSync(key)
	}
	return nil, nil
}

// SyncRaw implements interface function SyncRaw
func (s *MockedSyncer) SyncRaw(key string) (<-chan *mvccpb.KeyValue, error) {
	if s.MockedSyncRaw != nil {
		return s.MockedSyncRaw(key)
	}
	return nil, nil
}

// SyncPrefix implements interface function SyncPrefix
func (s *MockedSyncer) SyncPrefix(prefix string) (<-chan map[string]string, error) {
	if s.MockedSyncPrefix != nil {
		return s.MockedSyncPrefix(prefix)
	}
	return nil, nil
}

// SyncRawPrefix implements interface function SyncRawPrefix
func (s *MockedSyncer) SyncRawPrefix(prefix string) (<-chan map[string]*mvccpb.KeyValue, error) {
	if s.MockedSyncRawPrefix != nil {
		return s.MockedSyncRawPrefix(prefix)
	}
	return nil, nil
}

// Close implements interface function Close
func (s *MockedSyncer) Close() {}

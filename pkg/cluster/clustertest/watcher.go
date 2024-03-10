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

import (
	"github.com/megaease/easegress/v2/pkg/cluster"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// MockedWatcher defines a mocked watcher
type MockedWatcher struct {
	MockedWatch          func(key string) (<-chan *string, error)
	MockedWatchPrefix    func(prefix string) (<-chan map[string]*string, error)
	MockedWatchRaw       func(key string) (<-chan *clientv3.Event, error)
	MockedWatchRawPrefix func(prefix string) (<-chan map[string]*clientv3.Event, error)
	MockedWatchWithOp    func(key string, ops ...cluster.ClientOp) (<-chan map[string]*string, error)
	MockedClose          func()
}

var _ cluster.Watcher = (*MockedWatcher)(nil)

// NewMockedWatcher return MockedWatcher
func NewMockedWatcher() *MockedWatcher {
	return &MockedWatcher{}
}

// Watch implements interface function Watch
func (w *MockedWatcher) Watch(key string) (<-chan *string, error) {
	if w.MockedWatch != nil {
		return w.MockedWatch(key)
	}
	return nil, nil
}

// WatchPrefix implements interface function WatchPrefix
func (w *MockedWatcher) WatchPrefix(prefix string) (<-chan map[string]*string, error) {
	if w.MockedWatchPrefix != nil {
		return w.MockedWatchPrefix(prefix)
	}
	return nil, nil
}

// WatchRaw implements interface function WatchRaw
func (w *MockedWatcher) WatchRaw(key string) (<-chan *clientv3.Event, error) {
	if w.MockedWatchRaw != nil {
		return w.MockedWatchRaw(key)
	}
	return nil, nil
}

// WatchRawPrefix implements interface function WatchRawPrefix
func (w *MockedWatcher) WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error) {
	if w.MockedWatchRawPrefix != nil {
		return w.MockedWatchRawPrefix(prefix)
	}
	return nil, nil
}

// WatchWithOp implements interface function WatchWithOp
func (w *MockedWatcher) WatchWithOp(key string, ops ...cluster.ClientOp) (<-chan map[string]*string, error) {
	if w.MockedWatchWithOp != nil {
		return w.MockedWatchWithOp(key, ops...)
	}
	return nil, nil
}

// Close implements interface function Close
func (w *MockedWatcher) Close() {}

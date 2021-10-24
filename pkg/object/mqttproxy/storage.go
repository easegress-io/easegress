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

import (
	"strings"
	"sync"

	"github.com/megaease/easegress/pkg/cluster"
	etcderror "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
)

type (
	storage interface {
		get(key string) (*string, error)
		getPrefix(prefix string) (map[string]string, error)
		put(key, value string) error
		delete(key string) error
	}

	mockStorage struct {
		mu    sync.RWMutex
		store map[string]string
	}

	clusterStorage struct {
		cls cluster.Cluster
	}
)

var _ storage = (*mockStorage)(nil)
var _ storage = (*clusterStorage)(nil)

func newStorage(cls cluster.Cluster) storage {
	if cls != nil {
		return &clusterStorage{cls: cls}
	}
	return &mockStorage{store: make(map[string]string)}
}

func (m *mockStorage) get(key string) (*string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val, ok := m.store[key]; ok {
		return &val, nil
	}
	return nil, etcderror.ErrKeyNotFound
}

func (m *mockStorage) getPrefix(prefix string) (map[string]string, error) {
	m.mu.RLock()
	out := make(map[string]string)
	for k, v := range m.store {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	m.mu.RUnlock()
	return out, nil
}

func (m *mockStorage) put(key, value string) error {
	m.mu.Lock()
	m.store[key] = value
	m.mu.Unlock()
	return nil
}

func (m *mockStorage) delete(key string) error {
	m.mu.Lock()
	delete(m.store, key)
	m.mu.Unlock()
	return nil
}

func (cs *clusterStorage) get(key string) (*string, error) {
	return cs.cls.Get(key)
}

func (cs *clusterStorage) getPrefix(prefix string) (map[string]string, error) {
	return cs.cls.GetPrefix(prefix)
}

func (cs *clusterStorage) put(key, value string) error {
	return cs.cls.Put(key, value)
}

func (cs *clusterStorage) delete(key string) error {
	return cs.cls.Delete(key)
}

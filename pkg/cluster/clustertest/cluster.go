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

// Package clustertest provides a mocked cluster for testing.
package clustertest

import (
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// MockedCluster defines a mocked cluster
type MockedCluster struct {
	MockedIsLeader               func() bool
	MockedLayout                 func() *cluster.Layout
	MockedGet                    func(key string) (*string, error)
	MockedGetPrefix              func(prefix string) (map[string]string, error)
	MockedGetRaw                 func(key string) (*mvccpb.KeyValue, error)
	MockedGetRawPrefix           func(prefix string) (map[string]*mvccpb.KeyValue, error)
	MockedGetWithOp              func(key string, ops ...cluster.ClientOp) (map[string]string, error)
	MockedPut                    func(key, value string) error
	MockedPutUnderTimeout        func(key, value string, timeout time.Duration) error
	MockedPutUnderLease          func(key, value string) error
	MockedPutAndDelete           func(map[string]*string) error
	MockedPutAndDeleteUnderLease func(map[string]*string) error
	MockedDelete                 func(key string) error
	MockedDeletePrefix           func(prefix string) error
	MockedSTM                    func(apply func(concurrency.STM) error) error
	MockedWatcher                func() (cluster.Watcher, error)
	MockedSyncer                 func(pullInterval time.Duration) (cluster.Syncer, error)
	MockedMutex                  func(name string) (cluster.Mutex, error)
	MockedCloseServer            func(wg *sync.WaitGroup)
	MockedStartServer            func() (chan struct{}, chan struct{}, error)
	MockedClose                  func(wg *sync.WaitGroup)
	MockedPurgeMember            func(member string) error
}

var _ cluster.Cluster = (*MockedCluster)(nil)

// NewMockedCluster creates a new mocked cluster
func NewMockedCluster() *MockedCluster {
	return &MockedCluster{}
}

// IsLeader implements interface function IsLeader
func (mc *MockedCluster) IsLeader() bool {
	if mc.MockedIsLeader != nil {
		return mc.MockedIsLeader()
	}
	return true
}

// Layout implements interface function Layout
func (mc *MockedCluster) Layout() *cluster.Layout {
	if mc.MockedLayout != nil {
		return mc.MockedLayout()
	}
	return nil
}

// Get implements interface function Get
func (mc *MockedCluster) Get(key string) (*string, error) {
	if mc.MockedGet != nil {
		return mc.MockedGet(key)
	}
	return nil, nil
}

// GetPrefix implements interface function GetPrefix
func (mc *MockedCluster) GetPrefix(prefix string) (map[string]string, error) {
	if mc.MockedGetPrefix != nil {
		return mc.MockedGetPrefix(prefix)
	}
	return nil, nil
}

// GetRaw implements interface function GetRaw
func (mc *MockedCluster) GetRaw(key string) (*mvccpb.KeyValue, error) {
	if mc.MockedGetRaw != nil {
		return mc.MockedGetRaw(key)
	}
	return nil, nil
}

// GetRawPrefix implements interface function GetRawPrefix
func (mc *MockedCluster) GetRawPrefix(prefix string) (map[string]*mvccpb.KeyValue, error) {
	if mc.MockedGetRawPrefix != nil {
		return mc.MockedGetRawPrefix(prefix)
	}
	return nil, nil
}

// GetWithOp implements interface function GetWithOp
func (mc *MockedCluster) GetWithOp(key string, ops ...cluster.ClientOp) (map[string]string, error) {
	if mc.MockedGetWithOp != nil {
		return mc.MockedGetWithOp(key, ops...)
	}
	return nil, nil
}

// Put implements interface function Put
func (mc *MockedCluster) Put(key, value string) error {
	if mc.MockedPut != nil {
		return mc.MockedPut(key, value)
	}
	return nil
}

// PutUnderTimeout implements interface function PutUnderTimeout
func (mc *MockedCluster) PutUnderTimeout(key, value string, timeout time.Duration) error {
	if mc.MockedPutUnderTimeout != nil {
		return mc.MockedPutUnderTimeout(key, value, timeout)
	}
	return nil
}

// PutUnderLease implements interface function PutUnderLease
func (mc *MockedCluster) PutUnderLease(key, value string) error {
	if mc.MockedPutUnderLease != nil {
		return mc.MockedPutUnderLease(key, value)
	}
	return nil
}

// PutAndDelete implements interface function PutAndDelete
func (mc *MockedCluster) PutAndDelete(m map[string]*string) error {
	if mc.MockedPutAndDelete != nil {
		return mc.MockedPutAndDelete(m)
	}
	return nil
}

// PutAndDeleteUnderLease implements interface function PutAndDeleteUnderLease
func (mc *MockedCluster) PutAndDeleteUnderLease(m map[string]*string) error {
	if mc.MockedPutAndDeleteUnderLease != nil {
		return mc.MockedPutAndDeleteUnderLease(m)
	}
	return nil
}

// Delete implements interface function Delete
func (mc *MockedCluster) Delete(key string) error {
	if mc.MockedDelete != nil {
		return mc.MockedDelete(key)
	}
	return nil
}

// DeletePrefix implements interface function DeletePrefix
func (mc *MockedCluster) DeletePrefix(prefix string) error {
	if mc.MockedDeletePrefix != nil {
		return mc.MockedDeletePrefix(prefix)
	}
	return nil
}

// STM implements interface function STM
func (mc *MockedCluster) STM(apply func(concurrency.STM) error) error {
	if mc.MockedSTM != nil {
		return mc.MockedSTM(apply)
	}
	return nil
}

// Watcher implements interface function Watcher
func (mc *MockedCluster) Watcher() (cluster.Watcher, error) {
	if mc.MockedWatcher != nil {
		return mc.MockedWatcher()
	}
	return nil, nil
}

// Syncer implements interface function Syncer
func (mc *MockedCluster) Syncer(pullInterval time.Duration) (cluster.Syncer, error) {
	if mc.MockedSyncer != nil {
		return mc.MockedSyncer(pullInterval)
	}
	return nil, nil
}

// Mutex implements interface function Mutex
func (mc *MockedCluster) Mutex(name string) (cluster.Mutex, error) {
	if mc.MockedMutex != nil {
		return mc.MockedMutex(name)
	}
	return nil, nil
}

// CloseServer implements interface function CloseServer
func (mc *MockedCluster) CloseServer(wg *sync.WaitGroup) {
	if mc.MockedCloseServer != nil {
		mc.MockedCloseServer(wg)
	}
}

// StartServer implements interface function StartServer
func (mc *MockedCluster) StartServer() (chan struct{}, chan struct{}, error) {
	if mc.MockedStartServer != nil {
		return mc.MockedStartServer()
	}
	return nil, nil, nil
}

// Close implements interface function Close
func (mc *MockedCluster) Close(wg *sync.WaitGroup) {
	if mc.MockedClose != nil {
		mc.MockedClose(wg)
	}
}

// PurgeMember implements interface function PurgeMember
func (mc *MockedCluster) PurgeMember(member string) error {
	if mc.MockedPurgeMember != nil {
		return mc.MockedPurgeMember(member)
	}
	return nil
}

// MockedSTM is a mocked cocurrency.STM
type MockedSTM struct {
	// embed concurrency.STM for commit & reset
	concurrency.STM
	MockedGet func(key ...string) string
	MockedPut func(key, val string, opts ...clientv3.OpOption)
	MockedRev func(key string) int64
	MockedDel func(key string)
}

// Get implements STM.Get
func (stm *MockedSTM) Get(key ...string) string {
	if stm.MockedGet != nil {
		return stm.MockedGet(key...)
	}
	return ""
}

// Put implements STM.Put
func (stm *MockedSTM) Put(key, val string, opts ...clientv3.OpOption) {
	if stm.MockedPut != nil {
		stm.MockedPut(key, val, opts...)
	}
}

// Rev implements STM.Rev
func (stm *MockedSTM) Rev(key string) int64 {
	if stm.MockedRev != nil {
		return stm.MockedRev(key)
	}
	return 0
}

// Del implements STM.Del
func (stm *MockedSTM) Del(key string) {
	if stm.MockedRev != nil {
		stm.MockedDel(key)
	}
}

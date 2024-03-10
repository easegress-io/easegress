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

package cluster

import (
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type (
	// Cluster is the open cluster interface.
	Cluster interface {
		IsLeader() bool

		Layout() *Layout

		Get(key string) (*string, error)
		GetPrefix(prefix string) (map[string]string, error)
		GetRaw(key string) (*mvccpb.KeyValue, error)
		GetRawPrefix(prefix string) (map[string]*mvccpb.KeyValue, error)
		GetWithOp(key string, ops ...ClientOp) (map[string]string, error)

		Put(key, value string) error
		PutUnderLease(key, value string) error
		PutAndDelete(map[string]*string) error
		PutAndDeleteUnderLease(map[string]*string) error

		PutUnderTimeout(key, value string, timeout time.Duration) error

		Delete(key string) error
		DeletePrefix(prefix string) error

		// The STM function is used to do cluster-level atomic operations like
		// increase/decrease an integer by one, which is very useful to create
		// a cluster-level counter.
		STM(apply func(concurrency.STM) error) error

		Watcher() (Watcher, error)
		Syncer(pullInterval time.Duration) (Syncer, error)

		Mutex(name string) (Mutex, error)

		CloseServer(wg *sync.WaitGroup)
		StartServer() (chan struct{}, chan struct{}, error)

		Close(wg *sync.WaitGroup)

		PurgeMember(member string) error
	}

	// ClientOp is client operation option type for etcd client used in cluster and watcher
	ClientOp string

	// Watcher wraps etcd watcher.
	Watcher interface {
		Watch(key string) (<-chan *string, error)
		WatchPrefix(prefix string) (<-chan map[string]*string, error)
		WatchRaw(key string) (<-chan *clientv3.Event, error)
		WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error)
		WatchWithOp(key string, ops ...ClientOp) (<-chan map[string]*string, error)
		Close()
	}

	// Syncer syncs data from Etcd, it uses an Etcd watcher to receive update.
	// The syncer keeps a full copy of data, and keeps apply changes onto it when an
	// update event is received from the watcher, and then send out the full data copy.
	// The syncer also pulls full data from Etcd at a configurable pull interval, this
	// is to ensure data consistency, as Etcd watcher may be cancelled if it cannot catch
	// up with the key-value store.
	Syncer interface {
		Sync(string) (<-chan *string, error)
		SyncRaw(string) (<-chan *mvccpb.KeyValue, error)
		SyncPrefix(string) (<-chan map[string]string, error)
		SyncRawPrefix(string) (<-chan map[string]*mvccpb.KeyValue, error)
		Close()
	}
)

const (
	// OpPrefix will watch all event with certain prefix
	OpPrefix ClientOp = "prefix"

	// OpNotWatchPut will not watch put event
	OpNotWatchPut ClientOp = "put"

	// OpNotWatchDelete will not watch delete event
	OpNotWatchDelete ClientOp = "delete"

	// OpKeysOnly will get etcd and only return keys, for example, get all prefix without values
	OpKeysOnly ClientOp = "keysOnly"
)

func getOpOption(op ClientOp) clientv3.OpOption {
	switch op {
	case OpPrefix:
		return clientv3.WithPrefix()
	case OpNotWatchPut:
		return clientv3.WithFilterPut()
	case OpNotWatchDelete:
		return clientv3.WithFilterDelete()
	case OpKeysOnly:
		return clientv3.WithKeysOnly()
	default:
		logger.Errorf("unsupported client operation: %v", op)
		return nil
	}
}

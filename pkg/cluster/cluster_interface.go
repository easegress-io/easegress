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

package cluster

import (
	"sync"
	"time"

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

		Put(key, value string) error
		PutUnderLease(key, value string) error
		PutAndDelete(map[string]*string) error
		PutAndDeleteUnderLease(map[string]*string) error

		Delete(key string) error
		DeletePrefix(prefix string) error

		// The STM function is used to do cluster-level atomic operations like
		// increase/decrease an integer by one, which is very useful to create
		// a cluster-level counter.
		STM(apply func(concurrency.STM) error) error

		Watcher() (Watcher, error)
		Syncer(pullInterval time.Duration) (*Syncer, error)

		Mutex(name string) (Mutex, error)

		CloseServer(wg *sync.WaitGroup)
		StartServer() (chan struct{}, chan struct{}, error)

		Close(wg *sync.WaitGroup)

		PurgeMember(member string) error
	}

	// Watcher wraps etcd watcher.
	Watcher interface {
		Watch(key string) (<-chan *string, error)
		WatchPrefix(prefix string) (<-chan map[string]*string, error)
		WatchRaw(key string) (<-chan *clientv3.Event, error)
		WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error)
		Close()
	}
)

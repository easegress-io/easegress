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
	"fmt"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func mockClusters(count int) []*cluster {
	opts, _, _ := mockMembers(count)
	clusters := make([]*cluster, count)

	bootCluster, err := New(opts[0])
	if err != nil {
		panic(fmt.Errorf("new cluster failed: %v", err))
	}
	clusters[0] = bootCluster.(*cluster)

	time.Sleep(HeartbeatInterval)

	for i := 1; i < count; i++ {
		opts[i].ClusterJoinURLs = opts[0].ClusterListenPeerURLs

		cls, err := New(opts[i])

		if err != nil {
			totalRetryTime := time.After(60 * time.Second)
		Loop:
			for {
				if err == nil {
					break
				}
				select {
				case <-totalRetryTime:
					break Loop

				case <-time.After(HeartbeatInterval):
					cls, err = New(opts[i])
				}
			}

		}
		if err != nil {
			panic(fmt.Errorf("new cluster failed: %v", err))
		}

		c := cls.(*cluster)

		for {
			_, err := c.getClient()
			time.Sleep(HeartbeatInterval)
			if err != nil {
				continue
			} else {
				break
			}
		}

		clusters[i] = c
	}

	// for testing longRequestContext()
	clusters[0].longRequestContext()

	return clusters
}

func closeClusters(clusters []*cluster) {
	wg := &sync.WaitGroup{}
	wg.Add(len(clusters))

	for _, cls := range clusters {
		cls.Close(wg)
	}
}

func TestCluster(t *testing.T) {
	clusters := mockClusters(3)
	defer closeClusters(clusters)
}

func TestLease(t *testing.T) {
	_, err := strToLease("266394")
	if err != nil {
		t.Errorf("str to lease failed: %v", err)
	}
}

func TestLeaseInvalid(t *testing.T) {
	_, err := strToLease("test")
	if err == nil {
		t.Errorf("str to lease should not succ with \"test\" value")
	}
}

func TestClusterSyncer(t *testing.T) {
	opts, _, _ := mockMembers(1)
	cls, err := New(opts[0])

	if err != nil {
		t.Errorf("init failed: %v", err)
	}

	c := cls.(*cluster)

	err = c.getReady()
	if err != nil {
		t.Errorf("get ready failed: %v", err)
	}

	syncer, err := c.Syncer(3 * time.Second)

	if err != nil {
		t.Errorf("new syncer failed: %v", err)
	}

	if _, err = syncer.Sync("akey"); err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	if _, err = syncer.SyncRaw("akey"); err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	if _, err = syncer.SyncPrefix("akey"); err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	if _, err = syncer.SyncRawPrefix("akey"); err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	syncer.Close()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	cls.CloseServer(wg)
	wg.Wait()
}

func TestClusterWatcher(t *testing.T) {
	opts, _, _ := mockMembers(1)
	cls, err := New(opts[0])

	if err != nil {
		t.Errorf("init failed: %v", err)
	}

	c := cls.(*cluster)

	err = c.getReady()
	if err != nil {
		t.Errorf("get ready failed: %v", err)
	}

	watcher, err := c.Watcher()

	if err != nil {
		t.Errorf("new syncer failed: %v", err)
	}

	if _, err = watcher.Watch("akey"); err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	if _, err = watcher.WatchRaw("akey"); err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	if _, err = watcher.WatchPrefix("akey"); err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	if _, err = watcher.WatchRawPrefix("akey"); err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	watcher.Close()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	cls.CloseServer(wg)
	wg.Wait()
}

func TestUtil(t *testing.T) {
	equal := isDataEuqal(map[string]*mvccpb.KeyValue{
		"aaa": {
			Key:     []byte("akey"),
			Version: 11233,
		},
	}, map[string]*mvccpb.KeyValue{
		"aaa": {
			Key:     []byte("akey"),
			Version: 11233,
		},
	})

	if !equal {
		t.Error("isDataEqual failed")
	}

	equal = isDataEuqal(map[string]*mvccpb.KeyValue{
		"aaa": {
			Key:     []byte("akey"),
			Version: 11233,
		},
		"bbb": {
			Key:     []byte("akey"),
			Version: 11233,
		},
	}, map[string]*mvccpb.KeyValue{
		"aaa": {
			Key:     []byte("akey"),
			Version: 11233,
		},
	})

	if equal {
		t.Error("isDataEqual should not equal failed")
	}
}

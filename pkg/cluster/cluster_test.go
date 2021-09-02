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
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
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
	// for testing longRequestContext()
	clusters[0].longRequestContext()

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

func TestClusterStart(t *testing.T) {
	opts, _, _ := mockMembers(1)

	cls, err := New(opts[0])

	if err != nil {
		t.Errorf("init failed: %v", err)
	}

	c := cls.(*cluster)

	_, _, err = c.StartServer()

	if err != nil {
		t.Errorf("start server failed, %v", err)
	}
}

func TestClusterPurgeMember(t *testing.T) {
	opts, _, _ := mockMembers(2)

	cls, err := New(opts[0])

	if err != nil {
		t.Errorf("init failed: %v", err)
	}

	c := cls.(*cluster)
	err = c.PurgeMember("no-member")
	if err == nil {
		t.Errorf("purge a none exit member, should be failed")
	}
}

func TestClusterSyncer(t *testing.T) {
	opts, _, _ := mockMembers(1)
	cls, err := New(opts[0])

	if err != nil {
		t.Errorf("init failed: %v", err)
	}

	c := cls.(*cluster)

	_, err = c.getClient()
	if err != nil {
		t.Errorf("get ready failed: %v", err)
	}

	syncer, err := c.Syncer(3 * time.Second)

	if err != nil {
		t.Errorf("new syncer failed: %v", err)
	}

	schan, err := syncer.Sync("/akey")
	if err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	rschan, err := syncer.SyncRaw("/akey")
	if err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	c.Put("/akey", "avalue")

	for {
		var value *string
		select {
		case value = <-schan:
			fmt.Printf("sync value is %v\n", value)
		}
		break
	}

	for {
		var value *mvccpb.KeyValue
		select {
		case value = <-rschan:
			fmt.Printf("sync raw value is %v\n", value)
		}
		break
	}

	pchan, err := syncer.SyncPrefix("/abcd")
	if err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	rpchan, err := syncer.SyncRawPrefix("/abcd")
	if err != nil {
		t.Errorf("syncer sync failed: %v", err)
	}

	c.Put("/abcd/efg", "yoyo")

	if _, err = c.GetPrefix("/abcd"); err != nil {
		t.Errorf("cluster get prefix failed: %v", err)
	}

	for {
		value := make(map[string]string, 1)
		select {
		case value = <-pchan:
			fmt.Printf("sync prefix value is %v\n", value)
		}
		break
	}

	for {
		value := make(map[string]*mvccpb.KeyValue)
		select {
		case value = <-rpchan:
			fmt.Printf("sync raw pvalue is %v\n", value)
		}
		break
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

	_, err = c.getClient()
	if err != nil {
		t.Errorf("get ready failed: %v", err)
	}

	watcher, err := c.Watcher()

	if err != nil {
		t.Errorf("new syncer failed: %v", err)
	}

	wchan, err := watcher.Watch("/akey/value")
	if err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	rchan, err := watcher.WatchRaw("/akey/value")
	if err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	c.Put("/akey/value", "yes")

	for {
		var value *string
		select {
		case value = <-wchan:
			fmt.Printf("watch value is %v\n", value)
		}
		break
	}

	for {
		var value *clientv3.Event
		select {
		case value = <-rchan:
			fmt.Printf("watch raw value is %v\n", value)
		}
		break
	}

	pchan, err := watcher.WatchPrefix("/ab")
	if err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	rawpchan, err := watcher.WatchRawPrefix("/abcd")
	if err != nil {

		t.Errorf("watcher watch failed: %v", err)
	}

	c.Put("/abc", "kkk")

	for {
		value := make(map[string]*string, 1)
		select {
		case value = <-pchan:
			fmt.Printf("watch prefix value is %v\n", value)
		}
		break
	}
	c.Put("/abcd/ef", "jjj")

	for {
		value := make(map[string]*clientv3.Event, 1)
		select {
		case value = <-rawpchan:
			fmt.Printf("watch prefix raw value is %v\n", value)
		}
		break
	}
	c.DeletePrefix("/abc")

	for {
		value := make(map[string]*string, 1)
		select {
		case value = <-pchan:
			fmt.Printf("watch delete prefix value is %v\n", value)
		}
		break
	}

	c.DeletePrefix("/abcd/")

	for {
		value := make(map[string]*clientv3.Event, 1)
		select {
		case value = <-rawpchan:
			fmt.Printf("watch delete prefix raw value is %v\n", value)
		}
		break
	}

	watcher.Close()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	cls.CloseServer(wg)
	wg.Wait()
}

func TestUtil(t *testing.T) {
	equal := isDataEqual(map[string]*mvccpb.KeyValue{
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

	equal = isDataEqual(map[string]*mvccpb.KeyValue{
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

func TestMutexAndOP(t *testing.T) {
	opts, _, _ := mockMembers(1)
	cls, err := New(opts[0])

	if err != nil {
		t.Errorf("init failed: %v", err)
	}

	c := cls.(*cluster)

	_, err = c.getClient()
	if err != nil {
		t.Errorf("get ready failed: %v", err)
	}

	m, err := c.Mutex("akey")
	if err != nil {
		t.Errorf("cluster mutex failed: %v", err)
	}

	m.Lock()
	defer m.Unlock()

	value := "a value"
	err = c.PutAndDeleteUnderLease(map[string]*string{
		"akey": &value,
	})

	if err != nil {
		t.Errorf("PutAndDeleteUnderLease failed: %v", err)
	}

	err = c.PutAndDelete(map[string]*string{
		"/test/akey": &value,
	})

	if err != nil {
		t.Errorf("PutAndDelete failed :%v", err)
	}

	if err = c.Delete("/test/akey"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	err = c.DeletePrefix("akey")
	if err != nil {
		t.Errorf("DeletePrefix failed: %v", err)
	}

	err = c.STM(func(s concurrency.STM) error {
		return nil
	})
	if err != nil {
		t.Errorf("STM failed: %v", err)
	}
}

func TestUtilEqual(t *testing.T) {
	equal := isKeyValueEqual(&mvccpb.KeyValue{
		Key: []byte("abc"),
	}, &mvccpb.KeyValue{
		Key: []byte("abc"),
	})

	if !equal {
		t.Error("isKeyValueEqual invalid")
	}

	equal = isKeyValueEqual(nil, &mvccpb.KeyValue{
		Key: []byte("abc"),
	})

	if equal {
		t.Error("isKeyValueEqual invalid, should not equal")
	}

	equal = isKeyValueEqual(&mvccpb.KeyValue{
		Key: []byte("abc"),
	}, nil)

	if equal {
		t.Error("isKeyValueEqual invalid, should not equal")
	}

	equal = isKeyValueEqual(nil, nil)
	if !equal {
		t.Error("isKeyValueEqual invalid, should equal")
	}
}

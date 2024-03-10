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
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/phayes/freeport"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/megaease/easegress/v2/pkg/env"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/stretchr/testify/assert"
)

var memberCounter = 0

func mockTestOpt(ports []int) *option.Options {
	memberCounter++
	name := fmt.Sprintf("test-member-%03d", memberCounter)

	opt := option.New()
	opt.Name = name
	opt.ClusterName = "test-cluster"
	opt.ClusterRole = "primary"
	opt.ClusterRequestTimeout = "10s"
	opt.Cluster.ListenClientURLs = []string{fmt.Sprintf("http://localhost:%d", ports[0])}
	opt.Cluster.AdvertiseClientURLs = opt.Cluster.ListenClientURLs
	opt.Cluster.ListenPeerURLs = []string{fmt.Sprintf("http://localhost:%d", ports[1])}
	opt.Cluster.InitialAdvertisePeerURLs = opt.Cluster.ListenPeerURLs
	opt.APIAddr = fmt.Sprintf("localhost:%d", ports[2])
	opt.HomeDir = filepath.Join(tempDir, name)
	opt.DataDir = "data"
	opt.LogDir = "log"
	opt.Debug = false

	if err := opt.Parse(); err != nil {
		panic(fmt.Errorf("parse option failed: %v", err))
	}

	return opt
}

func mockMembers(count int) ([]*option.Options, []*pb.Member) {
	ports, err := freeport.GetFreePorts(count * 3)
	if err != nil {
		panic(fmt.Errorf("get %d free ports failed: %v", count*3, err))
	}

	opts := make([]*option.Options, count)
	pbMembers := make([]*pb.Member, count)

	for i := 0; i < count; i++ {
		id := i + 1
		opt := mockTestOpt(ports[i*3 : (i+1)*3])

		opts[i] = opt
		pbMembers[i] = &pb.Member{
			ID:         uint64(id),
			Name:       opt.Name,
			PeerURLs:   []string{opt.Cluster.InitialAdvertisePeerURLs[0]},
			ClientURLs: []string{opt.Cluster.AdvertiseClientURLs[0]},
		}

		env.InitServerDir(opt)
	}

	initCluster := map[string]string{}
	for _, opt := range opts {
		if opt.ClusterRole == "primary" {
			initCluster[opt.Name] = opt.Cluster.ListenPeerURLs[0]
		}
	}
	for _, opt := range opts {
		if opt.ClusterRole == "primary" {
			opt.Cluster.InitialCluster = initCluster
		}
	}
	return opts, pbMembers
}

var tempDir = os.TempDir()

func getRandomString(n int) string {
	randBytes := make([]byte, n/2)
	rand.Read(randBytes)
	return fmt.Sprintf("%x", randBytes)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	// logger.InitMock()
	tempDir = path.Join(tempDir, getRandomString(6))
	code := m.Run()
	os.Exit(code)
}

func mockStaticClusterMembers(count int) ([]*option.Options, []*pb.Member) {
	opts := make([]*option.Options, count)
	pbMembers := make([]*pb.Member, count)

	portCount := (count * 2) + 1 // two for each member and one for egctl API.
	ports, err := freeport.GetFreePorts(portCount)
	if err != nil {
		panic(fmt.Errorf("get %d free ports failed: %v", portCount, err))
	}
	initialCluster := make(map[string]string)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("static-cluster-test-member-%03d", i)
		peerURL := fmt.Sprintf("http://localhost:%d", ports[(i*2)+1])
		initialCluster[name] = peerURL
	}

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("static-cluster-test-member-%03d", i)
		opt := option.New()
		opt.Name = name
		opt.ClusterName = "test-static-sized-cluster"
		opt.ClusterRole = "primary"
		opt.ClusterRequestTimeout = "10s"
		listenPort := ports[(i*2)+2]
		advertisePort := ports[(i*2)+1]

		opt.APIAddr = fmt.Sprintf("localhost:%d", ports[0])
		opt.Cluster.ListenClientURLs = []string{fmt.Sprintf("http://localhost:%d", listenPort)}
		opt.Cluster.AdvertiseClientURLs = opt.Cluster.ListenClientURLs
		opt.Cluster.ListenPeerURLs = []string{fmt.Sprintf("http://localhost:%d", advertisePort)}
		opt.Cluster.InitialAdvertisePeerURLs = opt.Cluster.ListenPeerURLs
		opt.Cluster.InitialCluster = initialCluster
		opt.HomeDir = filepath.Join(tempDir, name)
		opt.DataDir = "data"
		opt.LogDir = "log"
		opt.Debug = false
		err = opt.Parse() // create directories
		if err != nil {
			panic(fmt.Errorf("parse option failed: %v", err))
		}

		id := uint64(i + 1)

		opts[i] = opt
		pbMembers[i] = &pb.Member{
			ID:         id,
			Name:       opt.Name,
			PeerURLs:   []string{opt.Cluster.InitialAdvertisePeerURLs[0]},
			ClientURLs: []string{opt.Cluster.AdvertiseClientURLs[0]},
		}
		env.InitServerDir(opts[i])
	}
	return opts, pbMembers
}

func mockStaticCluster(count int) []*cluster {
	opts, _ := mockStaticClusterMembers(count)

	clusterNodes := make([]*cluster, count)
	clusterNodesLock := sync.Mutex{}
	clusterCreationWg := &sync.WaitGroup{}
	clusterCreationWg.Add(count)
	startNode := func(i int) error {
		node, err := New(opts[i])
		if err != nil {
			panic(fmt.Errorf("new cluster failed: %v", err))
		}
		clusterNodesLock.Lock()
		clusterNodes[i] = node.(*cluster)
		clusterNodesLock.Unlock()
		clusterCreationWg.Done()
		return nil
	}

	for i := 0; i < count; i++ {
		go startNode(i)
	}
	clusterCreationWg.Wait()

	for {
		_, err := clusterNodes[0].getClient()
		time.Sleep(HeartbeatInterval)
		if err != nil {
			fmt.Println(err)
			continue
		} else {
			break
		}
	}
	return clusterNodes
}

func closeClusters(clusters []*cluster) {
	wg := &sync.WaitGroup{}
	wg.Add(len(clusters))

	for _, cls := range clusters {
		cls.Close(wg)
	}
}

func createSecondaryNode(clusterName string, primaryListenPeerURLs []string) *cluster {
	ports, err := freeport.GetFreePorts(1)
	check(err)
	name := "secondary-member-x"
	opt := option.New()
	opt.Name = name
	opt.ClusterName = clusterName
	opt.ClusterRole = "secondary"
	opt.ClusterRequestTimeout = "10s"
	opt.Cluster.PrimaryListenPeerURLs = primaryListenPeerURLs
	opt.APIAddr = fmt.Sprintf("localhost:%d", ports[0])

	err = opt.Parse()
	check(err)

	env.InitServerDir(opt)

	clusterInstance, err := New(opt)
	check(err)
	return clusterInstance.(*cluster)
}

func TestCluster(t *testing.T) {
	clusterNodes := mockStaticCluster(3)
	primaryName := clusterNodes[0].opt.ClusterName
	primaryAddress := clusterNodes[0].opt.Cluster.InitialAdvertisePeerURLs
	secondaryNode := createSecondaryNode(primaryName, primaryAddress)
	defer closeClusters(clusterNodes)
	defer closeClusters([]*cluster{secondaryNode})
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
	opts, _ := mockMembers(1)

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
	assert := assert.New(t)
	opts, _ := mockMembers(2)

	go func() {
		_, err := New(opts[1])
		assert.Nil(err)
	}()

	cls, err := New(opts[0])
	assert.Nil(err)

	c := cls.(*cluster)
	err = c.PurgeMember("no-member")
	assert.NotNil(err)
}

func TestClusterSyncer(t *testing.T) {
	opts, _ := mockMembers(1)
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

	{
		value := <-schan
		fmt.Printf("sync value is %v\n", value)
	}

	{
		value := <-rschan
		fmt.Printf("sync raw value is %v\n", value)
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
	if _, err = c.GetWithOp("/abcd", OpPrefix); err != nil {
		t.Errorf("cluster get prefix failed: %v", err)
	}

	{
		value := <-pchan
		fmt.Printf("sync prefix value is %v\n", value)
	}

	{
		value := <-rpchan
		fmt.Printf("sync raw pvalue is %v\n", value)
	}

	syncer.Close()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	cls.CloseServer(wg)
	wg.Wait()
}

func TestClusterWatcher(t *testing.T) {
	opts, _ := mockMembers(1)
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

	{
		value := <-wchan
		fmt.Printf("watch value is %v\n", value)
	}

	{
		value := <-rchan
		fmt.Printf("watch raw value is %v\n", value)
	}

	pchan, err := watcher.WatchPrefix("/ab")
	if err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}
	opchan, err := watcher.WatchWithOp("/ab", OpPrefix)
	if err != nil {
		t.Errorf("watcher watch failed: %v", err)
	}

	rawpchan, err := watcher.WatchRawPrefix("/abcd")
	if err != nil {

		t.Errorf("watcher watch failed: %v", err)
	}

	c.Put("/abc", "kkk")

	{
		value := <-pchan
		fmt.Printf("watch prefix value is %v\n", value)
	}

	{
		value := <-opchan
		fmt.Printf("watch prefix value is %v\n", value)
	}
	c.Put("/abcd/ef", "jjj")

	{
		value := <-rawpchan
		fmt.Printf("watch prefix raw value is %v\n", value)
	}

	c.DeletePrefix("/abc")

	{
		value := <-pchan
		fmt.Printf("watch delete prefix value is %v\n", value)
	}

	{
		value := <-opchan
		fmt.Printf("watch delete prefix value is %v\n", value)
	}

	c.DeletePrefix("/abcd/")

	{
		value := <-rawpchan
		fmt.Printf("watch delete prefix raw value is %v\n", value)
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
	opts, _ := mockMembers(1)
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

	err = c.PutUnderTimeout("/test/abcd", "value", 10*time.Second)
	if err != nil {
		t.Errorf("PutUnderTimeout failed: %v", err)
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

func TestIsLeader(t *testing.T) {
	etcdDirName, err := os.MkdirTemp("", "cluster-test")
	check(err)
	defer os.RemoveAll(etcdDirName)

	clusterInstance := CreateClusterForTest(etcdDirName)
	if !clusterInstance.IsLeader() {
		t.Error("single node cluster should be leader")
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	clusterInstance.CloseServer(wg)
	wg.Wait()
}

func TestInvalidConfig(t *testing.T) {
	assert := assert.New(t)
	opt := option.New()
	opt.ClusterRequestTimeout = "10ssss"
	_, err := New(opt)
	assert.NotNil(err)
}

func TestRunDefrag(t *testing.T) {
	assert := assert.New(t)
	etcdDirName, err := os.MkdirTemp("", "cluster-test")
	check(err)
	defer os.RemoveAll(etcdDirName)

	opt := CreateOptionsForTest(etcdDirName)

	cluster := &cluster{
		opt:            opt,
		requestTimeout: 1 * time.Second,
		done:           make(chan struct{}),
	}
	cluster.initLayout()
	cluster.run()

	assert.Equal(defragNormalInterval, cluster.runDefrag())
	cluster.opt.Cluster.AdvertiseClientURLs[0] = "wrong-urlll"
	assert.Equal(defragFailedInterval, cluster.runDefrag())
	cluster.opt.Cluster.AdvertiseClientURLs = []string{} // make GetFirstAdvertiseClientURL fail
	assert.Equal(defragNormalInterval, cluster.runDefrag())

	// test session
	cluster.session = nil
	_, err = cluster.getSession()
	assert.Nil(err)
	_, err = cluster.getSession()
	assert.Nil(err)

	// test checkClusterName
	originalName := cluster.opt.ClusterName
	cluster.opt.ClusterName = "totally different"
	assert.Panics(func() { cluster.checkClusterName() })
	cluster.opt.ClusterName = originalName
	cluster.Delete(cluster.Layout().ClusterNameKey())
	assert.NotNil(cluster.checkClusterName())

	wg := &sync.WaitGroup{}
	wg.Add(1)
	cluster.CloseServer(wg)
	wg.Wait()

	assert.NotNil(cluster.checkClusterName())
}

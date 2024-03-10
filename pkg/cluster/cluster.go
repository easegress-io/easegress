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

// Package cluster provides the cluster management.
package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/server/v3/embed"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// HeartbeatInterval is the interval for heartbeat.
	HeartbeatInterval = 5 * time.Second

	defragNormalInterval = 1 * time.Hour
	defragFailedInterval = 1 * time.Minute

	// waitServerTimeout is the timeout for waiting server to start.
	waitServerTimeout = 10 * time.Minute

	// client config
	autoSyncInterval     = 1 * time.Minute
	dialTimeout          = 10 * time.Second
	dialKeepAliveTime    = 1 * time.Minute
	dialKeepAliveTimeout = 1 * time.Minute

	// lease config
	leaseTTL = clientv3.MaxLeaseTTL // 9000000000Second=285Year

	minTTL = 5 // grant a new lease if the lease ttl is less than minTTL
)

type (
	// MemberStatus is the member status.
	MemberStatus struct {
		Options option.Options `json:"options"`

		// RFC3339 format
		LastHeartbeatTime string `json:"lastHeartbeatTime"`

		LastDefragTime string `json:"lastDefragTime,omitempty"`

		// Etcd is non-nil only if it's cluster status is primary.
		Etcd *EtcdStatus `json:"etcd,omitempty"`
	}

	// EtcdStatus is the etcd status,
	// and extracts fields from server.Server.SelfStats.
	EtcdStatus struct {
		ID        string `json:"id"`
		StartTime string `json:"startTime"`
		State     string `json:"state"`
	}

	// etcdStats aims to extract fields from server.Server.SelfStats.
	etcdStats struct {
		ID        string    `json:"id"`
		State     string    `json:"state"`
		StartTime time.Time `json:"startTime"`
	}
)

func strToLease(s string) (*clientv3.LeaseID, error) {
	leaseNum, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return nil, err
	}
	leaseID := clientv3.LeaseID(leaseNum)
	return &leaseID, nil
}

func newEtcdStats(buff []byte) (*etcdStats, error) {
	stats := etcdStats{}
	err := codectool.Unmarshal(buff, &stats)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

func (s *etcdStats) toEtcdStatus() *EtcdStatus {
	return &EtcdStatus{
		ID:        s.ID,
		State:     strings.TrimPrefix(s.State, "State"),
		StartTime: s.StartTime.Format(time.RFC3339),
	}
}

type cluster struct {
	opt            *option.Options
	requestTimeout time.Duration

	layout *Layout

	server       *embed.Etcd
	client       *clientv3.Client
	lease        *clientv3.LeaseID
	session      *concurrency.Session
	serverMutex  sync.RWMutex
	clientMutex  sync.RWMutex
	leaseMutex   sync.RWMutex
	sessionMutex sync.RWMutex

	done chan struct{}
}

// New creates a cluster asynchronously,
// return non-nil err only if reaching hard limit.
func New(opt *option.Options) (Cluster, error) {
	// defensive programming
	requestTimeout, err := time.ParseDuration(opt.ClusterRequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster request timeout: %v", err)
	}

	if len(opt.GetPeerURLs()) == 0 {
		return nil, fmt.Errorf("no peer urls in cluster.initial-cluster for primary and cluster.primary-listen-peer-url for secondary")
	}

	c := &cluster{
		opt:            opt,
		requestTimeout: requestTimeout,
		done:           make(chan struct{}),
	}

	c.initLayout()

	c.run()

	return c, nil
}

func (c *cluster) IsLeader() bool {
	server, err := c.getServer()
	if err != nil {
		return false
	}

	return server.Server.Leader() == server.Server.ID()
}

// requestContext returns context with request timeout,
// please use it immediately in case of incorrect timeout.
func (c *cluster) requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.requestTimeout)
}

// longRequestContext takes 3 times longer than requestContext.
func (c *cluster) longRequestContext() (context.Context, context.CancelFunc) {
	requestTimeout := 3 * c.requestTimeout
	return context.WithTimeout(context.Background(), requestTimeout)
}

func (c *cluster) run() {
	tryTimes := 0
	tryReady := func() error {
		tryTimes++
		return c.getReady()
	}

	// NOTE: Try to be ready in first time synchronously.
	// If it got failed, try it asynchronously.
	if err := tryReady(); err != nil {
		logger.Errorf("start cluster failed (%d retries): %v", tryTimes, err)

		for {
			time.Sleep(HeartbeatInterval)
			err := tryReady()
			if err != nil {
				logger.Errorf("failed start many times(%d), "+
					"start others if they're not online, "+
					"otherwise purge this member, clean data directory "+
					"and rejoin it back.", tryTimes)
			} else {
				break
			}
		}
	}

	logger.Infof("cluster is ready")

	if c.opt.ClusterRole == "primary" {
		go c.defrag()
	}

	go c.heartbeat()
}

func (c *cluster) getReady() error {
	if c.opt.ClusterRole == "secondary" {
		_, err := c.getClient()
		if err != nil {
			return err
		}

		err = c.checkClusterName()
		if err != nil {
			return err
		}

		err = c.initLease()
		if err != nil {
			return fmt.Errorf("init lease failed: %v", err)
		}

		go c.keepAliveLease()

		return nil
	}

	done, timeout, err := c.startServer()
	if err != nil {
		return fmt.Errorf("start server failed: %v", err)
	}

	select {
	case <-done:
		_, err = c.getClient()
		if err != nil {
			return err
		}
	case <-timeout:
		err := fmt.Errorf("start server timeout(%v)", waitServerTimeout)
		logger.Errorf("%v", err)
		panic(err)
	}

	err = c.initLease()
	if err != nil {
		return fmt.Errorf("init lease failed: %v", err)
	}

	go c.keepAliveLease()

	return nil
}

// checkClusterName checks if the local configured cluster name
// matches the existed cluster name in etcd.
// This function returns error if it can't check,
// panics if it checked and found the names are not the same.
func (c *cluster) checkClusterName() error {
	value, err := c.Get(c.Layout().ClusterNameKey())
	if err != nil {
		return fmt.Errorf("failed to check cluster name: %v", err)
	}

	if value != nil {
		if c.opt.ClusterName != *value {
			err := fmt.Errorf("cluster names mismatch, local(%s) != existed(%s)",
				c.opt.ClusterName, *value)
			logger.Errorf("%v", err)
			panic(err)
		}
	} else if c.opt.UseStandaloneEtcd {
		err := c.Put(c.Layout().ClusterNameKey(), c.opt.ClusterName)
		if err != nil {
			return fmt.Errorf("register cluster name %s failed: %v",
				c.opt.ClusterName, err)
		}
	} else {
		return fmt.Errorf("key %s not found", c.Layout().ClusterNameKey())
	}

	return nil
}

func (c *cluster) getClient() (*clientv3.Client, error) {
	c.clientMutex.RLock()
	if c.client != nil {
		client := c.client
		c.clientMutex.RUnlock()
		return client, nil
	}
	c.clientMutex.RUnlock()

	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	// DCL
	if c.client != nil {
		return c.client, nil
	}

	endpoints := c.opt.GetPeerURLs()
	logger.Infof("client connect with endpoints: %v", endpoints)
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		AutoSyncInterval:     autoSyncInterval,
		DialTimeout:          dialTimeout,
		DialKeepAliveTime:    dialKeepAliveTime,
		DialKeepAliveTimeout: dialKeepAliveTimeout,
		LogConfig:            logger.EtcdClientLoggerConfig(c.opt, logger.EtcdClientFilename),
		MaxCallSendMsgSize:   c.opt.Cluster.MaxCallSendMsgSize,
	})
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}

	logger.Infof("client is ready")

	c.client = client

	return client, nil
}

func (c *cluster) closeClient() {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.client == nil {
		return
	}

	err := c.client.Close()
	if err != nil {
		logger.Errorf("close client failed: %v", err)
	}

	c.client = nil
}

func (c *cluster) getLease() (clientv3.LeaseID, error) {
	c.leaseMutex.RLock()
	defer c.leaseMutex.RUnlock()

	if c.lease == nil {
		return 0, fmt.Errorf("lease is not ready")
	}

	return *c.lease, nil
}

func (c *cluster) keepAliveLease() {
	handleFailed := func() {
		err := c.grantNewLease()
		if err != nil {
			logger.Errorf("grant new lease failed: %v", err)
		}
	}

	for {
		select {
		case <-c.done:
			return
		case <-time.After(c.requestTimeout):
			client, err := c.getClient()
			if err != nil {
				logger.Errorf("get client failed: %v", err)
				continue
			}

			leaseID, err := c.getLease()
			if err != nil {
				logger.Errorf("get lease failed: %v", err)
				handleFailed()
				continue
			}

			_, err = func() (*clientv3.LeaseKeepAliveResponse, error) {
				ctx, cancel := c.requestContext()
				defer cancel()
				return client.Lease.KeepAliveOnce(ctx, leaseID)
			}()
			if err != nil {
				logger.Errorf("keep alive for lease %x failed: %v", leaseID, err)
				handleFailed()
				continue
			}
		}
	}
}

func (c *cluster) initLease() error {
	leaseStr, err := c.Get(c.Layout().Lease())
	if err != nil {
		return err
	}

	var leaseID *clientv3.LeaseID
	if leaseStr != nil {
		leaseID, err = strToLease(*leaseStr)
		if err != nil {
			logger.Errorf("BUG: parse lease %s failed: %v", *leaseStr, err)
			return err
		}
	}

	client, err := c.getClient()
	if err != nil {
		return fmt.Errorf("get client failed: %v", err)
	}

	if leaseID != nil {
		resp, err := func() (*clientv3.LeaseTimeToLiveResponse, error) {
			ctx, cancel := c.requestContext()
			defer cancel()
			return client.Lease.TimeToLive(ctx, *leaseID)
		}()
		if err != nil || resp.TTL < minTTL {
			return c.grantNewLease()
		}
		// NOTE: Use existed lease.
		c.lease = leaseID
		logger.Infof("lease is ready(use existed one: %x)", *c.lease)
		return nil

	}
	return c.grantNewLease()
}

func (c *cluster) grantNewLease() error {
	client, err := c.getClient()
	if err != nil {
		return fmt.Errorf("get client failed: %v", err)
	}

	c.leaseMutex.Lock()
	defer c.leaseMutex.Unlock()

	respGrant, err := func() (*clientv3.LeaseGrantResponse, error) {
		ctx, cancel := c.requestContext()
		defer cancel()
		return client.Lease.Grant(ctx, leaseTTL)
	}()
	if err != nil {
		return err
	}

	// NOTE: c.PutUnderLease will cause deadlock cause it used lease lock internally.
	_, err = func() (*clientv3.PutResponse, error) {
		ctx, cancel := c.requestContext()
		defer cancel()
		return client.Put(ctx, c.layout.Lease(), fmt.Sprintf("%x", respGrant.ID),
			clientv3.WithLease(respGrant.ID))
	}()

	if err != nil {
		// NOTE: Ignore the return error is fine.
		func() (*clientv3.LeaseRevokeResponse, error) {
			ctx, cancel := c.requestContext()
			defer cancel()
			return client.Lease.Revoke(ctx, respGrant.ID)
		}()

		return fmt.Errorf("put lease to %s failed: %v", c.Layout().Lease(), err)
	}

	lease := respGrant.ID
	c.lease = &lease

	logger.Infof("lease is ready (grant new one: %x)", *c.lease)

	return nil
}

func (c *cluster) getSession() (*concurrency.Session, error) {
	c.sessionMutex.RLock()
	if c.session != nil {
		session := c.session
		c.sessionMutex.RUnlock()
		return session, nil
	}
	c.sessionMutex.RUnlock()

	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()

	// DCL
	if c.session != nil {
		return c.session, nil
	}

	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	lease, err := c.getLease()
	if err != nil {
		return nil, err
	}

	session, err := concurrency.NewSession(client,
		concurrency.WithLease(lease))
	if err != nil {
		return nil, fmt.Errorf("create session failed: %v", err)
	}

	c.session = session

	logger.Infof("session is ready")

	return session, nil
}

func (c *cluster) closeSession() {
	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()

	if c.session == nil {
		return
	}

	err := c.session.Close()
	if err != nil {
		logger.Errorf("close session failed: %v", err)
	}

	c.session = nil
}

func (c *cluster) getServer() (*embed.Etcd, error) {
	c.serverMutex.RLock()
	defer c.serverMutex.RUnlock()
	if c.server == nil {
		return nil, fmt.Errorf("server is not ready")
	}
	return c.server, nil
}

func closeEtcdServer(s *embed.Etcd) {
	select {
	case <-s.Server.ReadyNotify():
		s.Close()
		<-s.Server.StopNotify()
	default:
		s.Server.HardStop()
		for _, client := range s.Clients {
			if client != nil {
				client.Close()
			}
		}
		for _, peer := range s.Peers {
			if peer != nil {
				peer.Close()
			}
		}
		logger.Infof("hard stop server")
	}
}

func (c *cluster) startServer() (done, timeout chan struct{}, err error) {
	c.serverMutex.Lock()
	defer c.serverMutex.Unlock()

	done, timeout = make(chan struct{}), make(chan struct{})
	if c.server != nil {
		close(done)
		return done, timeout, nil
	}

	etcdConfig, err := CreateStaticClusterEtcdConfig(c.opt)
	if err != nil {
		return nil, nil, err
	}

	server, err := embed.StartEtcd(etcdConfig)
	if err != nil {
		return nil, nil, err
	}

	monitorServer := func(s *embed.Etcd) {
		select {
		case err, ok := <-s.Err():
			if ok {
				logger.Errorf("etcd server %s serve failed: %v",
					c.server.Config().Name, err)
				closeEtcdServer(s)
			}
		case <-c.done:
			return
		}
	}

	go func() {
		select {
		case <-c.done:
			return
		case <-server.Server.ReadyNotify():
			c.server = server
			if c.server.Config().IsNewCluster() {
				err := c.Put(c.Layout().ClusterNameKey(), c.opt.ClusterName)
				if err != nil {
					err = fmt.Errorf("register cluster name %s failed: %v",
						c.opt.ClusterName, err)
					logger.Errorf("%v", err)
					panic(err)
				}
			}
			go monitorServer(c.server)
			logger.Infof("server is ready")
			close(done)
		case <-time.After(waitServerTimeout):
			closeEtcdServer(server)
			close(timeout)
		}
	}()

	return done, timeout, nil
}

func (c *cluster) closeServer() {
	c.serverMutex.Lock()
	defer c.serverMutex.Unlock()

	if c.server == nil {
		return
	}

	closeEtcdServer(c.server)
	c.server = nil
}

func (c *cluster) CloseServer(wg *sync.WaitGroup) {
	defer wg.Done()
	c.closeServer()
}

func (c *cluster) StartServer() (done, timeout chan struct{}, err error) {
	return c.startServer()
}

func (c *cluster) heartbeat() {
	for {
		select {
		case <-time.After(HeartbeatInterval):
			err := c.syncStatus()
			if err != nil {
				logger.Errorf("sync status failed: %v", err)
			}
		case <-c.done:
			return
		}
	}
}

func (c *cluster) runDefrag() time.Duration {
	client, err := c.getClient()
	if err != nil {
		logger.Errorf("defrag failed: get client failed: %v", err)
		return defragFailedInterval
	}
	defragmentURL, err := c.opt.GetFirstAdvertiseClientURL()
	if err != nil {
		logger.Errorf("defrag failed: %v", err)
		return defragNormalInterval // url is wrong
	}
	// NOTICE: It needs longer time than normal ones.
	_, err = func() (*clientv3.DefragmentResponse, error) {
		ctx, cancel := c.longRequestContext()
		defer cancel()
		return client.Defragment(ctx, defragmentURL)
	}()
	if err != nil {
		logger.Errorf("defrag failed: %v", err)
		return defragFailedInterval
	}

	logger.Infof("defrag successfully")
	return defragNormalInterval
}

func (c *cluster) defrag() {
	defragInterval := defragNormalInterval
	for {
		select {
		case <-time.After(defragInterval):
			defragInterval = c.runDefrag()
		case <-c.done:
			return
		}
	}
}

func (c *cluster) syncStatus() error {
	status := MemberStatus{
		Options: *c.opt,
	}

	if c.opt.ClusterRole == "primary" {
		server, err := c.getServer()
		if err != nil {
			return err
		}

		buff := server.Server.SelfStats()
		stats, err := newEtcdStats(buff)
		if err != nil {
			return err
		}
		status.Etcd = stats.toEtcdStatus()
	}

	status.LastHeartbeatTime = time.Now().Format(time.RFC3339)

	buff, err := codectool.MarshalJSON(status)
	if err != nil {
		return err
	}

	err = c.PutUnderLease(c.Layout().StatusMemberKey(), string(buff))
	if err != nil {
		return fmt.Errorf("put status failed: %v", err)
	}
	return nil
}

func (c *cluster) PurgeMember(memberName string) error {
	client, err := c.getClient()
	if err != nil {
		return err
	}

	// remove etcd member if there is it.
	respList, err := func() (*clientv3.MemberListResponse, error) {
		ctx, cancel := c.requestContext()
		defer cancel()
		return client.MemberList(ctx)
	}()
	if err != nil {
		return err
	}
	var id *uint64
	for _, member := range respList.Members {
		if member.Name == memberName {
			id = &member.ID
		}
	}
	if id != nil {
		_, err = func() (*clientv3.MemberRemoveResponse, error) {
			ctx, cancel := c.requestContext()
			defer cancel()
			return client.MemberRemove(ctx, *id)
		}()
		if err != nil {
			return err
		}
	}

	// remove all stuff under the lease of the member.
	leaseKey := c.Layout().OtherLease(memberName)
	leaseStr, err := c.Get(leaseKey)
	if err != nil {
		return err
	}
	if leaseStr == nil {
		return fmt.Errorf("%s not found", leaseKey)
	}
	leaseID, err := strToLease(*leaseStr)
	if err != nil {
		return err
	}

	_, err = func() (*clientv3.LeaseRevokeResponse, error) {
		ctx, cancel := c.requestContext()
		defer cancel()
		return client.Lease.Revoke(ctx, *leaseID)
	}()
	if err != nil {
		return err
	}

	return nil
}

func (c *cluster) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	close(c.done)

	c.closeSession()
	c.closeClient()
	c.closeServer()
}

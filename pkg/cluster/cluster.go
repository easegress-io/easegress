package cluster

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ghodss/yaml"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"go.etcd.io/etcd/embed"
	"go.etcd.io/etcd/etcdserver/api/v2stats"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

// Cluster store tree layout.
var (
	MemberConfigPrefix = "/runtime/members/config/"
	MemberStatusPrefix = "/runtime/members/status/"
	MemberConfigKey    string

	ConfigObjectPrefix           = "/config/objects/"
	ConfigPluginPrefix           = "/config/objects/plugins/"      // + plugin-name
	ConfigPipelinePrefix         = "/config/objects/pipelines/"    // + pipeline-name
	ConfigPluginUsedPrefixFormat = "/config/relation/%s/usedby/"   // + plugin-name
	ConfigPluginUsedKeyFormat    = "/config/relation/%s/usedby/%s" // + plugin-name pipeline-name

	StatPipelinePrefix = "/runtime/stat/pipelines/"
	StatPipelineFormat string // + pipeline-name

	MemberConfigValue string = option.GlobalYAML
)

const (
	// client config
	autoSyncInterval     = 1 * time.Minute
	dialTimeout          = 10 * time.Second
	dialKeepAliveTime    = 1 * time.Minute
	dialKeepAliveTimeout = 1 * time.Minute

	// lease config
	sessionTTL      = 60 // Second
	registerTimeout = 10 * time.Second
)

type cluster struct {
	Cluster
	server                *embed.Etcd
	client                *clientv3.Client
	session               *concurrency.Session
	closed                int32
	started               int32
	canceLearnEtcdMembers context.CancelFunc
	etcdTimeoutInMilli    time.Duration
	name                  string
}

// New creates an etcd instance and start to serve.
//
// The instance can be a writer or reader, depending on the config in opt.
//
// New return a new `cluster` instance immediately
func New(opt option.Options) (c *cluster, done chan struct{}, err error) {
	c = &cluster{}
	done = make(chan struct{})

	c.etcdTimeoutInMilli = time.Duration(opt.EtcdRequestTimeoutInMilli)
	c.name = opt.Name
	MemberConfigKey = MemberConfigPrefix + opt.Name
	StatPipelineFormat = "/runtime/stat/pipelines/%s/" + opt.Name // + pipeline-name
	MemberConfigValue = option.GlobalYAML

	knownMembers := newMembers()
	_ = knownMembers.loadFromFile(filepath.Join(opt.ConfDir, KNOWN_MEMBERS_CFG_FILE))

	switch {
	case opt.ClusterRole == "writer":
		switch {
		case hasLearntMembers(knownMembers):
			opt.ClusterJoinURLs = "do not use when there's known members"
			logger.Infof("etcd member %s start... as peer2peer elector from known members", opt.ClusterClientURL)
			go c.startEtcdSErverAndElection(knownMembers, opt, done)
		case isBoostrapLeader(opt):
			logger.Infof("etcd member %s start... as bootstrap leader, accept other joiners", opt.ClusterClientURL)
			go c.startBootstrapEtcdServer(opt, done)
		default:
			logger.Infof("etcd member %s start... as fresh joiner to the bootstrap leader", opt.ClusterClientURL)
			go c.joinEtcdClusterAndRetry(opt, done)
		}
	case opt.ClusterRole == "reader":
		switch {
		case hasLearntMembers(knownMembers):
			logger.Infof("etcd member %s start... as reader by connecting to known members ", opt.ClusterClientURL)
			go c.subscribe2EtcdclusterByKnownMembers(knownMembers, opt, done)
		default:
			logger.Infof("etcd member %s start... as fresh reader by connecting to bootstrap leaser", opt.ClusterClientURL)
			go c.subscribe2Etcdcluster(opt, done)
		}
	default:
		close(done)
		return nil, done, fmt.Errorf("unknown etcd property opt.CluserRole: %s", opt.ClusterRole)
	}

	c.registerService(opt, done)

	go etcd_heartbeat(c)

	go func(done chan struct{}) {
		<-done
		var canceLearnCtx context.Context
		canceLearnCtx, c.canceLearnEtcdMembers = context.WithCancel(context.Background())
		c.learnEtcdMembers(canceLearnCtx, opt)
	}(done)

	return c, done, nil
}

// isFollower returns `true` if the member is a etcd follower.
// Only a writer can check if it's a `follower` or `boostrapleader`
func isFollower(opt option.Options) bool {
	return opt.ClusterJoinURLs != ""
}

// isBoostrapLeader returns `true` if the member is a etcd boostrap leader.
// A bootstrapLeader will start a new etcd cluster and elect itself as leader, waiting other followers to join.
// Only a writer can check if it's a `follower` or `boostrap leader`.
func isBoostrapLeader(opt option.Options) bool {
	return opt.ClusterJoinURLs == ""
}

// hasLearntMembers returns `true` if the node access the etcd cluster and learned all the members in the etcd cluster.
// The node needn't to depends on the single boostrap leader since it can connect to any of the members.
func hasLearntMembers(knownMembers *members) bool {
	return len(knownMembers.Members) > 0
}

func (c *cluster) createEtcdCluster(opt option.Options, initCluster string, knownMembers *members, done chan struct{}) error {
	var err error
	err = c.createEtcdServer(opt, initCluster, knownMembers)
	if err != nil {
		logger.Errorf("Node %s failed start etcd instance  %s, err: %v",
			opt.ClusterPeerURL, opt.ClusterName, err)
		return err
	}

	err = c.createEtcdClient([]string{opt.ClusterClientURL})
	if err != nil {
		return err
	}

	logger.Infof("Node %s succeeded start etcd instance in cluster %s",
		opt.ClusterPeerURL, opt.ClusterName)
	close(done)
	return nil
}

func (c *cluster) createEtcdServer(opt option.Options, initCluster string, knownMembers *members) error {
	ec, err := generateEtcdConfigFromOption(opt, initCluster)
	if err != nil {
		return err
	}

	c.server, err = embed.StartEtcd(ec)
	if err != nil {
		return err
	}

	<-c.server.Server.ReadyNotify()
	go func() {
		err = <-c.server.Err()
		c.server.Close()
		logger.Errorf("Node %s closed for error: %s", c.server.Config().Name, err.Error())
	}()

	return nil
}

func (c *cluster) createEtcdClient(endpoints []string) error {
	var err error
	c.client, err = clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		AutoSyncInterval:     autoSyncInterval,
		DialTimeout:          dialTimeout,
		DialKeepAliveTime:    dialKeepAliveTime,
		DialKeepAliveTimeout: dialKeepAliveTimeout,
	})

	if err != nil {
		return fmt.Errorf("create etcd client failed: %v", err)
	}

	return nil
}

func newCtx() context.Context {
	return context.Background()
}

func (c *cluster) joinEtcdClusterAndRetry(opt option.Options, done chan struct{}) {
	count := 0
	for {
		var err error
		err = c.joinEtcdCluster(opt)
		if err == nil {
			logger.Infof("Node %s joined cluster %s ", opt.APIAddr, opt.ClusterName)
			close(done)
			return
		}

		count++
		if count%10 == 0 {
			logger.Errorf("Node %s failed to join cluster %s, having retried for %d times ", opt.APIAddr, opt.ClusterName, count)
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *cluster) joinEtcdCluster(opt option.Options) error {
	err := c.createEtcdClient(strings.Split(opt.ClusterJoinURLs, ","))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
	defer cancel()
	listResp, err := c.client.MemberList(ctx)
	if err != nil {
		return err
	}
	pbMembers := listResp.Members

	if !containsMember(listResp.Members, opt.Name) {
		ctx, cancel := context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
		defer cancel()
		addResp, err := c.client.MemberAdd(ctx, []string{opt.ClusterPeerURL})
		if err != nil {
			return err
		}
		pbMembers = addResp.Members
	}

	initCluster := buildInitClusterParam(pbMembers2KnownMembers(pbMembers), opt)
	err = c.createEtcdServer(opt, strings.Join(initCluster, ","), newMembers())
	return err
}

func pbMembers2KnownMembers(pbmembers []*pb.Member) *members {
	members := newMembers()
	for _, m := range pbmembers {
		members.Members = append(members.Members, member{Name: m.Name, PeerListener: strings.Join(m.PeerURLs, ",")})
	}
	return members
}

func buildInitClusterParam(knownMembers *members, opt option.Options) []string {
	initCluster := make([]string, 0)
	for _, member := range knownMembers.Members {
		name := member.Name
		if name == "" {
			name = opt.Name
		}
		initCluster = append(initCluster,
			fmt.Sprintf("%s=%s", name, member.PeerListener))
	}

	return initCluster
}

func containsMember(members []*pb.Member, memberName string) bool {
	for _, member := range members {
		if member.Name == memberName {
			return true
		}
	}

	return false
}

// registerService registers the conf at /runtime/members/config/<memberName>
//
// It will also mark the cluster `started` and ready to serve.
func (c *cluster) registerService(opt option.Options, done chan struct{}) {
	var (
		count = 0
		err   error
	)

	<-done

	for {
		if err != nil && count%10 == 0 {
			logger.Errorf("Failed to register easegateway member %s  at %s, having retried %d times, the last error: %s",
				opt.Name, MemberConfigKey, count, err)
		}

		c.session, err = concurrency.NewSession(c.client,
			concurrency.WithTTL(sessionTTL))
		if err != nil {
			count++
			continue
		}

		value, err := opt.Marshal()
		if err != nil {
			count++
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
		_, err = c.client.Put(ctx, MemberConfigKey, value)
		cancel()
		if err != nil {
			count++
			continue
		}

		atomic.StoreInt32(&c.started, 1)

		logger.Infof("Register easegateway member %s at %s", opt.Name, MemberConfigKey)
		return
	}
}

// Update and save etcd members under opt.ConfLog.
// This function will be run in a job periodically
func (c *cluster) learnEtcdMembers(ctx context.Context, opt option.Options) {
	filename := filepath.Join(opt.ConfDir, KNOWN_MEMBERS_CFG_FILE)
	var knownMembers = newMembers()
	knownMembers.loadFromFile(filename)
	for {
		select {
		case <-ctx.Done():
			logger.Errorf("Node %s is shuting down, abort to update etcd knownMembers", opt.Name)
			break
		case <-time.After(2 * time.Second):
			ctx, cancel := context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
			listResp, err := c.client.MemberList(ctx)
			cancel()
			if err != nil {
				logger.Errorf("Node %s failed to update etcd knownMembers, error: %v", opt.Name, err)
				continue
			}

			var newMembers = newMembers()
			for _, m := range listResp.Members {
				newMembers.Members = append(newMembers.Members, member{Name: m.Name,
					PeerListener: strings.Join(m.PeerURLs, ",")})
			}

			if newMembers.Sum256() == knownMembers.Sum256() {
				continue
			}

			err = newMembers.save2file(filename)
			if err != nil {
				logger.Errorf("Node %s failed to save etcd knownMembers into file %s, err: %v",
					opt.Name, filename, err)
			} else {
				knownMembers = newMembers
				logger.Infof("Node %s saved etcd knownMembers into file %s ",
					opt.Name, filename)
			}
		}
	}
}

func (c *cluster) startEtcdSErverAndElection(knownMembers *members, opt option.Options, done chan struct{}) error {
	initCluster := buildInitClusterParam(knownMembers, opt)
	return c.createEtcdCluster(opt, strings.Join(initCluster, ","), knownMembers, done)
}

func (c *cluster) startBootstrapEtcdServer(opt option.Options, done chan struct{}) error {
	return c.createEtcdCluster(opt, "", newMembers(), done)
}

func (c *cluster) subscribe2Etcdcluster(opt option.Options, done chan struct{}) {
	count := 0
	for {
		err := c.createEtcdClient(strings.Split(opt.ClusterJoinURLs, ","))
		if err == nil {
			close(done)
			return
		}

		count++
		if count%10 == 0 {
			logger.Errorf("etcd client %s failed to connect to the etcd cluster %s,  for %d times.the last error: %s", opt.APIAddr, opt.ClusterName, count, err)
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *cluster) subscribe2EtcdclusterByKnownMembers(knownMembers *members, opt option.Options, done chan struct{}) {
	endpoints := make([]string, 0, 5)
	for _, m := range knownMembers.Members {
		endpoints = append(endpoints, m.PeerListener)
	}

	var err error
	count := 0
	for {
		err = c.createEtcdClient(endpoints)
		if err == nil {
			close(done)
			return
		}

		count++
		if count%10 == 0 {
			logger.Errorf("etcd client %s failed to connect to the etcd cluster %s,  for %d times.the last error: %s", opt.APIAddr, opt.ClusterName, count, err)
		}

		time.Sleep(time.Second)
	}
}

func (c *cluster) Leader() string {

	//a reader has no server
	if c.server == nil {
		return ""
	}

	leaderID := c.server.Server.Leader()
	members := c.server.Server.Cluster().Members()
	for _, member := range members {
		if member.ID == leaderID {
			return member.Name
		}
	}

	return ""
}

func (c *cluster) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	atomic.StoreInt32(&c.started, 0)
	atomic.StoreInt32(&c.closed, 1)

	c.canceLearnEtcdMembers()

	err := c.session.Close()
	if err != nil {
		logger.Errorf("etcd session close faield: %v", err)
	} else {
		logger.Infof("etcd session close successfully")
	}

	c.client.Close()

	//Only a writer owns an etcd server
	if c.server != nil {
		c.server.Close()
		<-c.server.Server.StopNotify()
		logger.Infof("etcd server close successfully")
	}
}

func (c *cluster) Started() bool {
	value := atomic.LoadInt32(&c.started)
	return value != 0
}

func (c *cluster) PurgeMember(member string) error {
	if !c.Started() {
		return fmt.Errorf("cluster not ready yet")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
	defer cancel()
	listResp, err := c.client.MemberList(ctx)
	if err != nil {
		return err
	}

	// todo: refuse to remove etcd member if the node is still active

	var id uint64
	for _, m := range listResp.Members {
		if m.Name == member {
			id = m.ID
			break
		}

	}

	ctx, cancel = context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
	defer cancel()
	_, err = c.client.MemberRemove(ctx, id)
	if err != nil {
		return err
	}

	return nil
}

func (c *cluster) MemberStatus() MemberStatus {
	m := MemberStatus{}
	m.LastHeartbeatTime = time.Now().Unix()
	m.Name = c.name

	if c.server != nil {
		m.Role = "writer"
	} else {
		m.Role = "reader"
	}

	if !c.Started() {
		m.EtcdStatus = "offline"
		return m
	}

	if c.server != nil {
		buff := c.server.Server.SelfStats()
		state := v2stats.ServerStats{}
		yaml.Unmarshal(buff, &state)

		m.Name = state.Name
		m.Id, _ = strconv.ParseUint(state.ID, 16, 64)

		switch state.State.String() {
		case "StateFollower":
			m.EtcdStatus = "follower"
		case "StateLeader":
			m.EtcdStatus = "leader"
		default:
			m.EtcdStatus = "offline"
		}

		// This is a strange workaround, looks `SelfSates` not accurate or delayed
		// SelfSate may show it's follower even it's a leader
		if c.server.Server.Lead() == m.Id {
			m.EtcdStatus = "leader"
		}

	} else {
		ctx, cancel := context.WithTimeout(context.Background(), c.etcdTimeoutInMilli*time.Millisecond)
		defer cancel()
		err := c.client.Sync(ctx)

		if err != nil {
			m.EtcdStatus = "offline"
		} else {
			m.EtcdStatus = "subscriber"
		}
	}

	return m
}

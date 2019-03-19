package cluster

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"go.etcd.io/etcd/embed"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

// Cluster store tree layout.
var (
	MemberConfigPrefix = "/runtime/members/config/"
	MemberConfigKey    = "/runtime/members/config/" + option.Global.Name

	ConfigObjectPrefix           = "/config/objects/"
	ConfigPluginPrefix           = "/config/objects/plugins/"      // + plugin-name
	ConfigPipelinePrefix         = "/config/objects/pipelines/"    // + pipeline-name
	ConfigPluginUsedPrefixFormat = "/config/relation/%s/usedby/"   // + plugin-name
	ConfigPluginUsedKeyFormat    = "/config/relation/%s/usedby/%s" // + plugin-name pipeline-name

	StatPipelinePrefix = "/runtime/stat/pipelines/"
	StatPipelineFormat = "/runtime/stat/pipelines/%s/" + option.Global.Name // + pipeline-name

	MemberConfigValue = option.GlobalYAML
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
	canceLearnEtcdMembers context.CancelFunc
}

//Create a etcd instance and start to serve.
//
// The instance can be a writer or reader, depending on the config in opt.
func NewCluster(opt option.Options) (*cluster, error) {
	var err error
	c := &cluster{}

	knownMembers := newMembers()
	err = knownMembers.loadFromFile(filepath.Join(opt.ConfDir, KNOWN_MEMBERS_CFG_FILE))
	switch {
	case opt.ClusterRole == "writer":
		switch {
		case len(knownMembers.Members) > 0:
			opt.ClusterJoinURLs = "do not use when there's known members"
			logger.Infof("Node %s start... as peer2peer elector from known members", opt.ClusterClientURL)
			err = c.startEtcdSErverAndElection(knownMembers, opt)
		case opt.ClusterJoinURLs == "":
			logger.Infof("Node %s start... as bootstrap leader, accept other joiners", opt.ClusterClientURL)
			err = c.startBootstrapEtcdServer(opt)
		case opt.ClusterJoinURLs != "":
			logger.Infof("Node %s start... as fresh joiner to the bootstrap leader", opt.ClusterClientURL)
			err = c.joinEtcdClusterAndRetry(opt, 30)
		}
	case opt.ClusterRole == "reader":
		switch {
		case len(knownMembers.Members) > 0:
			logger.Infof("Node %s start... as reader by connecting to known members ", opt.ClusterClientURL)
			err = c.subscribe2EtcdclusterByKnownMembers(knownMembers, opt)
		case len(knownMembers.Members) == 0:
			logger.Infof("Node %s start... as fresh reader by connecting to bootstrap leaser", opt.ClusterClientURL)
			err = c.subscribe2Etcdcluster(opt)
		}
	default:
		return nil, fmt.Errorf("unknown opt.CluserRole: %s", opt.ClusterRole)
	}

	if err != nil {
		return nil, err
	}

	err = c.registerService(opt)
	if err != nil {
		return nil, err
	}

	go func() {
		var canceLearnCtx context.Context
		canceLearnCtx, c.canceLearnEtcdMembers = context.WithCancel(context.Background())
		c.learnEtcdMembers(canceLearnCtx, opt)
	}()

	return c, nil
}

func (c *cluster) createEtcdCluster(opt option.Options, initCluster string, knownMembers *members) error {
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
	return nil
}

func (c *cluster) createEtcdServer(opt option.Options, initCluster string, knownMembers *members) error {
	ec, err := generateEtcdConfigFromOption(opt, initCluster, knownMembers)
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
		c.canceLearnEtcdMembers()
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

func ctx() context.Context {
	return context.Background()
}
func (c *cluster) joinEtcdClusterAndRetry(opt option.Options, retries int) error {
	var err error
	err = c.joinEtcdCluster(opt)
	if err == nil {
		logger.Infof("Node %s joined cluster %s ", opt.ClusterClientURL, opt.ClusterName)
		return nil
	}

	logger.Errorf("Node %s failed to join cluster %s ", opt.ClusterClientURL, opt.ClusterName)

	if retries > 0 {
		time.Sleep(time.Second)
		return c.joinEtcdClusterAndRetry(opt, retries-1)
	}

	return err
}

func (c *cluster) joinEtcdCluster(opt option.Options) error {
	err := c.createEtcdClient(strings.Split(opt.ClusterJoinURLs, ","))
	if err != nil {
		return err
	}

	listResp, err := c.client.MemberList(ctx())
	if err != nil {
		return err
	}
	pbMembers := listResp.Members

	memberName, err := generateMemberName(opt.ClusterPeerURL)
	if !containsMember(listResp.Members, memberName) {
		addResp, err := c.client.MemberAdd(ctx(), []string{opt.ClusterPeerURL})
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
	initCluster := make([]string, 0, 5)
	for _, member := range knownMembers.Members {
		name := member.Name
		if name == "" {
			name, _ = generateMemberName(opt.ClusterPeerURL)
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

// Register the conf at /runtime/members/config/<memberName>
func (c *cluster) registerService(opt option.Options) error {
	var err error
	c.session, err = concurrency.NewSession(c.client,
		concurrency.WithTTL(sessionTTL))
	if err != nil {
		return err
	}

	value, err := opt.Marshal()
	if err != nil {
		return err
	}

	memberName, err := generateMemberName(opt.ClusterPeerURL)
	if err != nil {
		return err
	}

	_, err = c.client.Put(context.Background(), MemberConfigKey+memberName, value)
	if err != nil {
		logger.Errorf("Failed to register easegateway member %s  at %s",
			memberName, MemberConfigKey)
		return err
	}

	logger.Infof("Register easegateway member %s at %s", memberName, MemberConfigKey)
	return nil
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
			listCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			listResp, err := c.client.MemberList(listCtx)
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

			if newMembers.Md5() == knownMembers.Md5() {
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

func (c *cluster) startEtcdSErverAndElection(knownMembers *members, opt option.Options) error {
	initCluster := buildInitClusterParam(knownMembers, opt)
	return c.createEtcdCluster(opt, strings.Join(initCluster, ","), knownMembers)
}

func (c *cluster) startBootstrapEtcdServer(opt option.Options) error {
	return c.createEtcdCluster(opt, "", newMembers())
}

func (c *cluster) subscribe2Etcdcluster(opt option.Options) error {
	var err error
	for i := 0; i < 30; i++ {
		err = c.createEtcdClient(strings.Split(opt.ClusterJoinURLs, ","))
		if err == nil {
			return nil
		} else {
			time.Sleep(time.Second)
		}
	}
	return err
}

func (c *cluster) subscribe2EtcdclusterByKnownMembers(knownMembers *members, options option.Options) error {
	endpoints := make([]string, 0, 5)
	for _, m := range knownMembers.Members {
		endpoints = append(endpoints, m.PeerListener)
	}

	var err error
	for i := 0; i < 30; i++ {
		err = c.createEtcdClient(endpoints)
		if err == nil {
			return nil
		} else {
			time.Sleep(time.Second)
		}
	}

	return err
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

	atomic.StoreInt32(&c.closed, 1)

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

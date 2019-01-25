package cluster

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"go.etcd.io/etcd/embed"
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

func ctx() context.Context {
	return context.Background()
}

type Cluster interface {
	Get(key string) (*string, error)
	GetPrefix(prefix string) (map[string]string, error)

	Put(key, value string) error
	// The lease may be expired or revoked, it's callers' duty to
	// care the situation.
	PutUnderLease(key, value string) error
	PutAndDelete(map[string]*string) error
	PutAndDeleteUnderLease(map[string]*string) error

	Delete(key string) error
	DeletePrefix(prefix string) error

	// Currently we doesn't support to cancel watch.
	Watch(key string) (<-chan *string, error)
	WatchPrefix(prefix string) (<-chan map[string]*string, error)

	Mutex(name string, timeout time.Duration) Mutex

	Leader() string
	Close(wg *sync.WaitGroup)
}

type cluster struct {
	server  *embed.Etcd
	client  *clientv3.Client
	session *concurrency.Session
	closed  int32
}

func New() (Cluster, error) {
	var err error
	c := &cluster{}
	if option.Global.ClusterRole == "writer" {
		if option.Global.ClusterJoinURL == "" {
			c.server, c.client, err = createEtcdCluster()
		} else {
			c.server, c.client, err = joinEtcdCluster()
		}
		if err != nil {
			return nil, err
		}
	} else {
		c.client, err = createEtcdClient([]string{option.Global.ClusterJoinURL})
		if err != nil {
			return nil, err
		}
	}

	err = c.registerService()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func createEtcdCluster() (*embed.Etcd, *clientv3.Client, error) {
	server, err := createEtcdServer("")
	if err != nil {
		return nil, nil, err
	}

	client, err := createEtcdClient([]string{option.Global.ClusterClientURL})
	if err != nil {
		return nil, nil, err
	}

	return server, client, nil
}

func joinEtcdCluster() (*embed.Etcd, *clientv3.Client, error) {
	client, err := createEtcdClient([]string{option.Global.ClusterJoinURL})
	if err != nil {
		return nil, nil, err
	}

	listResp, err := client.MemberList(ctx())
	if err != nil {
		return nil, nil, err
	}
	for _, member := range listResp.Members {
		if member.Name == option.Global.Name {
			_, err := client.MemberRemove(ctx(), member.ID)
			if err != nil {
				return nil, nil, err
			}
			break
		}
	}

	addResp, err := client.MemberAdd(ctx(),
		[]string{option.Global.ClusterPeerURL})
	if err != nil {
		return nil, nil, err
	}
	for _, member := range addResp.Members {
		if addResp.Member.ID == member.ID {
			member.Name = option.Global.Name
			break
		}
	}

	var initCluster []string
	for _, member := range addResp.Members {
		for _, u := range member.PeerURLs {
			initCluster = append(initCluster,
				fmt.Sprintf("%s=%s", member.Name, u))
		}
	}

	server, err := createEtcdServer(strings.Join(initCluster, ","))
	if err != nil {
		return nil, nil, err
	}

	return server, client, err
}

func createEtcdServer(initCluster string) (*embed.Etcd, error) {
	ec := generateEtcdConfig(initCluster)
	server, err := embed.StartEtcd(ec)
	if err != nil {
		return nil, err
	}

	<-server.Server.ReadyNotify()
	go func() {
		err := <-server.Err()
		logger.Errorf("etcd server run failed: %v", err)
		server.Close()
	}()
	logger.Infof("etcd server started successfully")

	return server, nil
}

func createEtcdClient(endpoints []string) (*clientv3.Client, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		AutoSyncInterval:     autoSyncInterval,
		DialTimeout:          dialTimeout,
		DialKeepAliveTime:    dialKeepAliveTime,
		DialKeepAliveTimeout: dialKeepAliveTimeout,
	})

	if err != nil {
		return nil, fmt.Errorf("create etcd client failed: %v", err)
	}

	return client, nil
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

	closeClient := func() {
		err = c.client.Close()
		if err != nil {
			logger.Errorf("etcd client close faield: %v", err)
		} else {
			logger.Infof("etcd client close successfully")
		}
	}

	if c.server != nil {
		listResp, err := c.client.MemberList(ctx())
		if err != nil {
			logger.Errorf("list member failed: %v", err)
		} else if len(listResp.Members) > 1 {
			_, err := c.client.MemberRemove(ctx(), uint64(c.server.Server.ID()))
			if err != nil {
				logger.Errorf("remove member %s failed: %v",
					option.Global.Name, err)
			}
		}
		closeClient()
		c.server.Close()
		<-c.server.Server.StopNotify()
		logger.Infof("etcd server close successfully")
	} else {
		closeClient()
	}
}

// key: /runtime/meta/member-name
func (c *cluster) registerService() error {
	register := func() error {
		var err error
		c.session, err = concurrency.NewSession(c.client,
			concurrency.WithTTL(sessionTTL))
		if err != nil {
			return err
		}

		err = c.PutUnderLease(MemberConfigKey, MemberConfigValue)
		if err != nil {
			return err
		}

		logger.Infof("register service at %s", MemberConfigKey)
		return nil
	}

	err := register()
	if err != nil {
		return err
	}

	go func() {
		for {
			<-c.session.Done()
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}

			logger.Errorf("lease %016x expired or revoked", c.session.Lease())

			for {
				err := register()
				if err == nil {
					break
				}
				logger.Errorf("register service failed(retry after %v): %v",
					registerTimeout, err)
				<-time.After(registerTimeout)
			}
		}
	}()

	return nil
}

func (c *cluster) Leader() string {
	leaderID := c.server.Server.Leader()
	members := c.server.Server.Cluster().Members()
	for _, member := range members {
		if member.ID == leaderID {
			return member.Name
		}
	}

	return ""
}

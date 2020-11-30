package ZookeeperServiceRegistry

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/serviceregistry"
	"github.com/megaease/easegateway/pkg/scheduler"

	zookeeper "github.com/go-zookeeper/zk"
)

const (
	// Kind is ZookeeperServiceRegistry kind.
	Kind = "ZookeeperServiceRegistry"
)

func init() {
	scheduler.Register(&scheduler.ObjectRecord{
		Kind:              Kind,
		DefaultSpecFunc:   DefaultSpec,
		NewFunc:           New,
		DependObjectKinds: nil,
	})
}

type (
	// ZookeeperServiceRegistry is Object ZookeeperServiceRegistry.
	ZookeeperServiceRegistry struct {
		spec *Spec

		connMutex sync.RWMutex
		conn      *zookeeper.Conn

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the ZookeeperServiceRegistry.
	Spec struct {
		scheduler.ObjectMeta `yaml:",inline"`

		ConnTimeout  string   `yaml:"conntimeout" jsonschema:"required,format=duration"`
		ZKServices   []string `yaml:"zkservices" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `yaml:"prefix" jsonschema:"required,pattern=^/"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of ZookeeperServiceRegistry.
	Status struct {
		Timestamp  int64          `yaml:"timestamp"`
		Health     string         `yaml:"health"`
		ServersNum map[string]int `yaml:"serversNum"`
	}
)

// DefaultSpec returns ZookeeperServiceRegistry default spec.
func DefaultSpec() *Spec {
	return &Spec{
		ZKServices:   []string{"127.0.0.1:2181"},
		SyncInterval: "10s",
		Prefix:       "/",
		ConnTimeout:  "6s",
	}
}

// Validate validates Spec.
func (spec Spec) Validate() error {
	return nil
}

// New creates an ZookeeperServiceRegistry.
func New(spec *Spec, prev *ZookeeperServiceRegistry, handlers *sync.Map) *ZookeeperServiceRegistry {
	zk := &ZookeeperServiceRegistry{
		spec:       spec,
		serversNum: map[string]int{},
		done:       make(chan struct{}),
	}
	if prev != nil {
		prev.Close()
	}

	_, err := zk.getConn()
	if err != nil {
		logger.Errorf("%s get zookeeper conn failed: %v", spec.Name, err)
	}

	go zk.run()

	return zk
}

func (zk *ZookeeperServiceRegistry) getConn() (*zookeeper.Conn, error) {
	zk.connMutex.RLock()
	if zk.conn != nil {
		conn := zk.conn
		zk.connMutex.RUnlock()
		return conn, nil
	}
	zk.connMutex.RUnlock()

	return zk.buildConn()
}

func (zk *ZookeeperServiceRegistry) buildConn() (*zookeeper.Conn, error) {
	zk.connMutex.Lock()
	defer zk.connMutex.Unlock()

	// DCL
	if zk.conn != nil {
		return zk.conn, nil
	}

	conntimeout, err := time.ParseDuration(zk.spec.ConnTimeout)
	if err != nil {
		logger.Errorf("BUG: parse connection timeout duration %s failed: %v",
			zk.spec.ConnTimeout, err)
		return nil, err
	}

	conn, _, err := zookeeper.Connect(zk.spec.ZKServices, conntimeout)

	if err != nil {
		logger.Errorf("zookeeper get connection failed: %v", err)
		return nil, err
	}

	exist, _, err := conn.Exists(zk.spec.Prefix)
	if err != nil {
		logger.Errorf("zookeeper check path: %s exist failed: %v", zk.spec.Prefix, err)
		return nil, err
	}

	if exist == false {
		logger.Errorf("zookeeper path: %s no exist", zk.spec.Prefix)
		return nil, fmt.Errorf("path [%s] no exist", zk.spec.Prefix)
	}

	zk.conn = conn

	return conn, nil
}

func (zk *ZookeeperServiceRegistry) closeConn() {
	zk.connMutex.Lock()
	defer zk.connMutex.Unlock()

	if zk.conn == nil {
		return
	}

	zk.conn.Close()
}

func (zk *ZookeeperServiceRegistry) run() {
	syncInterval, err := time.ParseDuration(zk.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			zk.spec.SyncInterval, err)
		return
	}

	zk.update()

	for {
		select {
		case <-zk.done:
			return
		case <-time.After(syncInterval):
			zk.update()
		}
	}
}

func (zk *ZookeeperServiceRegistry) update() {
	conn, err := zk.getConn()
	if err != nil {
		logger.Errorf("%s get zookeeper conn failed: %v",
			zk.spec.Name, err)
		return
	}

	childs, _, err := conn.Children(zk.spec.Prefix)

	if err != nil {
		logger.Errorf("%s get path: %s children failed: %v", zk.spec.Name, zk.spec.Prefix, err)
		return
	}

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
	for _, child := range childs {

		fullPath := zk.spec.Prefix + "/" + child
		data, _, err := conn.Get(fullPath)

		if err != nil {
			if err == zookeeper.ErrNoNode {
				continue
			}

			logger.Errorf("%s get child path %s failed: %v", zk.spec.Name, fullPath, err)
			return
		}

		server := new(serviceregistry.Server)
		// Note: zookeeper allows store custom format into one path, so we choose to store
		//       serviceregistry.Server JSON format directly.
		err = json.Unmarshal(data, server)
		if err != nil {
			logger.Errorf("BUG %s unmarshal fullpath %s failed %v", zk.spec.Name, fullPath, err)
			return
		}
		logger.Debugf("zk %s fullpath %s server is  %v", zk.spec.Name, fullPath, server)
		serversNum[fullPath]++
		servers = append(servers, server)
	}

	serviceregistry.Global.ReplaceServers(zk.spec.Name, servers)

	zk.statusMutex.Lock()
	zk.serversNum = serversNum
	zk.statusMutex.Unlock()
}

// Status returns status of EurekaServiceRegister.
func (zk *ZookeeperServiceRegistry) Status() *Status {
	s := &Status{Health: "ready"}

	_, err := zk.getConn()
	if err != nil {
		s.Health = err.Error()
	}

	zk.statusMutex.Lock()
	s.ServersNum = zk.serversNum
	zk.statusMutex.Unlock()

	return s
}

// Close closes ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Close() {
	zk.closeConn()
	close(zk.done)

	serviceregistry.Global.CloseRegistry(zk.spec.Name)
}

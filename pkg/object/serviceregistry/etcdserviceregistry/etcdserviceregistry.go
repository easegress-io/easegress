package etcdserviceregistry

import (
	"context"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/serviceregistry"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/scheduler"

	"go.etcd.io/etcd/clientv3"
)

const (
	// Kind is EtcdServiceRegistry kind.
	Kind = "EtcdServiceRegistry"
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
	// EtcdServiceRegistry is Object EtcdServiceRegistry.
	EtcdServiceRegistry struct {
		spec *Spec

		clientMutex sync.RWMutex
		client      *clientv3.Client

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the EtcdServiceRegistry.
	Spec struct {
		scheduler.ObjectMeta `yaml:",inline"`

		Endpoints    []string `yaml:"endpoints" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `yaml:"prefix" jsonschema:"required,pattern=^/"`
		CacheTimeout string   `yaml:"cacheTimeout" jsonschema:"required,format=duration"`
	}

	// Status is the status of EtcdServiceRegistry.
	Status struct {
		Timestamp  int64          `yaml:"timestamp"`
		Health     string         `yaml:"health"`
		ServersNum map[string]int `yaml:"serversNum"`
	}
)

// DefaultSpec returns EtcdServiceRegistry default spec.
func DefaultSpec() *Spec {
	return &Spec{
		Prefix:       "/services/",
		CacheTimeout: "60s",
	}
}

// Validate validates Spec.
func (spec Spec) Validate() error {
	return nil
}

// New creates an EtcdServiceRegistry.
func New(spec *Spec, prev *EtcdServiceRegistry, handlers *sync.Map) *EtcdServiceRegistry {
	etcd := &EtcdServiceRegistry{
		spec:       spec,
		serversNum: map[string]int{},
		done:       make(chan struct{}),
	}
	if prev != nil {
		prev.Close()
	}

	_, err := etcd.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v", spec.Name, err)
	}

	go etcd.run()

	return etcd
}

func (etcd *EtcdServiceRegistry) getClient() (*clientv3.Client, error) {
	etcd.clientMutex.RLock()
	if etcd.client != nil {
		client := etcd.client
		etcd.clientMutex.RUnlock()
		return client, nil
	}
	etcd.clientMutex.RUnlock()

	return etcd.buildClient()
}

func (etcd *EtcdServiceRegistry) buildClient() (*clientv3.Client, error) {
	etcd.clientMutex.Lock()
	defer etcd.clientMutex.Unlock()

	// DCL
	if etcd.client != nil {
		return etcd.client, nil
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:            etcd.spec.Endpoints,
		AutoSyncInterval:     1 * time.Minute,
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Minute,
		DialKeepAliveTimeout: 1 * time.Minute,
		LogConfig:            logger.EtcdClientLoggerConfig(option.Global, "object_"+etcd.spec.Name),
	})

	if err != nil {
		return nil, err
	}

	etcd.client = client

	return client, nil
}

func (etcd *EtcdServiceRegistry) closeClient() {
	etcd.clientMutex.Lock()
	defer etcd.clientMutex.Unlock()

	if etcd.client == nil {
		return
	}
	err := etcd.client.Close()
	if err != nil {
		logger.Errorf("%s close etdc client failed: %v", etcd.spec.Name, err)
	}
	etcd.client = nil
}

func (etcd *EtcdServiceRegistry) run() {
	cacheTimeout, err := time.ParseDuration(etcd.spec.CacheTimeout)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			etcd.spec.CacheTimeout, err)
		return
	}

	etcd.update()

	for {
		select {
		case <-etcd.done:
			return
		case <-time.After(cacheTimeout):
			etcd.update()
		}
	}
}

func (etcd *EtcdServiceRegistry) update() {
	client, err := etcd.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v",
			etcd.spec.Name, err)
		return
	}
	resp, err := client.Get(context.Background(), etcd.spec.Prefix, clientv3.WithPrefix())
	if err != nil {
		logger.Errorf("%s pull services failed: %v",
			etcd.spec.Name, err)
		return
	}

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
	for _, kv := range resp.Kvs {
		server := &serviceregistry.Server{}
		err := yaml.Unmarshal(kv.Value, server)
		if err != nil {
			logger.Errorf("%s: unmarshal %s to yaml failed: %v",
				kv.Key, kv.Value, err)
			continue
		}
		if err := server.Validate(); err != nil {
			logger.Errorf("%s is invalid: %v", kv.Value, err)
			continue
		}

		servers = append(servers, server)
		serversNum[server.ServiceName]++
	}

	serviceregistry.Global.ReplaceServers(etcd.spec.Name, servers)

	etcd.statusMutex.Lock()
	etcd.serversNum = serversNum
	etcd.statusMutex.Unlock()
}

// Status returns status of EtcdServiceRegister.
func (etcd *EtcdServiceRegistry) Status() *Status {
	s := &Status{}

	_, err := etcd.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	etcd.statusMutex.Lock()
	serversNum := etcd.serversNum
	etcd.statusMutex.Unlock()

	s.ServersNum = serversNum

	return s
}

// Close closes EtcdServiceRegistry.
func (etcd *EtcdServiceRegistry) Close() {
	etcd.closeClient()
	close(etcd.done)

	serviceregistry.Global.CloseRegistry(etcd.spec.Name)
}

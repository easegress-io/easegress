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
	esr := &EtcdServiceRegistry{
		spec:       spec,
		serversNum: map[string]int{},
		done:       make(chan struct{}),
	}
	if prev != nil {
		prev.Close()
	}

	_, err := esr.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v", spec.Name, err)
	}

	go esr.run()

	return esr
}

func (esr *EtcdServiceRegistry) getClient() (*clientv3.Client, error) {
	esr.clientMutex.RLock()
	if esr.client != nil {
		client := esr.client
		esr.clientMutex.RUnlock()
		return client, nil
	}
	esr.clientMutex.RUnlock()

	return esr.buildClient()
}

func (esr *EtcdServiceRegistry) buildClient() (*clientv3.Client, error) {
	esr.clientMutex.Lock()
	defer esr.clientMutex.Unlock()

	// DCL
	if esr.client != nil {
		return esr.client, nil
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:            esr.spec.Endpoints,
		AutoSyncInterval:     1 * time.Minute,
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Minute,
		DialKeepAliveTimeout: 1 * time.Minute,
		LogConfig:            logger.EtcdClientLoggerConfig(option.Global, "object_"+esr.spec.Name),
	})

	if err != nil {
		return nil, err
	}

	esr.client = client

	return client, nil
}

func (esr *EtcdServiceRegistry) closeClient() {
	esr.clientMutex.Lock()
	defer esr.clientMutex.Unlock()

	if esr.client == nil {
		return
	}
	err := esr.client.Close()
	if err != nil {
		logger.Errorf("%s close etdc client failed: %v", esr.spec.Name, err)
	}
	esr.client = nil
}

func (esr *EtcdServiceRegistry) run() {
	cacheTimeout, err := time.ParseDuration(esr.spec.CacheTimeout)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			esr.spec.CacheTimeout, err)
		return
	}

	esr.update()

	for {
		select {
		case <-esr.done:
			return
		case <-time.After(cacheTimeout):
			esr.update()
		}
	}
}

func (esr *EtcdServiceRegistry) update() {
	client, err := esr.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v",
			esr.spec.Name, err)
		return
	}
	resp, err := client.Get(context.Background(), esr.spec.Prefix, clientv3.WithPrefix())
	if err != nil {
		logger.Errorf("%s pull services failed: %v",
			esr.spec.Name, err)
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

	serviceregistry.Global.ReplaceServers(esr.spec.Name, servers)

	esr.statusMutex.Lock()
	esr.serversNum = serversNum
	esr.statusMutex.Unlock()
}

// Status returns status of EtcdServiceRegister.
func (esr *EtcdServiceRegistry) Status() *Status {
	s := &Status{}

	_, err := esr.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	esr.statusMutex.Lock()
	serversNum := esr.serversNum
	esr.statusMutex.Unlock()

	s.ServersNum = serversNum

	return s
}

// Close closes EtcdServiceRegistry.
func (esr *EtcdServiceRegistry) Close() {
	esr.closeClient()
	close(esr.done)

	serviceregistry.Global.CloseRegistry(esr.spec.Name)
}

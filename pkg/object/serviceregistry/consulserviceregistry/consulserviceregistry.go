package consulserviceregistry

import (
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/serviceregistry"
	"github.com/megaease/easegateway/pkg/supervisor"

	"github.com/hashicorp/consul/api"
)

const (
	// Category is the category of ConsulServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of ConsulServiceRegistry.
	Kind = "ConsulServiceRegistry"
)

func init() {
	supervisor.Register(&ConsulServiceRegistry{})
}

type (
	// ConsulServiceRegistry is Object ConsulServiceRegistry.
	ConsulServiceRegistry struct {
		spec *Spec

		clientMutex sync.RWMutex
		client      *api.Client

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the ConsulServiceRegistry.
	Spec struct {
		supervisor.ObjectMetaSpec `yaml:",inline"`

		Address      string   `yaml:"address" jsonschema:"required"`
		Scheme       string   `yaml:"scheme" jsonschema:"omitempty,enum=http,enum=https"`
		Datacenter   string   `yaml:"datacenter" jsonschema:"omitempty"`
		Token        string   `yaml:"token" jsonschema:"omitempty"`
		Namespace    string   `yaml:"namespace" jsonschema:"omitempty"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
		ServiceTags  []string `yaml:"serviceTags" jsonschema:"omitempty"`
	}

	// Status is the status of ConsulServiceRegistry.
	Status struct {
		Health     string         `yaml:"health"`
		ServersNum map[string]int `yaml:"serversNum"`
	}
)

// Category returns the category of ConsulServiceRegistry.
func (csr *ConsulServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of ConsulServiceRegistry.
func (csr *ConsulServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of ConsulServiceRegistry.
func (csr *ConsulServiceRegistry) DefaultSpec() supervisor.ObjectSpec {
	return &Spec{
		Address:      "127.0.0.1:8500",
		Scheme:       "http",
		SyncInterval: "10s",
	}
}

// Renew renews ConsulServiceRegistry.
func (csr *ConsulServiceRegistry) Renew(spec supervisor.ObjectSpec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	if previousGeneration != nil {
		previousGeneration.Close()
	}

	csr.spec = spec.(*Spec)
	csr.serversNum = map[string]int{}
	csr.done = make(chan struct{})

	_, err := csr.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v", csr.spec.Name, err)
	}

	go csr.run()
}

func (csr *ConsulServiceRegistry) getClient() (*api.Client, error) {
	csr.clientMutex.RLock()
	if csr.client != nil {
		client := csr.client
		csr.clientMutex.RUnlock()
		return client, nil
	}
	csr.clientMutex.RUnlock()

	return csr.buildClient()
}

func (csr *ConsulServiceRegistry) buildClient() (*api.Client, error) {
	csr.clientMutex.Lock()
	defer csr.clientMutex.Unlock()

	// DCL
	if csr.client != nil {
		return csr.client, nil
	}

	config := api.DefaultConfig()
	config.Address = csr.spec.Address
	if config.Scheme != "" {
		config.Scheme = csr.spec.Scheme
	}
	if config.Datacenter != "" {
		config.Datacenter = csr.spec.Datacenter
	}
	if config.Token != "" {
		config.Token = csr.spec.Token
	}
	if config.Namespace != "" {
		config.Namespace = csr.spec.Namespace
	}

	client, err := api.NewClient(config)

	if err != nil {
		return nil, err
	}

	csr.client = client

	return client, nil
}

func (csr *ConsulServiceRegistry) closeClient() {
	csr.clientMutex.Lock()
	defer csr.clientMutex.Unlock()

	if csr.client == nil {
		return
	}

	csr.client = nil
}

func (csr *ConsulServiceRegistry) run() {
	syncInterval, err := time.ParseDuration(csr.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			csr.spec.SyncInterval, err)
		return
	}

	csr.update()

	for {
		select {
		case <-csr.done:
			return
		case <-time.After(syncInterval):
			csr.update()
		}
	}
}

func (csr *ConsulServiceRegistry) update() {
	client, err := csr.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v",
			csr.spec.Name, err)
		return
	}

	q := &api.QueryOptions{
		Namespace:  csr.spec.Namespace,
		Datacenter: csr.spec.Datacenter,
	}
	catalog := client.Catalog()

	resp, _, err := catalog.Services(q)
	if err != nil {
		logger.Errorf("%s pull catalog services failed: %v",
			csr.spec.Name, err)
		return
	}

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
	for serviceName := range resp {
		services, _, err := catalog.ServiceMultipleTags(serviceName,
			csr.spec.ServiceTags, q)
		if err != nil {
			logger.Errorf("%s pull catalog service %s failed: %v",
				csr.spec.Name, serviceName, err)
			continue
		}
		for _, service := range services {
			server := &serviceregistry.Server{
				ServiceName: serviceName,
			}
			server.HostIP = service.ServiceAddress
			if server.HostIP == "" {
				server.HostIP = service.Address
			}
			server.Port = int16(service.ServicePort)
			server.Tags = service.ServiceTags

			if err := server.Validate(); err != nil {
				logger.Errorf("invalid server: %v", err)
				continue
			}

			servers = append(servers, server)
			serversNum[serviceName]++
		}
	}

	serviceregistry.Global.ReplaceServers(csr.spec.Name, servers)

	csr.statusMutex.Lock()
	csr.serversNum = serversNum
	csr.statusMutex.Unlock()
}

// Status returns status of ConsulServiceRegister.
func (csr *ConsulServiceRegistry) Status() interface{} {
	s := &Status{}

	if csr.spec == nil {
		return s
	}

	_, err := csr.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	csr.statusMutex.Lock()
	serversNum := csr.serversNum
	csr.statusMutex.Unlock()

	s.ServersNum = serversNum

	return s
}

// Close closes ConsulServiceRegistry.
func (csr *ConsulServiceRegistry) Close() {
	csr.closeClient()
	close(csr.done)

	serviceregistry.Global.CloseRegistry(csr.spec.Name)
}

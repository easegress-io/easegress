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
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		clientMutex sync.RWMutex
		client      *api.Client

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the ConsulServiceRegistry.
	Spec struct {
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
func (c *ConsulServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Address:      "127.0.0.1:8500",
		Scheme:       "http",
		SyncInterval: "10s",
	}
}

// Init initilizes ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	c.superSpec, c.spec, c.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	c.reload()
}

// Inherit inherits previous generation of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Inherit(superSpec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	c.Init(superSpec, super)
}

func (c *ConsulServiceRegistry) reload() {
	c.serversNum = map[string]int{}
	c.done = make(chan struct{})

	_, err := c.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v", c.superSpec.Name(), err)
	}

	go c.run()
}

func (c *ConsulServiceRegistry) getClient() (*api.Client, error) {
	c.clientMutex.RLock()
	if c.client != nil {
		client := c.client
		c.clientMutex.RUnlock()
		return client, nil
	}
	c.clientMutex.RUnlock()

	return c.buildClient()
}

func (c *ConsulServiceRegistry) buildClient() (*api.Client, error) {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	// DCL
	if c.client != nil {
		return c.client, nil
	}

	config := api.DefaultConfig()
	config.Address = c.spec.Address
	if config.Scheme != "" {
		config.Scheme = c.spec.Scheme
	}
	if config.Datacenter != "" {
		config.Datacenter = c.spec.Datacenter
	}
	if config.Token != "" {
		config.Token = c.spec.Token
	}
	if config.Namespace != "" {
		config.Namespace = c.spec.Namespace
	}

	client, err := api.NewClient(config)

	if err != nil {
		return nil, err
	}

	c.client = client

	return client, nil
}

func (c *ConsulServiceRegistry) closeClient() {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.client == nil {
		return
	}

	c.client = nil
}

func (c *ConsulServiceRegistry) run() {
	syncInterval, err := time.ParseDuration(c.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			c.spec.SyncInterval, err)
		return
	}

	c.update()

	for {
		select {
		case <-c.done:
			return
		case <-time.After(syncInterval):
			c.update()
		}
	}
}

func (c *ConsulServiceRegistry) update() {
	client, err := c.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v",
			c.superSpec.Name(), err)
		return
	}

	q := &api.QueryOptions{
		Namespace:  c.spec.Namespace,
		Datacenter: c.spec.Datacenter,
	}
	catalog := client.Catalog()

	resp, _, err := catalog.Services(q)
	if err != nil {
		logger.Errorf("%s pull catalog services failed: %v",
			c.superSpec.Name(), err)
		return
	}

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
	for serviceName := range resp {
		services, _, err := catalog.ServiceMultipleTags(serviceName,
			c.spec.ServiceTags, q)
		if err != nil {
			logger.Errorf("%s pull catalog service %s failed: %v",
				c.superSpec.Name(), serviceName, err)
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
			server.Port = uint16(service.ServicePort)
			server.Tags = service.ServiceTags

			if err := server.Validate(); err != nil {
				logger.Errorf("invalid server: %v", err)
				continue
			}

			servers = append(servers, server)
			serversNum[serviceName]++
		}
	}

	serviceregistry.Global.ReplaceServers(c.superSpec.Name(), servers)

	c.statusMutex.Lock()
	c.serversNum = serversNum
	c.statusMutex.Unlock()
}

// Status returns status of ConsulServiceRegister.
func (c *ConsulServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := c.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	c.statusMutex.Lock()
	serversNum := c.serversNum
	c.statusMutex.Unlock()

	s.ServersNum = serversNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Close() {
	c.closeClient()
	close(c.done)

	serviceregistry.Global.CloseRegistry(c.superSpec.Name())
}

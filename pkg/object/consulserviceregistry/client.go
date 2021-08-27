package consulserviceregistry

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

type (
	// consulClient is the common client interface for consul.
	consulClient interface {
		ServiceRegister(registration *api.AgentServiceRegistration) error
		ServiceDeregister(instanceID string) error
		ListServiceInstances(serviceName string) ([]*api.CatalogService, error)
		ListAllServiceInstances() ([]*api.CatalogService, error)
	}

	consulAPIClient struct {
		client *api.Client
	}
)

func newConsulAPIClient(client *api.Client) *consulAPIClient {
	return &consulAPIClient{
		client: client,
	}
}

func (c *consulAPIClient) ServiceRegister(registration *api.AgentServiceRegistration) error {
	return c.client.Agent().ServiceRegister(registration)
}

func (c *consulAPIClient) ServiceDeregister(instanceID string) error {
	return c.client.Agent().ServiceDeregister(instanceID)
}

func (c *consulAPIClient) ListServiceInstances(serviceName string) ([]*api.CatalogService, error) {
	resp, _, err := c.client.Catalog().Service(serviceName, "", &api.QueryOptions{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *consulAPIClient) ListAllServiceInstances() ([]*api.CatalogService, error) {
	resp, _, err := c.client.Catalog().Services(&api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("pull catalog services failed: %v", err)
	}

	catalogServices := []*api.CatalogService{}
	for serviceName := range resp {
		services, _, err := c.client.Catalog().Service(serviceName, "", &api.QueryOptions{})
		if err != nil {
			return nil, fmt.Errorf("pull catalog service %s failed: %v", serviceName, err)
		}

		for _, service := range services {
			catalogServices = append(catalogServices, service)
		}
	}

	return catalogServices, nil
}

package spec

import (
	"fmt"
	"testing"

	"github.com/megaease/easegateway/pkg/filter/backend"
)

func TestIngressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: backend.PolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	superSpec := s.IngressPipelineSpec(443)
	fmt.Println(superSpec.YAMLConfig())
}

func TestEgressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: backend.PolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
		},
		{
			ServiceName: "fake-002",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
		},
	}

	superSpec := s.EgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

package spec

import (
	"fmt"
	"testing"

	"github.com/megaease/easegateway/pkg/filter/backend"
	"github.com/megaease/easegateway/pkg/util/httpfilter"
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
			Status:      "UP",
		},
		{
			ServiceName: "fake-002",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
		},
	}

	superSpec := s.EgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestEgressPipelineWithCanarySpec(t *testing.T) {
	s := &Service{
		Name: "order-002-canary",
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
		Canary: &Canary{
			CanaryRules: []*CanaryRule{
				{
					Filter: &httpfilter.Spec{
						Headers: map[string]*httpfilter.ValueFilter{
							"X-canary": &httpfilter.ValueFilter{
								Values: []string{"v1"},
							},
						},
					},
					ServiceLabels: map[string]string{
						"version": "v1",
					},
				},
			},
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec := s.EgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestEgressPipelineWithMultipleCanarySpec(t *testing.T) {
	s := &Service{
		Name: "order-002-canary-array",
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
		Canary: &Canary{
			CanaryRules: []*CanaryRule{
				{
					Filter: &httpfilter.Spec{
						Headers: map[string]*httpfilter.ValueFilter{
							"X-canary": &httpfilter.ValueFilter{
								Values: []string{"v1"},
							},
						},
					},
					ServiceLabels: map[string]string{
						"version": "v1",
					},
				},
				{
					Filter: &httpfilter.Spec{
						Headers: map[string]*httpfilter.ValueFilter{
							"X-canary": &httpfilter.ValueFilter{
								Values: []string{"ams"},
							},
						},
					},
					ServiceLabels: map[string]string{
						"version": "v2",
					},
				},
			},
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec := s.EgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestEgressPipelineWithCanaryNoInstanceSpec(t *testing.T) {
	s := &Service{
		Name: "order-002-canary-no-instance",
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
		Canary: &Canary{
			CanaryRules: []*CanaryRule{
				{
					Filter: &httpfilter.Spec{
						Headers: map[string]*httpfilter.ValueFilter{
							"X-canary": &httpfilter.ValueFilter{
								Values: []string{"aaa"},
							},
						},
					},
					ServiceLabels: map[string]string{
						"version": "v1",
					},
				},
			},
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary-no-match",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v3",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec := s.EgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

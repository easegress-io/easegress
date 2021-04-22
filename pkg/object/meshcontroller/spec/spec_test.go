package spec

import (
	"fmt"
	"testing"

	"github.com/megaease/easegateway/pkg/filter/backend"
	"github.com/megaease/easegateway/pkg/filter/resilience"
	"github.com/megaease/easegateway/pkg/filter/resilience/circuitbreaker"
	"github.com/megaease/easegateway/pkg/filter/resilience/ratelimiter"
	"github.com/megaease/easegateway/pkg/filter/resilience/retryer"
	"github.com/megaease/easegateway/pkg/filter/resilience/timelimiter"
	"github.com/megaease/easegateway/pkg/util/httpfilter"
)

func TestSideCarIngressPipelineSpec(t *testing.T) {
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

	superSpec, _ := s.SideCarIngressPipelineSpec(443)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSideCarEgressPipelineSpec(t *testing.T) {
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

	superSpec, _ := s.SideCarEgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSideCarEgressPipelineWithCanarySpec(t *testing.T) {
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

	superSpec, _ := s.SideCarEgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSideCarEgressPipelineWithMultipleCanarySpec(t *testing.T) {
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

	superSpec, _ := s.SideCarEgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSideCarEgressPipelineWithCanaryNoInstanceSpec(t *testing.T) {
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

	superSpec, _ := s.SideCarEgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}
func TestSideCarEgressPipelineWithCanaryInstanceMultipleLabelSpec(t *testing.T) {
	s := &Service{
		Name: "order-002-canary-instance-multiple-label",
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
						"app":     "backend",
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
			ServiceName: "fake-002-canary-match-two",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
				"app":     "backend",
			},
		},
		{
			ServiceName: "fake-003-canary-match-one",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
	}

	superSpec, _ := s.SideCarEgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSideCarIngressWithResiliencePipelineSpec(t *testing.T) {
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
		Resilience: &Resilience{
			RateLimiter: &ratelimiter.Spec{
				Policies: []*ratelimiter.Policy{{
					Name:               "default",
					TimeoutDuration:    "100ms",
					LimitForPeriod:     50,
					LimitRefreshPeriod: "10ms",
				}},
				DefaultPolicyRef: "default",
				URLs: []*ratelimiter.URLRule{{
					URLRule: resilience.URLRule{
						Methods: []string{"GET"},
						URL: resilience.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					}}},
			},
		},
	}

	superSpec, _ := s.SideCarIngressPipelineSpec(443)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSideCarEgressResiliencePipelineSpec(t *testing.T) {
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

		Resilience: &Resilience{
			CircuitBreaker: &circuitbreaker.Spec{
				Policies: []*circuitbreaker.Policy{{
					Name:                             "default",
					SlidingWindowType:                "COUNT_BASED",
					FailureRateThreshold:             50,
					SlowCallRateThreshold:            100,
					SlidingWindowSize:                100,
					PermittedNumberOfCallsInHalfOpen: 10,
					MinimumNumberOfCalls:             20,
					SlowCallDurationThreshold:        "100ms",
					MaxWaitDurationInHalfOpen:        "60s",
					WaitDurationInOpen:               "60s",
					CountingNetworkError:             false,
					FailureStatusCodes:               []int{500, 501},
				}},
				DefaultPolicyRef: "default",
				URLs: []*circuitbreaker.URLRule{{
					URLRule: resilience.URLRule{
						Methods: []string{"GET"},
						URL: resilience.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					}}},
			},

			Retryer: &retryer.Spec{
				Policies: []*retryer.Policy{{
					Name:                 "default",
					MaxAttempts:          3,
					WaitDuration:         "500ms",
					BackOffPolicy:        "random",
					RandomizationFactor:  0.5,
					CountingNetworkError: false,
					FailureStatusCodes:   []int{500, 501},
				}},
				DefaultPolicyRef: "default",
				URLs: []*retryer.URLRule{{
					URLRule: resilience.URLRule{
						Methods: []string{"GET"},
						URL: resilience.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					}}},
			},

			TimeLimiter: &timelimiter.Spec{
				DefaultTimeoutDuration: "500ms",
				URLs: []*timelimiter.URLRule{{
					URLRule: resilience.URLRule{
						Methods: []string{"GET"},
						URL: resilience.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
					},
					TimeoutDuration: "500ms",
				}},
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
			ServiceName: "fake-002",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
		},
	}

	superSpec, _ := s.SideCarEgressPipelineSpec(instanceSpecs)
	fmt.Println(superSpec.YAMLConfig())
}

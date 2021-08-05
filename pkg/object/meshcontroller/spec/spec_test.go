/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spec

import (
	"fmt"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/filter/circuitbreaker"
	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/filter/ratelimiter"
	"github.com/megaease/easegress/pkg/filter/retryer"
	"github.com/megaease/easegress/pkg/filter/timelimiter"
	"github.com/megaease/easegress/pkg/logger"
	_ "github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestSideCarIngressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	superSpec, err := s.SideCarIngressPipelineSpec(443)
	if err != nil {
		t.Fatalf("%v", err)
	}
	fmt.Println(superSpec.YAMLConfig())
}

func TestAdminInValidat(t *testing.T) {
	a := Admin{
		RegistryType:      "unknow",
		HeartbeatInterval: "10s",
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("registry type is invalid, should failed")
	}
}

func TestAdminValidat(t *testing.T) {
	a := Admin{
		RegistryType:      "eureka",
		HeartbeatInterval: "10s",
	}

	err := a.Validate()

	if err != nil {
		t.Errorf("registry type is valid, err: %v", err)
	}
}

func TestSideCarEgressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyIPHash,
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
			Policy: proxy.PolicyIPHash,
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
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "lv1",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
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

func TestSideCarEgressPipelneNotLoadBalancer(t *testing.T) {
	s := &Service{
		Name: "order-003-canary-array",
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
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "lv1",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
						"version": "v1",
					},
				},
				{
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "ams",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
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

func TestSideCarEgressPipelineWithMultipleCanarySpec(t *testing.T) {
	s := &Service{
		Name: "order-003-canary-array",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyIPHash,
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
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "lv1",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
						"version": "v1",
					},
				},
				{
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "ams",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
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
		Name: "order-004-canary-no-instance",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyIPHash,
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
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "aaa",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
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
		Name: "order-005-canary-instance-multiple-label",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyIPHash,
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
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "lv1",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
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

func TestIngressHTTPServerSpec(t *testing.T) {
	rule := []*IngressRule{
		{
			Host: "megaease.com",
			Paths: []*IngressPath{
				{
					Path:    "/",
					Backend: "portal",
				},
			},
		},
	}

	_, err := IngressHTTPServerSpec(1233, rule)

	if err != nil {
		t.Errorf("ingress http server spec failed: %v", err)
	}

}
func TestSideCarIngressWithResiliencePipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyRandom,
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
					URLRule: urlrule.URLRule{
						Methods: []string{"GET"},
						URL: urlrule.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					},
				}},
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
			Policy: proxy.PolicyIPHash,
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
					URLRule: urlrule.URLRule{
						Methods: []string{"GET"},
						URL: urlrule.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					},
				}},
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
					URLRule: urlrule.URLRule{
						Methods: []string{"GET"},
						URL: urlrule.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					},
				}},
			},

			TimeLimiter: &timelimiter.Spec{
				DefaultTimeoutDuration: "500ms",
				URLs: []*timelimiter.URLRule{{
					URLRule: urlrule.URLRule{
						Methods: []string{"GET"},
						URL: urlrule.StringMatch{
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

func TestPipelineBuilderFailed(t *testing.T) {
	builder := newPipelineSpecBuilder("abc")

	builder.appendRateLimiter(nil)

	builder.appendCircuitBreaker(nil)

	builder.appendRetryer(nil)

	builder.appendTimeLimiter(nil)

	yamlStr := builder.yamlConfig()
	if len(yamlStr) == 0 {
		t.Errorf("builder append nil resilience filter failed")
	}
}

func TestPipelineBuilder(t *testing.T) {
	builder := newPipelineSpecBuilder("abc")

	rateLimiter := &ratelimiter.Spec{
		Policies: []*ratelimiter.Policy{{
			Name:               "default",
			TimeoutDuration:    "100ms",
			LimitForPeriod:     50,
			LimitRefreshPeriod: "10ms",
		}},
		DefaultPolicyRef: "default",
		URLs: []*ratelimiter.URLRule{{
			URLRule: urlrule.URLRule{
				Methods: []string{"GET"},
				URL: urlrule.StringMatch{
					Exact:  "/path1",
					Prefix: "/path2/",
					RegEx:  "^/path3/[0-9]+$",
				},
				PolicyRef: "default",
			},
		}},
	}

	builder.appendRateLimiter(rateLimiter)
	yaml := builder.yamlConfig()

	if len(yaml) == 0 {
		t.Errorf("pipeline builder yamlconfig failed")
	}

}

func TestIngressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyRandom,
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
	superSpec, err := s.IngressPipelineSpec(instanceSpecs)

	if err != nil {
		t.Fatalf("%v", err)
	}
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarIngressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	superSpec, err := s.SideCarIngressHTTPServerSpec()

	if err != nil {
		t.Fatalf("ingress http server spec failed: %v", err)
	}
	fmt.Println(superSpec.YAMLConfig())

	superSpec, err = s.SideCarEgressHTTPServerSpec()

	if err != nil {
		t.Fatalf("egress http server spec failed: %v", err)
	}

	fmt.Println(superSpec.YAMLConfig())
}

func TestUniqueCanaryHeadersEmpty(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	val := s.UniqueCanaryHeaders()
	if len(val) > 0 {
		t.Errorf("canary header should be none")
	}

}

func TestUniqueCanaryHeaders(t *testing.T) {
	s := &Service{
		Name: "order-002-canary",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyIPHash,
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
					Headers: map[string]*urlrule.StringMatch{
						"X-canary": {
							Exact: "lv1",
						},
					},
					URLs: []*urlrule.URLRule{
						{
							Methods: []string{
								"GET",
								"POST",
							},
							URL: urlrule.StringMatch{
								Prefix: "/",
							},
						},
					},
					ServiceInstanceLabels: map[string]string{
						"version": "v1",
					},
				},
			},
		},
	}

	val := s.UniqueCanaryHeaders()
	if len(val) == 0 {
		t.Errorf("canary header should not be none")
	}
}

func TestEgressName(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.PolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}
	if len(s.EgressHTTPServerName()) == 0 {
		t.Error("egress httpserver name should not be none")
	}

	if len(s.EgressHandlerName()) == 0 {
		t.Error("egress httpserver handler should not be none")
	}

	if len(s.IngressHTTPServerName()) == 0 {
		t.Error("ingress httpserver handler should not be none")
	}

	if len(s.IngressHandlerName()) == 0 {
		t.Error("ingress httpserver handler should not be none")
	}

	if len(s.BackendName()) == 0 {
		t.Error("backend name should not be none")
	}

	if len(s.IngressEndpoint()) == 0 {
		t.Error("ingress endpoint name should not be none")
	}

	if len(s.EgressEndpoint()) == 0 {
		t.Error("egress endpoint name should not be none")
	}
}

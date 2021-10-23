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
	"bytes"
	"fmt"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/filter/circuitbreaker"
	"github.com/megaease/easegress/pkg/filter/mock"
	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/filter/ratelimiter"
	"github.com/megaease/easegress/pkg/filter/retryer"
	"github.com/megaease/easegress/pkg/filter/timelimiter"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpfilter"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

const (
	// RegistryTypeConsul is the consul registry type.
	RegistryTypeConsul = "consul"
	// RegistryTypeEureka is the eureka registry type.
	RegistryTypeEureka = "eureka"
	// RegistryTypeNacos is the eureka registry type.
	RegistryTypeNacos = "nacos"

	// GlobalTenant is the reserved name of the system scope tenant,
	// its services can be accessible in mesh wide.
	GlobalTenant = "global"

	// ServiceStatusUp indicates this service instance can accept ingress traffic
	ServiceStatusUp = "UP"

	// ServiceStatusOutOfService indicates this service instance can't accept ingress traffic
	ServiceStatusOutOfService = "OUT_OF_SERVICE"

	// WorkerAPIPort is the default port for worker's API server
	WorkerAPIPort = 13009

	// IngressPort is the default port for ingress controller
	IngressPort = 13010

	// HeartbeatInterval is the default heartbeat interval for checking service heartbeat
	HeartbeatInterval = "5s"

	// SecurityLevelPermissive is the level for not enabling mTLS.
	SecurityLevelPermissive = "permissive"

	// SecurityLevelStrict is the level for enabling mTLS.
	SecurityLevelStrict = "strict"

	// DefaultCommonName is the name of root ca cert.
	DefaultCommonName = "mesh-root-ca"

	// CertProviderSelfSign is the in-memory, self-sign cert provider.
	CertProviderSelfSign = "selfSign"

	// IngressControllerName is the name of easemesh ingress controller.
	IngressControllerName = "ingresscontroller"

	// from k8s pod's env value

	// PodEnvHostname is the name of the pod in environment variable.
	PodEnvHostname = "HOSTNAME"

	// PodEnvApplicationIP is the IP of the pod in environment variable.
	PodEnvApplicationIP = "APPLICATION_IP"
)

var (
	// ErrParamNotMatch means RESTful request URL's object name or other fields are not matched in this request's body
	ErrParamNotMatch = fmt.Errorf("param in url and body's spec not matched")
	// ErrAlreadyRegistered indicates this instance has already been registered
	ErrAlreadyRegistered = fmt.Errorf("service already registered")
	// ErrNoRegisteredYet indicates this instance haven't registered successfully yet
	ErrNoRegisteredYet = fmt.Errorf("service not registered yet")
	// ErrServiceNotFound indicates could find target service in its tenant or in global tenant
	ErrServiceNotFound = fmt.Errorf("can't find service in its tenant or in global tenant")
	// ErrServiceNotavailable indicates could find target service's available instances.
	ErrServiceNotavailable = fmt.Errorf("can't find service available instances")
)

type (
	// Admin is the spec of MeshController.
	Admin struct {
		// HeartbeatInterval is the interval for one service instance reporting its heartbeat.
		HeartbeatInterval string `yaml:"heartbeatInterval" jsonschema:"required,format=duration"`
		// RegistryTime indicates which protocol the registry center accepts.
		RegistryType string `yaml:"registryType" jsonschema:"required"`

		// APIPort is the port for worker's API server
		APIPort int `yaml:"apiPort" jsonschema:"required"`

		// IngressPort is the port for http server in mesh ingress
		IngressPort int `yaml:"ingressPort" jsonschema:"required"`

		ExternalServiceRegistry string `yaml:"externalServiceRegistry" jsonschema:"omitempty"`

		Security *Security `yaml:"security" jsonschema:"omitempty"`
	}

	// Security is the spec for mesh-wide security.
	Security struct {
		MTLSMode     string `yaml:"mtlsMode" jsonschema:"required"`
		CertProvider string `yaml:"certProvider" jsonschema:"required"`

		RootCertTTL string `yaml:"rootCertTTL" jsonschema:"required, format=duration"`
		AppCertTTL  string `yaml:"appCertTTL" jsonschema:"required, format=duration"`
	}

	// Service contains the information of service.
	Service struct {
		// Empty means mesh registry itself.
		RegistryName   string `yaml:"registryName" jsonschema:"omitempty"`
		Name           string `yaml:"name" jsonschema:"required"`
		RegisterTenant string `yaml:"registerTenant" jsonschema:"required"`

		Sidecar       *Sidecar       `yaml:"sidecar" jsonschema:"required"`
		Mock          *Mock          `yaml:"mock" jsonschema:"omitempty"`
		Resilience    *Resilience    `yaml:"resilience" jsonschema:"omitempty"`
		Canary        *Canary        `yaml:"canary" jsonschema:"omitempty"`
		LoadBalance   *LoadBalance   `yaml:"loadBalance" jsonschema:"omitempty"`
		Observability *Observability `yaml:"observability" jsonschema:"omitempty"`
	}

	// Mock is the spec of configured and static API responses for this service.
	Mock struct {
		// Enable is the mocking switch for this service.
		Enabled bool `yaml:"enabled" jsonschema:"required"`

		// Rules are the mocking matching and responding configurations.
		Rules []*mock.Rule `yaml:"rules" jsonschema:"omitempty"`
	}

	// Resilience is the spec of service resilience.
	Resilience struct {
		RateLimiter    *ratelimiter.Spec    `yaml:"rateLimiter" jsonschema:"omitempty"`
		CircuitBreaker *circuitbreaker.Spec `yaml:"circuitBreaker" jsonschema:"omitempty"`
		Retryer        *retryer.Spec        `yaml:"retryer" jsonschema:"omitempty"`
		TimeLimiter    *timelimiter.Spec    `yaml:"timeLimiter" jsonschema:"omitempty"`
	}

	// Canary is the spec of service canary.
	Canary struct {
		CanaryRules []*CanaryRule `yaml:"canaryRules" jsonschema:"omitempty"`
	}

	// CanaryRule is one matching rule for canary.
	CanaryRule struct {
		ServiceInstanceLabels map[string]string               `yaml:"serviceInstanceLabels" jsonschema:"required"`
		Headers               map[string]*urlrule.StringMatch `yaml:"headers" jsonschema:"required"`
		URLs                  []*urlrule.URLRule              `yaml:"urls" jsonschema:"required"`
	}

	// GlobalCanaryHeaders is the spec of global service
	GlobalCanaryHeaders struct {
		ServiceHeaders map[string][]string `yaml:"serviceHeaders" jsonschema:"omitempty"`
	}

	// LoadBalance is the spec of service load balance.
	LoadBalance = proxy.LoadBalance

	// Sidecar is the spec of service sidecar.
	Sidecar struct {
		DiscoveryType   string `yaml:"discoveryType" jsonschema:"required"`
		Address         string `yaml:"address" jsonschema:"required"`
		IngressPort     int    `yaml:"ingressPort" jsonschema:"required"`
		IngressProtocol string `yaml:"ingressProtocol" jsonschema:"required"`
		EgressPort      int    `yaml:"egressPort" jsonschema:"required"`
		EgressProtocol  string `yaml:"egressProtocol" jsonschema:"required"`
	}

	// Observability is the spec of service observability.
	Observability struct {
		OutputServer *ObservabilityOutputServer `yaml:"outputServer" jsonschema:"omitempty"`
		Tracings     *ObservabilityTracings     `yaml:"tracings" jsonschema:"omitempty"`
		Metrics      *ObservabilityMetrics      `yaml:"metrics" jsonschema:"omitempty"`
	}

	// ObservabilityOutputServer is the output server of observability.
	ObservabilityOutputServer struct {
		Enabled         bool   `yaml:"enabled" jsonschema:"required"`
		BootstrapServer string `yaml:"bootstrapServer" jsonschema:"required"`
		Timeout         int    `yaml:"timeout" jsonschema:"required"`
	}

	// ObservabilityTracings is the tracings of observability.
	ObservabilityTracings struct {
		Enabled     bool                              `yaml:"enabled" jsonschema:"required"`
		SampleByQPS int                               `yaml:"sampleByQPS" jsonschema:"required"`
		Output      ObservabilityTracingsOutputConfig `yaml:"output" jsonschema:"required"`

		Request      ObservabilityTracingsDetail `yaml:"request" jsonschema:"required"`
		RemoteInvoke ObservabilityTracingsDetail `yaml:"remoteInvoke" jsonschema:"required"`
		Kafka        ObservabilityTracingsDetail `yaml:"kafka" jsonschema:"required"`
		Jdbc         ObservabilityTracingsDetail `yaml:"jdbc" jsonschema:"required"`
		Redis        ObservabilityTracingsDetail `yaml:"redis" jsonschema:"required"`
		Rabbit       ObservabilityTracingsDetail `yaml:"rabbit" jsonschema:"required"`
	}

	// ObservabilityTracingsOutputConfig is the tracing output configuration
	ObservabilityTracingsOutputConfig struct {
		Enabled         bool   `yaml:"enabled" jsonschema:"required"`
		ReportThread    int    `yaml:"reportThread" jsonschema:"required"`
		Topic           string `yaml:"topic" jsonschema:"required"`
		MessageMaxBytes int    `yaml:"messageMaxBytes" jsonschema:"required"`
		QueuedMaxSpans  int    `yaml:"queuedMaxSpans" jsonschema:"required"`
		QueuedMaxSize   int    `yaml:"queuedMaxSize" jsonschema:"required"`
		MessageTimeout  int    `yaml:"messageTimeout" jsonschema:"required"`
	}
	// ObservabilityTracingsDetail is the tracing detail of observability.
	ObservabilityTracingsDetail struct {
		Enabled       bool   `yaml:"enabled" jsonschema:"required"`
		ServicePrefix string `yaml:"servicePrefix" jsonschema:"required"`
	}

	// ObservabilityMetrics is the metrics of observability.
	ObservabilityMetrics struct {
		Enabled        bool                       `yaml:"enabled" jsonschema:"required"`
		Access         ObservabilityMetricsDetail `yaml:"access" jsonschema:"required"`
		Request        ObservabilityMetricsDetail `yaml:"request" jsonschema:"required"`
		JdbcStatement  ObservabilityMetricsDetail `yaml:"jdbcStatement" jsonschema:"required"`
		JdbcConnection ObservabilityMetricsDetail `yaml:"jdbcConnection" jsonschema:"required"`
		Rabbit         ObservabilityMetricsDetail `yaml:"rabbit" jsonschema:"required"`
		Kafka          ObservabilityMetricsDetail `yaml:"kafka" jsonschema:"required"`
		Redis          ObservabilityMetricsDetail `yaml:"redis" jsonschema:"required"`
		JvmGC          ObservabilityMetricsDetail `yaml:"jvmGc" jsonschema:"required"`
		JvmMemory      ObservabilityMetricsDetail `yaml:"jvmMemory" jsonschema:"required"`
		Md5Dictionary  ObservabilityMetricsDetail `yaml:"md5Dictionary" jsonschema:"required"`
	}

	// ObservabilityMetricsDetail is the metrics detail of observability.
	ObservabilityMetricsDetail struct {
		Enabled  bool   `yaml:"enabled" jsonschema:"required"`
		Interval int    `yaml:"interval" jsonschema:"required"`
		Topic    string `yaml:"topic" jsonschema:"required"`
	}

	// Tenant contains the information of tenant.
	Tenant struct {
		Name string `yaml:"name"`

		Services []string `yaml:"services" jsonschema:"omitempty"`
		// Format: RFC3339
		CreatedAt   string `yaml:"createdAt" jsonschema:"omitempty"`
		Description string `yaml:"description"`
	}

	// Certificate is one cert for mesh service instance or root CA.
	Certificate struct {
		IP          string `yaml:"ip" jsonschema:"required"`
		ServiceName string `yaml:"servieName" jsonschema:"required"`
		CertBase64  string `yaml:"certBase64" jsonschema:"required"`
		KeyBase64   string `yaml:"keyBase64" jsonschema:"required"`
		TTL         string `yaml:"ttl" jsonschema:"required,format=duration"`
		SignTime    string `yaml:"signTime" jsonschema:"required,format=timerfc3339"`
		HOST        string `yaml:"host" jsonschema:"required"`
	}

	// ServiceInstanceSpec is the spec of service instance.
	// FIXME: Use the unified struct: serviceregistry.ServiceInstanceSpec.
	ServiceInstanceSpec struct {
		RegistryName string `yaml:"registryName" jsonschema:"required"`
		// Provide by registry client
		ServiceName  string            `yaml:"serviceName" jsonschema:"required"`
		InstanceID   string            `yaml:"instanceID" jsonschema:"required"`
		IP           string            `yaml:"ip" jsonschema:"required"`
		Port         uint32            `yaml:"port" jsonschema:"required"`
		RegistryTime string            `yaml:"registryTime" jsonschema:"omitempty"`
		Labels       map[string]string `yaml:"labels" jsonschema:"omitempty"`

		// Set by heartbeat timer event or API
		Status string `yaml:"status" jsonschema:"omitempty"`
	}

	// IngressPath is the path for a mesh ingress rule
	IngressPath struct {
		Path          string `yaml:"path" jsonschema:"required"`
		RewriteTarget string `yaml:"rewriteTarget" jsonschema:"omitempty"`
		Backend       string `yaml:"backend" jsonschema:"required"`
	}

	// IngressRule is the rule for mesh ingress
	IngressRule struct {
		Host  string         `yaml:"host" jsonschema:"omitempty"`
		Paths []*IngressPath `yaml:"paths" jsonschema:"required"`
	}

	// Ingress is the spec of mesh ingress
	Ingress struct {
		Name  string         `yaml:"name" jsonschema:"required"`
		Rules []*IngressRule `yaml:"rules" jsonschema:"required"`
	}

	// ServiceInstanceStatus is the status of service instance.
	ServiceInstanceStatus struct {
		ServiceName string `yaml:"serviceName" jsonschema:"required"`
		InstanceID  string `yaml:"instanceID" jsonschema:"required"`
		// RFC3339 format
		LastHeartbeatTime string `yaml:"lastHeartbeatTime" jsonschema:"required,format=timerfc3339"`
	}

	pipelineSpecBuilder struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`

		// NOTE: Can't use *httppipeline.Spec here.
		// Reference: https://github.com/go-yaml/yaml/issues/356
		httppipeline.Spec `yaml:",inline"`
	}

	// DynamicObject defines a dynamic object which is a map of string to interface{}.
	// The value of this map could also be a dynamic object, but in this case, its type
	// must be `map[string]interface{}`, and should not be `map[interface{}]interface{}`.
	DynamicObject map[string]interface{}

	// CustomResourceKind defines the spec of a custom resource kind
	CustomResourceKind struct {
		Name       string        `yaml:"name" jsonschema:"required"`
		JSONSchema DynamicObject `yaml:"jsonSchema" jsonschema:"omitempty"`
	}

	// CustomResource defines the spec of a custom resource
	CustomResource DynamicObject

	// HTTPMatch defines an individual route for HTTP traffic
	HTTPMatch struct {
		// Name is the name of the match for referencing in a TrafficTarget
		Name string `json:"name,omitempty"`

		// Methods for inbound traffic as defined in RFC 7231
		// https://tools.ietf.org/html/rfc7231#section-4
		Methods []string `json:"methods,omitempty"`

		// PathRegex is a regular expression defining the route
		PathRegex string `json:"pathRegex,omitempty"`

		// Headers is a list of headers used to match HTTP traffic.
		//
		// But we are unable support headers in mesh, because all requests to mesh egress
		// contains a special header 'X-Mesh-Rpc-Service', whose value is the name of the
		// target service, and the relationship between multiple headers are 'OR'.
		//
		// Headers map[string]string `json:"headers,omitempty"`
	}

	// HTTPRouteGroup defines the spec of a HTTP route group
	HTTPRouteGroup struct {
		// Name is the name for referencing a HTTPRouteGroup
		Name string `yaml:"name" jsonschema:"required"`

		// Matches is a list of HTTPMatch to match traffic
		Matches []HTTPMatch `json:"matches,omitempty"`
	}

	// TrafficTargetRule is the TrafficSpec to allow for a TrafficTarget
	TrafficTargetRule struct {
		// Kind is the kind of TrafficSpec to allow
		Kind string `json:"kind"`

		// Name of the TrafficSpec to use
		Name string `json:"name"`

		// Matches is a list of TrafficSpec routes to allow traffic for
		// +optional
		Matches []string `json:"matches,omitempty"`
	}

	// IdentityBindingSubject is a service which should be allowed access to the TrafficTarget
	IdentityBindingSubject struct {
		// Kind is the type of Subject to allow ingress (Service)
		Kind string `json:"kind"`

		// Name of the Subject, i.e. ServiceName
		Name string `json:"name"`
	}

	// TrafficTarget is the specification of a TrafficTarget
	TrafficTarget struct {
		// Name is the name for referencing a TrafficTarget
		Name string `yaml:"name" jsonschema:"required"`

		// Destination is the service to allow ingress traffic
		Destination IdentityBindingSubject `json:"destination"`

		// Sources are the services to allow egress traffic
		Sources []IdentityBindingSubject `json:"sources,omitempty"`

		// Rules are the traffic rules to allow (HTTPRoutes)
		Rules []TrafficTargetRule `json:"rules,omitempty"`
	}
)

// UnmarshalYAML implements yaml.Unmarshaler
// the type of a DynamicObject field could be `map[interface{}]interface{}` if it is
// unmarshaled from yaml, but some packages, like the standard json package could not
// handle this type, so it must be converted to `map[string]interface{}`.
func (do *DynamicObject) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m := map[string]interface{}{}
	if err := unmarshal(&m); err != nil {
		return err
	}

	var convert func(interface{}) interface{}
	convert = func(src interface{}) interface{} {
		switch x := src.(type) {
		case map[interface{}]interface{}:
			x2 := map[string]interface{}{}
			for k, v := range x {
				x2[k.(string)] = convert(v)
			}
			return x2
		case []interface{}:
			x2 := make([]interface{}, len(x))
			for i, v := range x {
				x2[i] = convert(v)
			}
			return x2
		}
		return src
	}

	for k, v := range m {
		m[k] = convert(v)
	}
	*do = m

	return nil
}

// Name returns the 'name' field of the custom resource
func (cr CustomResource) Name() string {
	if v, ok := cr["name"].(string); ok {
		return v
	}
	return ""
}

// Kind returns the 'kind' field of the custom resource
func (cr CustomResource) Kind() string {
	if v, ok := cr["kind"].(string); ok {
		return v
	}
	return ""
}

// UnmarshalYAML implements yaml.Unmarshaler
func (cr *CustomResource) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return (*DynamicObject)(cr).UnmarshalYAML(unmarshal)
}

// Validate validates Spec.
func (a Admin) Validate() error {
	switch a.RegistryType {
	case RegistryTypeConsul, RegistryTypeEureka, RegistryTypeNacos:
	default:
		return fmt.Errorf("unsupported registry center type: %s", a.RegistryType)
	}

	if a.Security != nil {
		switch a.Security.CertProvider {
		case CertProviderSelfSign:
		default:
			return fmt.Errorf("unknown mTLS cert provider type: %s", a.Security.CertProvider)
		}

		switch a.Security.MTLSMode {
		case SecurityLevelPermissive, SecurityLevelStrict:
		default:
			return fmt.Errorf("unknown mTLS security level: %s", a.Security.MTLSMode)
		}
	}

	if a.EnablemTLS() {
		appCertTTL, err := time.ParseDuration(a.Security.AppCertTTL)
		if err != nil {
			return fmt.Errorf("parse appcertTTl: %s failed: %v", a.Security.AppCertTTL, err)
		}
		rootCertTTL, err := time.ParseDuration(a.Security.RootCertTTL)
		if err != nil {
			return fmt.Errorf("parse rootTTl: %s failed: %v", a.Security.AppCertTTL, err)
		}

		if appCertTTL >= rootCertTTL {
			err = fmt.Errorf("appCertTTL: %s is larger than rootCertTTL: %s", appCertTTL.String(), rootCertTTL.String())
			return err
		}
	}

	return nil
}

// EnablemTLS indicates whether we should enable mTLS in mesh or not.
func (a Admin) EnablemTLS() bool {
	if a.Security != nil && a.Security.MTLSMode == SecurityLevelStrict {
		return true
	}
	return false
}

// Key returns the key of ServiceInstanceSpec.
func (s *ServiceInstanceSpec) Key() string {
	return fmt.Sprintf("%s/%s/%s", s.RegistryName, s.ServiceName, s.InstanceID)
}

func newPipelineSpecBuilder(name string) *pipelineSpecBuilder {
	return &pipelineSpecBuilder{
		Kind: httppipeline.Kind,
		Name: name,
		Spec: httppipeline.Spec{},
	}
}

func (b *pipelineSpecBuilder) yamlConfig() string {
	buff, err := yaml.Marshal(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", b, err)
	}
	return string(buff)
}

func (b *pipelineSpecBuilder) appendRateLimiter(rl *ratelimiter.Spec) *pipelineSpecBuilder {
	const name = "rateLimiter"

	if rl == nil || len(rl.Policies) == 0 || len(rl.URLs) == 0 {
		return b
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":             ratelimiter.Kind,
		"name":             name,
		"policies":         rl.Policies,
		"defaultPolicyRef": rl.DefaultPolicyRef,
		"urls":             rl.URLs,
	})
	return b
}

func (b *pipelineSpecBuilder) appendCircuitBreaker(cb *circuitbreaker.Spec) *pipelineSpecBuilder {
	const name = "circuitBreaker"

	if cb == nil || len(cb.Policies) == 0 || len(cb.URLs) == 0 {
		return b
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":             circuitbreaker.Kind,
		"name":             name,
		"policies":         cb.Policies,
		"defaultPolicyRef": cb.DefaultPolicyRef,
		"urls":             cb.URLs,
	})
	return b
}

func (b *pipelineSpecBuilder) appendRetryer(r *retryer.Spec) *pipelineSpecBuilder {
	const name = "retryer"

	if r == nil || len(r.Policies) == 0 || len(r.URLs) == 0 {
		return b
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":             retryer.Kind,
		"name":             name,
		"policies":         r.Policies,
		"defaultPolicyRef": r.DefaultPolicyRef,
		"urls":             r.URLs,
	})
	return b
}

func (b *pipelineSpecBuilder) appendMock(m []*mock.Rule) *pipelineSpecBuilder {
	const name = "mock"
	if len(m) == 0 {
		return b
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":  mock.Kind,
		"name":  name,
		"rules": m,
	})

	return b
}

func (b *pipelineSpecBuilder) appendTimeLimiter(tl *timelimiter.Spec) *pipelineSpecBuilder {
	const name = "timeLimiter"

	if tl == nil || len(tl.URLs) == 0 {
		return b
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: name})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":           timelimiter.Kind,
		"name":           name,
		"defaultTimeout": tl.DefaultTimeoutDuration,
		"urls":           tl.URLs,
	})
	return b
}

func (b *pipelineSpecBuilder) appendProxyWithCanary(instanceSpecs []*ServiceInstanceSpec, canary *Canary, lb *proxy.LoadBalance, cert, rootCert *Certificate) *pipelineSpecBuilder {
	mainServers := []*proxy.Server{}
	canaryInstances := []*ServiceInstanceSpec{}

	needMTLS := false
	if cert != nil && rootCert != nil {
		needMTLS = true
	}

	for k, instanceSpec := range instanceSpecs {
		if instanceSpec.Status == ServiceStatusUp {
			if len(instanceSpec.Labels) == 0 {
				if needMTLS {
					mainServers = append(mainServers, &proxy.Server{
						URL: fmt.Sprintf("https://%s:%d", instanceSpec.IP, instanceSpec.Port),
					})
				} else {
					mainServers = append(mainServers, &proxy.Server{
						URL: fmt.Sprintf("http://%s:%d", instanceSpec.IP, instanceSpec.Port),
					})

				}
			} else {
				canaryInstances = append(canaryInstances, instanceSpecs[k])
			}
		}
	}
	backendName := "backend"

	if lb == nil {
		lb = &proxy.LoadBalance{
			Policy: proxy.PolicyRoundRobin,
		}
	}

	candidatePool := []*proxy.PoolSpec{}
	if len(canaryInstances) != 0 && canary != nil && len(canary.CanaryRules) != 0 {
		for _, v := range canary.CanaryRules {
			servers := []*proxy.Server{}
			for _, ins := range canaryInstances {
				for key, label := range v.ServiceInstanceLabels {
					match := false
					for insKey, insLabel := range ins.Labels {
						if key == insKey && label == insLabel {
							if needMTLS {
								servers = append(servers, &proxy.Server{
									URL: fmt.Sprintf("https://%s:%d", ins.IP, ins.Port),
								})
							} else {
								servers = append(servers, &proxy.Server{
									URL: fmt.Sprintf("http://%s:%d", ins.IP, ins.Port),
								})

							}
							match = true
							break
						}
					}
					if match {
						break
					}
				}
			}
			if len(servers) != 0 {
				candidatePool = append(candidatePool, &proxy.PoolSpec{
					Filter: &httpfilter.Spec{
						Headers: v.Headers,
						URLs:    v.URLs,
					},
					ServersTags:     []string{},
					Servers:         servers,
					ServiceRegistry: "",
					ServiceName:     "",
					LoadBalance:     lb,
				})
			}
		}
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: backendName})
	if needMTLS {
		b.Filters = append(b.Filters, map[string]interface{}{
			"kind": proxy.Kind,
			"name": backendName,
			"mainPool": &proxy.PoolSpec{
				Servers:     mainServers,
				LoadBalance: lb,
			},
			"candidatePools": candidatePool,
			"mtls": &proxy.MTLS{
				CertBase64:     cert.CertBase64,
				KeyBase64:      cert.KeyBase64,
				RootCertBase64: rootCert.CertBase64,
			},
		})

		return b
	}

	b.Filters = append(b.Filters, map[string]interface{}{
		"kind": proxy.Kind,
		"name": backendName,
		"mainPool": &proxy.PoolSpec{
			Servers:     mainServers,
			LoadBalance: lb,
		},
		"candidatePools": candidatePool,
	})

	return b
}

func (b *pipelineSpecBuilder) appendProxy(mainServers []*proxy.Server, lb *proxy.LoadBalance) *pipelineSpecBuilder {
	backendName := "backend"

	if lb == nil {
		lb = &proxy.LoadBalance{
			Policy: proxy.PolicyRoundRobin,
		}
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: backendName})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind": proxy.Kind,
		"name": backendName,
		"mainPool": &proxy.PoolSpec{
			Servers:     mainServers,
			LoadBalance: lb,
		},
	})

	return b
}

// IngressHTTPServerSpec generates HTTP server spec for ingress.
// as ingress does not belong to a service, it is not a method of 'Service'
func IngressHTTPServerSpec(port int, rules []*IngressRule) (*supervisor.Spec, error) {
	const specFmt = `
kind: HTTPServer
name: mesh-ingress-server
port: %d
keepAlive: false
https: false
rules:`

	const ruleFmt = `
  - host: %s
    paths:`

	const pathFmt = `
      - pathRegexp: %s
        rewriteTarget: %s
        backend: %s`

	buf := bytes.Buffer{}

	str := fmt.Sprintf(specFmt, port)
	buf.WriteString(str)

	for _, r := range rules {
		str = fmt.Sprintf(ruleFmt, r.Host)
		buf.WriteString(str)
		for j := range r.Paths {
			p := r.Paths[j]
			str = fmt.Sprintf(pathFmt, p.Path, p.RewriteTarget, p.Backend)
			buf.WriteString(str)
		}
	}

	yamlConfig := buf.String()
	spec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return spec, nil
}

// IngressPipelineSpec generates a spec for ingress pipeline spec
func (s *Service) IngressPipelineSpec(instanceSpecs []*ServiceInstanceSpec, cert, rootCert *Certificate) (*supervisor.Spec, error) {
	pipelineSpecBuilder := newPipelineSpecBuilder(s.IngressPipelineName())

	// inner pipeline won't need to active tls config for visiting the local app
	pipelineSpecBuilder.appendProxyWithCanary(instanceSpecs, s.Canary, s.LoadBalance, cert, rootCert)

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

// SideCarIngressHTTPServerSpec generates a spec for sidecar ingress HTTP server
func (s *Service) SideCarIngressHTTPServerSpec(cert, rootCert *Certificate) (*supervisor.Spec, error) {
	ingressHTTPServerFormat := `
kind: HTTPServer
name: %s
port: %d
keepAlive: false
https: %s
certBase64: %s
keyBase64: %s
caCertBase64: %s
rules:
  - paths:
    - pathPrefix: /
      backend: %s`

	name := fmt.Sprintf("mesh-ingress-server-%s", s.Name)
	pipelineName := fmt.Sprintf("mesh-ingress-pipeline-%s", s.Name)
	certBase64, keyBase64, rootCertBaser64, needHTTPS := "", "", "", "false"
	if cert != nil && rootCert != nil {
		certBase64 = cert.CertBase64
		keyBase64 = cert.KeyBase64
		rootCertBaser64 = rootCert.CertBase64
		needHTTPS = "true"
	}
	yamlConfig := fmt.Sprintf(ingressHTTPServerFormat, name,
		s.Sidecar.IngressPort, needHTTPS, certBase64, keyBase64, rootCertBaser64, pipelineName)

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

// UniqueCanaryHeaders returns the unique headers in canary filter rules.
func (s *Service) UniqueCanaryHeaders() []string {
	var headers []string
	if s.Canary == nil || len(s.Canary.CanaryRules) == 0 {
		return headers
	}
	keys := make(map[string]bool)
	for _, canaryRule := range s.Canary.CanaryRules {
		if canaryRule != nil {
			for k := range canaryRule.Headers {
				keys[k] = true
			}
		}
	}

	for k := range keys {
		headers = append(headers, k)
	}
	return headers
}

// EgressHTTPServerName returns egress HTTP server name
func (s *Service) EgressHTTPServerName() string {
	return fmt.Sprintf("mesh-egress-server-%s", s.Name)
}

// EgressHandlerName returns egress handler name.
func (s *Service) EgressHandlerName() string {
	return fmt.Sprintf("mesh-egress-handler-%s", s.Name)
}

// EgressPipelineName returns egress pipeline name
func (s *Service) EgressPipelineName() string {
	return fmt.Sprintf("mesh-egress-pipeline-%s", s.Name)
}

// IngressHTTPServerName returns the ingress server name
func (s *Service) IngressHTTPServerName() string {
	return fmt.Sprintf("mesh-ingress-server-%s", s.Name)
}

// IngressHandlerName returns the ingress handler name
func (s *Service) IngressHandlerName() string {
	return fmt.Sprintf("mesh-ingress-handler-%s", s.Name)
}

// IngressPipelineName returns the ingress pipeline name
func (s *Service) IngressPipelineName() string {
	return fmt.Sprintf("mesh-ingress-pipeline-%s", s.Name)
}

// BackendName returns backend service name
func (s *Service) BackendName() string {
	return s.Name
}

// SideCarEgressHTTPServerSpec returns a spec for egress HTTP server
func (s *Service) SideCarEgressHTTPServerSpec() (*supervisor.Spec, error) {
	egressHTTPServerFormat := `
kind: HTTPServer
name: %s
port: %d
keepAlive: false
https: false
`

	yamlConfig := fmt.Sprintf(egressHTTPServerFormat,
		s.EgressHTTPServerName(),
		s.Sidecar.EgressPort)

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", err)
		return nil, err
	}

	return superSpec, nil
}

// Runnable indicates this service is runnable inside mesh or not.
//   e.g., If this is a mock service, there is not need to be deployed and run.
func (s *Service) Runnable() bool {
	if s.Mock != nil && s.Mock.Enabled {
		return false
	}
	return true
}

// SideCarIngressPipelineSpec returns a spec for sidecar ingress pipeline
func (s *Service) SideCarIngressPipelineSpec(applicationPort uint32) (*supervisor.Spec, error) {
	mainServers := []*proxy.Server{
		{
			URL: s.ApplicationEndpoint(applicationPort),
		},
	}

	pipelineSpecBuilder := newPipelineSpecBuilder(s.IngressPipelineName())

	if s.Resilience != nil {
		pipelineSpecBuilder.appendRateLimiter(s.Resilience.RateLimiter)
	}

	pipelineSpecBuilder.appendProxy(mainServers, s.LoadBalance)

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

// SideCarEgressPipelineSpec returns a spec for sidecar egress pipeline
func (s *Service) SideCarEgressPipelineSpec(instanceSpecs []*ServiceInstanceSpec, appCert, rootCert *Certificate) (*supervisor.Spec, error) {
	if len(instanceSpecs) == 0 {
		return nil, fmt.Errorf("not instance")
	}

	pipelineSpecBuilder := newPipelineSpecBuilder(s.EgressPipelineName())

	if !s.Runnable() {
		pipelineSpecBuilder.appendMock(s.Mock.Rules)
	} else {
		if s.Resilience != nil {
			pipelineSpecBuilder.appendTimeLimiter(s.Resilience.TimeLimiter)
			pipelineSpecBuilder.appendRetryer(s.Resilience.Retryer)
			pipelineSpecBuilder.appendCircuitBreaker(s.Resilience.CircuitBreaker)
		}

		pipelineSpecBuilder.appendProxyWithCanary(instanceSpecs, s.Canary, s.LoadBalance, appCert, rootCert)
	}
	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

// ApplicationEndpoint returns application endpoint URL string
func (s *Service) ApplicationEndpoint(port uint32) string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.IngressProtocol, s.Sidecar.Address, port)
}

// IngressEndpoint returns Ingress endpoint URL string
func (s *Service) IngressEndpoint() string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.IngressProtocol, s.Sidecar.Address, s.Sidecar.IngressPort)
}

// EgressEndpoint returns Egress endpoint URL string
func (s *Service) EgressEndpoint() string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.EgressProtocol, s.Sidecar.Address, s.Sidecar.EgressPort)
}

/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package spec defines the spec for various objects in mesh.
package spec

import (
	"fmt"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster/customdata"
	"github.com/megaease/easegress/v2/pkg/filters/mock"
	proxy "github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/filters/ratelimiter"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
	"github.com/megaease/easegress/v2/pkg/util/urlrule"

	v1 "k8s.io/api/core/v1"
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

	// ServiceCanaryHeaderKey is the http header key of service canary.
	ServiceCanaryHeaderKey = "X-Mesh-Service-Canary"

	defaultKeepAliveTimeout = "60s"
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
		HeartbeatInterval string `json:"heartbeatInterval" jsonschema:"required,format=duration"`

		// RegistryTime indicates which protocol the registry center accepts.
		RegistryType string `json:"registryType" jsonschema:"required"`

		// APIPort is the port for worker's API server
		APIPort int `json:"apiPort" jsonschema:"required"`

		// IngressPort is the port for http server in mesh ingress
		IngressPort int `json:"ingressPort" jsonschema:"required"`

		ExternalServiceRegistry string `json:"externalServiceRegistry,omitempty"`

		CleanExternalRegistry bool `json:"cleanExternalRegistry,omitempty"`

		Security *Security `json:"security,omitempty"`

		// Sidecar injection relevant config.
		ImageRegistryURL          string `json:"imageRegistryURL,omitempty"`
		ImagePullPolicy           string `json:"imagePullPolicy,omitempty"`
		SidecarImageName          string `json:"sidecarImageName,omitempty"`
		AgentInitializerImageName string `json:"agentInitializerImageName,omitempty"`
		Log4jConfigName           string `json:"log4jConfigName,omitempty"`

		MonitorMTLS *MonitorMTLS `json:"monitorMTLS,omitempty"`
		WorkerSpec  WorkerSpec   `json:"workerSpec,omitempty"`
	}

	// WorkerSpec is the spec of worker
	WorkerSpec struct {
		Ingress IngressServerSpec `json:"ingress,omitempty"`
		Egress  EgressServerSpec  `json:"egress,omitempty"`
	}

	// IngressServerSpec is the spec of ingress httpserver in worker
	IngressServerSpec struct {
		KeepAlive        bool   `json:"keepAlive,omitempty"`
		KeepAliveTimeout string `json:"keepAliveTimeout,omitempty" jsonschema:"format=duration"`
	}

	// EgressServerSpec is the spec of egress httpserver in worker
	EgressServerSpec struct {
		KeepAlive        bool   `json:"keepAlive,omitempty"`
		KeepAliveTimeout string `json:"keepAliveTimeout,omitempty" jsonschema:"format=duration"`
	}

	// MonitorMTLS is the spec of mTLS specification of monitor.
	MonitorMTLS struct {
		Enabled  bool   `json:"enabled" jsonschema:"required"`
		URL      string `json:"url" jsonschema:"required"`
		Username string `json:"username" jsonschema:"required"`
		Password string `json:"password" jsonschema:"required"`

		ReporterAppendType string         `json:"reporterAppendType,omitempty"`
		CaCertBase64       string         `json:"caCertBase64" jsonschema:"required,format=base64"`
		Certs              []*MonitorCert `json:"certs" jsonschema:"required"`
	}

	// MonitorCert is the spec for single pack of mTLS.
	MonitorCert struct {
		CertBase64 string   `json:"certBase64" jsonschema:"required,format=base64"`
		KeyBase64  string   `json:"keyBase64" jsonschema:"required,format=base64"`
		Services   []string `json:"services" jsonschema:"required"`
	}

	// Security is the spec for mesh-wide security.
	Security struct {
		MTLSMode     string `json:"mtlsMode" jsonschema:"required"`
		CertProvider string `json:"certProvider" jsonschema:"required"`

		RootCertTTL string `json:"rootCertTTL" jsonschema:"required,format=duration"`
		AppCertTTL  string `json:"appCertTTL" jsonschema:"required,format=duration"`
	}

	// Service contains the information of service.
	Service struct {
		// Empty means mesh registry itself.
		RegistryName   string `json:"registryName,omitempty"`
		Name           string `json:"name" jsonschema:"required"`
		RegisterTenant string `json:"registerTenant" jsonschema:"required"`

		Sidecar       *Sidecar       `json:"sidecar" jsonschema:"required"`
		Mock          *Mock          `json:"mock,omitempty"`
		Resilience    *Resilience    `json:"resilience,omitempty"`
		LoadBalance   *LoadBalance   `json:"loadBalance,omitempty"`
		Observability *Observability `json:"observability,omitempty"`
	}

	// ServiceDeployment contains the information of service deployment.
	ServiceDeployment struct {
		// The spec of Deployment or StatefulSet of Kubernetes.
		App interface{} `json:"app" jsonschema:"required"`

		// All specs of ConfigMaps in volumes of the spec.
		ConfigMaps []*v1.ConfigMap `json:"configMaps,omitempty"`

		// All specs of Secrets in volumes of the spec.
		Secrets []*v1.Secret `json:"secrets,omitempty"`
	}

	// Mock is the spec of configured and static API responses for this service.
	Mock struct {
		// Enable is the mocking switch for this service.
		Enabled bool `json:"enabled" jsonschema:"required"`

		// Rules are the mocking matching and responding configurations.
		Rules []*mock.Rule `json:"rules,omitempty"`
	}

	// Resilience is the spec of service resilience.
	Resilience struct {
		RateLimiter    *ratelimiter.Rule              `json:"rateLimiter,omitempty"`
		CircuitBreaker *resilience.CircuitBreakerRule `json:"circuitBreaker,omitempty"`
		Retry          *resilience.RetryRule          `json:"retry,omitempty"`
		TimeLimiter    *TimeLimiterRule               `json:"timeLimiter,omitempty"`
		FailureCodes   []int                          `json:"failureCodes,omitempty" jsonschema:"uniqueItems=true"`
	}

	// TimeLimiterRule is the spec of TimeLimiter.
	TimeLimiterRule struct {
		Timeout string `json:"timeout" jsonschema:"required,format=duration"`
	}

	// CanaryRule is one matching rule for canary.
	CanaryRule struct {
		ServiceInstanceLabels map[string]string                    `json:"serviceInstanceLabels" jsonschema:"required"`
		Headers               map[string]*stringtool.StringMatcher `json:"headers" jsonschema:"required"`
		URLs                  []*urlrule.URLRule                   `json:"urls" jsonschema:"required"`
	}

	// ServiceCanary is the service canary entry.
	ServiceCanary struct {
		Name string `json:"name" jsonschema:"required"`
		// Priority must be [1, 9], the default is 5 if user does not set it.
		// The smaller number get higher priority.
		// The order is sorted by name alphabetically in the same priority.
		Priority     int              `json:"priority"`
		Selector     *ServiceSelector `json:"selector" jsonschema:"required"`
		TrafficRules *TrafficRules    `json:"trafficRules" jsonschema:"required"`
	}

	// TrafficRules is the rules of traffic.
	TrafficRules struct {
		Headers map[string]*stringtool.StringMatcher `json:"headers" jsonschema:"required"`
	}

	// LoadBalance is the spec of service load balance.
	LoadBalance = proxy.LoadBalanceSpec

	// Sidecar is the spec of service sidecar.
	Sidecar struct {
		DiscoveryType   string `json:"discoveryType" jsonschema:"required"`
		Address         string `json:"address" jsonschema:"required"`
		IngressPort     int    `json:"ingressPort" jsonschema:"required"`
		IngressProtocol string `json:"ingressProtocol" jsonschema:"required"`
		EgressPort      int    `json:"egressPort" jsonschema:"required"`
		EgressProtocol  string `json:"egressProtocol" jsonschema:"required"`
	}

	// Observability is the spec of service observability.
	Observability struct {
		OutputServer *ObservabilityOutputServer `json:"outputServer,omitempty"`
		Tracings     *ObservabilityTracings     `json:"tracings,omitempty"`
		Metrics      *ObservabilityMetrics      `json:"metrics,omitempty"`
	}

	// ObservabilityOutputServer is the output server of observability.
	ObservabilityOutputServer struct {
		Enabled         bool   `json:"enabled" jsonschema:"required"`
		BootstrapServer string `json:"bootstrapServer" jsonschema:"required"`
		Timeout         int    `json:"timeout" jsonschema:"required"`
	}

	// ObservabilityTracings is the tracings of observability.
	ObservabilityTracings struct {
		Enabled     bool                              `json:"enabled" jsonschema:"required"`
		SampleByQPS int                               `json:"sampleByQPS" jsonschema:"required"`
		Output      ObservabilityTracingsOutputConfig `json:"output" jsonschema:"required"`

		Request      ObservabilityTracingsDetail `json:"request" jsonschema:"required"`
		RemoteInvoke ObservabilityTracingsDetail `json:"remoteInvoke" jsonschema:"required"`
		Kafka        ObservabilityTracingsDetail `json:"kafka" jsonschema:"required"`
		Jdbc         ObservabilityTracingsDetail `json:"jdbc" jsonschema:"required"`
		Redis        ObservabilityTracingsDetail `json:"redis" jsonschema:"required"`
		Rabbit       ObservabilityTracingsDetail `json:"rabbit" jsonschema:"required"`
	}

	// ObservabilityTracingsOutputConfig is the tracing output configuration
	ObservabilityTracingsOutputConfig struct {
		Enabled         bool   `json:"enabled" jsonschema:"required"`
		ReportThread    int    `json:"reportThread" jsonschema:"required"`
		Topic           string `json:"topic" jsonschema:"required"`
		MessageMaxBytes int    `json:"messageMaxBytes" jsonschema:"required"`
		QueuedMaxSpans  int    `json:"queuedMaxSpans" jsonschema:"required"`
		QueuedMaxSize   int    `json:"queuedMaxSize" jsonschema:"required"`
		MessageTimeout  int    `json:"messageTimeout" jsonschema:"required"`
	}
	// ObservabilityTracingsDetail is the tracing detail of observability.
	ObservabilityTracingsDetail struct {
		Enabled       bool   `json:"enabled" jsonschema:"required"`
		ServicePrefix string `json:"servicePrefix" jsonschema:"required"`
	}

	// ObservabilityMetrics is the metrics of observability.
	ObservabilityMetrics struct {
		Enabled        bool                       `json:"enabled" jsonschema:"required"`
		Access         ObservabilityMetricsDetail `json:"access" jsonschema:"required"`
		Request        ObservabilityMetricsDetail `json:"request" jsonschema:"required"`
		JdbcStatement  ObservabilityMetricsDetail `json:"jdbcStatement" jsonschema:"required"`
		JdbcConnection ObservabilityMetricsDetail `json:"jdbcConnection" jsonschema:"required"`
		Rabbit         ObservabilityMetricsDetail `json:"rabbit" jsonschema:"required"`
		Kafka          ObservabilityMetricsDetail `json:"kafka" jsonschema:"required"`
		Redis          ObservabilityMetricsDetail `json:"redis" jsonschema:"required"`
		JvmGC          ObservabilityMetricsDetail `json:"jvmGc" jsonschema:"required"`
		JvmMemory      ObservabilityMetricsDetail `json:"jvmMemory" jsonschema:"required"`
		Md5Dictionary  ObservabilityMetricsDetail `json:"md5Dictionary" jsonschema:"required"`
	}

	// ObservabilityMetricsDetail is the metrics detail of observability.
	ObservabilityMetricsDetail struct {
		Enabled  bool   `json:"enabled" jsonschema:"required"`
		Interval int    `json:"interval" jsonschema:"required"`
		Topic    string `json:"topic" jsonschema:"required"`
	}

	// Tenant contains the information of tenant.
	Tenant struct {
		Name string `json:"name"`

		Services []string `json:"services,omitempty"`
		// Format: RFC3339
		CreatedAt   string `json:"createdAt"`
		Description string `json:"description,omitempty"`
	}

	// Certificate is one cert for mesh service instance or root CA.
	Certificate struct {
		IP          string `json:"ip" jsonschema:"required"`
		ServiceName string `json:"servieName" jsonschema:"required"`
		CertBase64  string `json:"certBase64" jsonschema:"required"`
		KeyBase64   string `json:"keyBase64" jsonschema:"required"`
		TTL         string `json:"ttl" jsonschema:"required,format=duration"`
		SignTime    string `json:"signTime" jsonschema:"required,format=timerfc3339"`
		HOST        string `json:"host" jsonschema:"required"`
	}

	// ServiceInstanceSpec is the spec of service instance.
	// FIXME: Use the unified struct: serviceregistry.ServiceInstanceSpec.
	ServiceInstanceSpec struct {
		// AgentType supports EaseAgent, GoSDK, None(same as empty value).
		AgentType    string `json:"agentType" jsonschema:"required"`
		RegistryName string `json:"registryName" jsonschema:"required"`
		// Provide by registry client
		ServiceName  string            `json:"serviceName" jsonschema:"required"`
		InstanceID   string            `json:"instanceID" jsonschema:"required"`
		IP           string            `json:"ip" jsonschema:"required"`
		Port         uint32            `json:"port" jsonschema:"required"`
		RegistryTime string            `json:"registryTime,omitempty"`
		Labels       map[string]string `json:"labels,omitempty"`

		// Set by heartbeat timer event or API
		Status string `json:"status"`
	}

	// IngressPath is the path for a mesh ingress rule
	IngressPath struct {
		Path          string `json:"path" jsonschema:"required,pattern=^/"`
		RewriteTarget string `json:"rewriteTarget,omitempty"`
		Backend       string `json:"backend" jsonschema:"required"`
	}

	// IngressRule is the rule for mesh ingress
	IngressRule struct {
		Host  string         `json:"host,omitempty"`
		Paths []*IngressPath `json:"paths" jsonschema:"required"`
	}

	// Ingress is the spec of mesh ingress
	Ingress struct {
		Name  string         `json:"name" jsonschema:"required"`
		Rules []*IngressRule `json:"rules" jsonschema:"required"`
	}

	// ServiceInstanceStatus is the status of service instance.
	ServiceInstanceStatus struct {
		ServiceName string `json:"serviceName" jsonschema:"required"`
		InstanceID  string `json:"instanceID" jsonschema:"required"`
		// RFC3339 format
		LastHeartbeatTime string `json:"lastHeartbeatTime" jsonschema:"required,format=timerfc3339"`
	}

	// CustomResourceKind defines the spec of a custom resource kind
	CustomResourceKind = customdata.Kind

	// CustomResource defines the spec of a custom resource
	CustomResource = customdata.Data

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
		Name string `json:"name" jsonschema:"required"`

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
		Name string `json:"name" jsonschema:"required"`

		// Destination is the service to allow ingress traffic
		Destination IdentityBindingSubject `json:"destination"`

		// Sources are the services to allow egress traffic
		Sources []IdentityBindingSubject `json:"sources,omitempty"`

		// Rules are the traffic rules to allow (HTTPRoutes)
		Rules []TrafficTargetRule `json:"rules,omitempty"`
	}
)

// Validate validates ServiceCanary.
func (sc ServiceCanary) Validate() error {
	if sc.Priority < 0 || sc.Priority > 9 {
		return fmt.Errorf("invalid priority (range is [0, 9], the default 0 will be set to 5)")
	}

	return nil
}

// Clone clones TrafficRules.
func (tr *TrafficRules) Clone() *TrafficRules {
	headers := map[string]*stringtool.StringMatcher{}
	for k, v := range tr.Headers {
		stringMatch := *v
		headers[k] = &stringMatch
	}

	return &TrafficRules{
		Headers: headers,
	}
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

	if a.MonitorMTLS != nil {
		serviceMap := map[string]struct{}{}
		for _, cert := range a.MonitorMTLS.Certs {
			for _, service := range cert.Services {
				_, exists := serviceMap[service]
				if exists {
					return fmt.Errorf("service %s in monitotMTLS.certs occurred multiple times", service)
				}
				serviceMap[service] = struct{}{}
			}
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

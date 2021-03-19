package spec

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/filter/backend"
	"github.com/megaease/easegateway/pkg/filter/resilience/circuitbreaker"
	"github.com/megaease/easegateway/pkg/filter/resilience/ratelimiter"
	"github.com/megaease/easegateway/pkg/filter/resilience/retryer"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/httpfilter"

	"gopkg.in/yaml.v2"
)

const (
	// RegistryTypeConsul is the consul registry type.
	RegistryTypeConsul = "consul"
	// RegistryTypeEureka is the eureka registry type.
	RegistryTypeEureka = "eureka"

	// GlobalTenant is the reserved name of the system scope tenant,
	// its services can be accessable in mesh wide.
	GlobalTenant = "global"
)

var (
	// ErrParamNotMatch means RESTful request URL's object name or other fields are not matched in this request's body
	ErrParamNotMatch = fmt.Errorf("param in url and body's spec not matched")
	// ErrAlreadyRegistered indicates this instance has already been registried
	ErrAlreadyRegistered = fmt.Errorf("serivce already registrired")
	// ErrNoRegisteredYet indicates this instance haven't registered successfully yet
	ErrNoRegisteredYet = fmt.Errorf("serivce not registrired yet")
	// ErrServiceNotFound indicates could find target service in same tenant or in global tenant
	ErrServiceNotFound = fmt.Errorf("can't find service in same tenant or in global tenant")
)

type (
	// Admin is the spec of MeshController.
	Admin struct {
		// HeartbeatInterval is the interval for one service instance reporting its heartbeat.
		HeartbeatInterval string `yaml:"heartbeatInterval" jsonschema:"required,format=duration"`
		// RegistryTime indicates which protocol the registry center accepts.
		RegistryType string `yaml:"registryType" jsonschema:"required"`
	}

	// Service contains the information of service.
	Service struct {
		Name           string `yaml:"name" jsonschema:"required"`
		RegisterTenant string `yaml:"registerTenant" jsonschema:"required"`

		Resilience    *Resilience    `yaml:"resilience" jsonschema:"omitempty"`
		Canary        *Canary        `yaml:"canary" jsonschema:"omitempty"`
		LoadBalance   *LoadBalance   `yaml:"loadBalance" jsonschema:"omitempty"`
		Sidecar       *Sidecar       `yaml:"sidecar" jsonschema:"omitempty"`
		Observability *Observability `yaml:"observability" jsonschema:"omitempty"`
	}

	// Resilience is the spec of service resilience.
	Resilience struct {
		CircuitBreaker *circuitbreaker.Spec
		RateLimiter    *ratelimiter.Spec
		Retryer        *retryer.Spec
	}

	// Canary is the spec of service canary.
	Canary struct {
		ServiceLabels []string         `yaml:"serviceLabels" jsonschema:"omitempty"`
		Filter        *httpfilter.Spec `yaml:"filter" jsonschema:"required"`
	}

	// LoadBalance is the spec of service load balance.
	LoadBalance = backend.LoadBalance

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
	}

	// ObservabilityTracings is the tracings of observability.
	ObservabilityTracings struct {
		Topic        string                      `yaml:"topic" jsonschema:"required"`
		SampledByQPS int                         `yaml:"sampledByQPS" jsonschema:"required"`
		Request      ObservabilityTracingsDetail `yaml:"request" jsonschema:"required"`
		RemoteInvoke ObservabilityTracingsDetail `yaml:"remoteInvoke" jsonschema:"required"`
		Kafka        ObservabilityTracingsDetail `yaml:"kafka" jsonschema:"required"`
		Jdbc         ObservabilityTracingsDetail `yaml:"jdbc" jsonschema:"required"`
		Redis        ObservabilityTracingsDetail `yaml:"redis" jsonschema:"required"`
		Rabbit       ObservabilityTracingsDetail `yaml:"rabbit" jsonschema:"required"`
	}

	// ObservabilityTracingsDetail is the tracing detail of observability.
	ObservabilityTracingsDetail struct {
		Enabled       bool   `yaml:"enabled" jsonschema:"required"`
		ServicePrefix string `yaml:"servicePrefix" jsonschema:"required"`
	}

	// ObservabilityMetrics is the metrics of observability.
	ObservabilityMetrics struct {
		Request        ObservabilityMetricsDetail `yaml:"request" jsonschema:"required"`
		JdbcStatement  ObservabilityMetricsDetail `yaml:"jdbcStatement" jsonschema:"required"`
		JdbcConnection ObservabilityMetricsDetail `yaml:"jdbcConnection" jsonschema:"required"`
		Rabbit         ObservabilityMetricsDetail `yaml:"rabbit" jsonschema:"required"`
		Kafka          ObservabilityMetricsDetail `yaml:"kafka" jsonschema:"required"`
		Redis          ObservabilityMetricsDetail `yaml:"redis" jsonschema:"required"`
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

		Services []string `yaml:"services"`
		// Format: RFC3339
		CreatedAt   string `yaml:"createdAt"`
		Description string `yaml:"description"`
	}

	// ServiceInstanceSpec is the spec of service instance.
	ServiceInstanceSpec struct {
		// Provide by registry client
		ServiceName  string `yaml:"serviceName" jsonschema:"required"`
		InstanceID   string `yaml:"instanceID" jsonschema:"required"`
		IP           string `yaml:"IP" jsonschema:"required"`
		Port         uint32 `yaml:"port" jsonschema:"required"`
		RegistryTime string `yaml:"registryTime" jsonschema:"omitempty"`

		// Set by heartbeat timer event or API
		Status string `yaml:"status" jsonschema:"omitempty"`
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
)

// Validate validates Spec.
func (a Admin) Validate() error {
	switch a.RegistryType {
	case RegistryTypeConsul, RegistryTypeEureka:
	default:
		return fmt.Errorf("unsupported registry center type: %s", a.RegistryType)
	}

	return nil
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

func (b *pipelineSpecBuilder) appendBackend(mainServers []*backend.Server, lb *backend.LoadBalance) *pipelineSpecBuilder {
	backendName := "backend"

	if lb == nil {
		lb = &backend.LoadBalance{
			Policy: backend.PolicyRoundRobin,
		}
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: backendName})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind": backend.Kind,
		"name": backendName,
		"mainPool": &backend.PoolSpec{
			Servers:     mainServers,
			LoadBalance: lb,
		},
	})

	return b
}

func (s *Service) IngressHTTPServerSpec() *supervisor.Spec {
	ingressHTTPServerFormat := `
kind: HTTPServer
name: %s
port: %d
rules:
  - paths:
    - pathPrefix: /
      backend: %s`

	name := fmt.Sprintf("mesh-ingress-server-%s", s.Name)
	pipelineName := fmt.Sprintf("mesh-ingress-pipeline-%s", s.Name)
	yamlConfig := fmt.Sprintf(ingressHTTPServerFormat, name, s.Sidecar.IngressPort, pipelineName)

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("BUG: new spec for %s failed: %v", err)
	}

	return superSpec
}

func (s *Service) EgressHTTPServerName() string {
	return fmt.Sprintf("mesh-egress-server-%s", s.Name)
}

func (s *Service) EgressHandlerName() string {
	return fmt.Sprintf("mesh-egress-handler-%s", s.Name)
}

func (s *Service) EgressPipelineName() string {
	return fmt.Sprintf("mesh-egress-pipeline-%s", s.Name)
}

func (s *Service) IngressHTTPServerName() string {
	return fmt.Sprintf("mesh-ingress-server-%s", s.Name)
}

func (s *Service) IngressHandlerName() string {
	return fmt.Sprintf("mesh-ingress-handler-%s", s.Name)
}

func (s *Service) IngressPipelineName() string {
	return fmt.Sprintf("mesh-ingress-pipeline-%s", s.Name)
}

func (s *Service) EgressHTTPServerSpec() *supervisor.Spec {
	egressHTTPServerFormat := `
kind: HTTPServer
name: %s
port: %d
rules:
  - paths:
    - pathPrefix: /
      backend: %s`

	yamlConfig := fmt.Sprintf(egressHTTPServerFormat,
		s.EgressHTTPServerName(),
		s.Sidecar.EgressPort,
		s.EgressHandlerName())

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("BUG: new spec for %s failed: %v", err)
		return nil
	}

	return superSpec
}

func (s *Service) IngressPipelineSpec(applicationPort uint32) *supervisor.Spec {
	mainServers := []*backend.Server{
		{
			URL: s.ApplicationEndpoint(applicationPort),
		},
	}

	pipelineSpecBuilder := newPipelineSpecBuilder(s.IngressPipelineName())

	// TODO: pipelineSpecBuilder.appendRateLimiter()

	pipelineSpecBuilder.appendBackend(mainServers, s.LoadBalance)

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("BUG: new spec for %s failed: %v", yamlConfig, err)
		return nil
	}

	return superSpec
}

func (s *Service) EgressPipelineSpec(instanceSpecs []*ServiceInstanceSpec) *supervisor.Spec {
	mainServers := []*backend.Server{}
	for _, instanceSpec := range instanceSpecs {
		mainServers = append(mainServers, &backend.Server{
			URL: fmt.Sprintf("http://%s:%d", instanceSpec.IP, instanceSpec.Port),
		})
	}

	pipelineSpecBuilder := newPipelineSpecBuilder(s.EgressPipelineName())

	// TODO: pipelineSpecBuilder.appendTimeLimiter()
	// TODO: pipelineSpecBuilder.appendRetryer()
	// TODO: pipelineSpecBuilder.appendCircuitBreaker()

	pipelineSpecBuilder.appendBackend(mainServers, s.LoadBalance)

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("BUG: new spec for %s failed: %v", yamlConfig, err)
		return nil
	}

	return superSpec
}

func (s *Service) ApplicationEndpoint(port uint32) string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.IngressProtocol, s.Sidecar.Address, port)
}

func (s *Service) IngressEndpoint() string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.IngressProtocol, s.Sidecar.Address, s.Sidecar.IngressPort)
}

func (s *Service) EgressEndpoint() string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.EgressProtocol, s.Sidecar.Address, s.Sidecar.EgressPort)
}

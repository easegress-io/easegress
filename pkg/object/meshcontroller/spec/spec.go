package spec

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/filter/backend"
	"github.com/megaease/easegateway/pkg/filter/resilience/circuitbreaker"
	"github.com/megaease/easegateway/pkg/filter/resilience/ratelimiter"
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
	// ErrAlreadyRegistried indicates this instance has already been registried
	ErrAlreadyRegistried = fmt.Errorf("serivce already registrired")
	// ErrNoRegistriedYet indicates this instance haven't registered successfully yet
	ErrNoRegistriedYet = fmt.Errorf("serivce not registrired yet")
	// ErrServiceNotFound indicates could find target service in same tenant or in global tenant
	ErrServiceNotFound = fmt.Errorf("can't find service in same tenant or in global tenant")

	// DefaultIngressPipelineYAML is the default yaml config of ingress pipeline
	DefaultIngressPipelineYAML = `
name: %s
kind: HTTPPipeline
flow:
  - filter: %s
filters:
  - name: %s
    kind: Backend
    mainPool:
      servers:
      - url: %s
`
	// DefaultIngressHTTPServerYAML is the default yaml config of ingress HTTPServer
	// all ingress traffic will be routed to local java process by path prefix '/' match
	DefaultIngressHTTPServerYAML = `
kind: HTTPServer
name: %s
port: %d
rules:
  - paths:
    - pathPrefix: /
      backend: %s,
`
	// DefaultEgressHTTPServerYAML is the default yaml config of egress HTTPServer
	// it doesn't contain any rules as Ingress's spec, cause these rules will dynamically
	// add/remove when Java business process needs to invoke RPC requests.
	DefaultEgressHTTPServerYAML = `
kind: HTTPServer
name: %s
port: %d
rules:
  - paths:
    - pathPrefix: /
      backend: %s,
`
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
	}

	// Canary is the spec of service canary.
	Canary struct {
		ServiceLabels []string         `yaml:"serviceLabels" jsonschema:"omitempty"`
		Filter        *httpfilter.Spec `yaml:"filter" jsonschema:"required"`
	}

	// LoadBalance is the spec of service load balance.
	LoadBalance backend.LoadBalance

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
		// RFC3339 format
		LastHeartbeatTime string `yaml:"lastHeartbeatTime" jsonschema:"required,format=timerfc3339"`
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

// GenIngressPipelineObjectName generates the EG running egress pipeline object name
func GenIngressPipelineObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-ingress-%s-pipeline", serviceName)
	return name
}

// GenIngressBackendFilterName generates the EG running ingress backend filter name
func GenIngressBackendFilterName(serviceName string) string {
	name := fmt.Sprintf("mesh-ingress-%s-backend", serviceName)
	return name
}

// GenIngressHTTPSvrObjectName generates the EG running ingress HTTPServer object name
func GenIngressHTTPSvrObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-ingress-%s-httpserver", serviceName)
	return name
}

// GenEgressHTTPSvrObjectName generates the EG running egress HTTPServer object name
func GenEgressHTTPSvrObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-egress-%s-httpserver", serviceName)
	return name
}

// GenEgressHTTPProtocolHandlerName generates the Mesh Egress
// HTTP protocol
func GenEgressHTTPProtocolHandlerName(serviceName string) string {
	name := fmt.Sprintf("mesh-egress-%s", serviceName)
	return name
}

// ToIngressPipelineSpec will transfer service spec for a ingress pipeline
// about how to handle inner traffic , between Worker(Sidecar) and
// java process
func (s *Service) ToIngressPipelineSpec() (*supervisor.Spec, error) {
	var pipeline httppipeline.HTTPPipeline

	//[TODO]
	// generate a complete pipeline according to the service structure

	var (
		buff []byte
		err  error
	)
	if buff, err = yaml.Marshal(pipeline); err != nil {
		return nil, err
	}
	return supervisor.NewSpec(string(buff))
}

// EgressAddr formats sidecar's address according to the sidecar spec.
func (s *Service) EgressAddr() string {
	return fmt.Sprintf("%s://%s:%d", s.Sidecar.IngressProtocol, s.Sidecar.Address, s.Sidecar.EgressPort)
}

// ToEgressPipelineSpec will transfer service spec for a engress pipeline
// about how other relied serivces request it. It accpets service instances
// list to fill egress backend filter's IP pool.
func (s *Service) ToEgressPipelineSpec(insList []*ServiceInstanceSpec) (*supervisor.Spec, error) {
	var pipeline httppipeline.HTTPPipeline

	//[TODO]
	// generate a complete pipeline according to the service structure
	// e.g. ServerA requests ServiceB, call serviceB.ToEgressServerSpec() to
	// get a pipeline spec, and apply into ServiceA's memory

	var (
		buff []byte
		err  error
	)
	if buff, err = yaml.Marshal(pipeline); err != nil {
		return nil, err
	}
	return supervisor.NewSpec(string(buff))
}

// GenDefaultIngressPipelineYAML generates a default ingress pipeline yaml with
// provided java process's listerning port
func (s *Service) GenDefaultIngressPipelineYAML(port uint32) string {
	addr := fmt.Sprintf("%s://%s:%d", s.Sidecar.IngressProtocol, s.Sidecar.Address, port)

	pipelineSpec := fmt.Sprintf(DefaultIngressPipelineYAML, GenIngressPipelineObjectName(s.Name),
		GenIngressBackendFilterName(s.Name),
		GenIngressBackendFilterName(s.Name),
		addr)
	return pipelineSpec
}

// GenDefaultIngressHTTPServerYAML generates default ingress HTTP server with Service's Sidecar spec
func (s *Service) GenDefaultIngressHTTPServerYAML() string {
	httpsvrSpec := fmt.Sprintf(DefaultIngressHTTPServerYAML, GenIngressHTTPSvrObjectName(s.Name),
		s.Sidecar.IngressPort, GenIngressPipelineObjectName(s.Name))
	return httpsvrSpec
}

// GenDefaultEgressHTTPServerYAML generates default egress HTTP server with Service's Sidecar spe
func (s *Service) GenDefaultEgressHTTPServerYAML() string {
	httpsvcSpec := fmt.Sprintf(DefaultEgressHTTPServerYAML, GenEgressHTTPSvrObjectName(s.Name),
		s.Sidecar.EgressPort, GenEgressHTTPProtocolHandlerName(s.Name))

	return httpsvcSpec
}

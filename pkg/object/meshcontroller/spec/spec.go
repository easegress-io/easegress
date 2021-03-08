package spec

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/filter/backend"
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
)

var (
	// ErrParamNotMatch means RESTful request URL's object name or other fields are not matched in this request's body
	ErrParamNotMatch = fmt.Errorf("param in url and body's spec not matched")

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
	Resilience struct{}

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
		Enabled         bool   `yaml:"enabled" jsonschema:"required"`
		BootstrapServer string `yaml:"discoveryType" jsonschema:"required"`

		Tracing *ObservabilityTracing `yaml:"tracing" jsonschema:"omitempty"`
		Metric  *ObservabilityMetric  `yaml:"metric" jsonschema:"omitempty"`
	}

	// ObservabilityTracing is the tracing of observability.
	ObservabilityTracing struct {
		Topic        string                     `yaml:"topic" jsonschema:"required"`
		SampledByQPS int                        `yaml:"sampledByQPS" jsonschema:"required"`
		Request      ObservabilityTracingDetail `yaml:"request" jsonschema:"required"`
		RemoteInvoke ObservabilityTracingDetail `yaml:"remoteInvoke" jsonschema:"required"`
		Kafka        ObservabilityTracingDetail `yaml:"kafka" jsonschema:"required"`
		Jdbc         ObservabilityTracingDetail `yaml:"jdbc" jsonschema:"required"`
		Redis        ObservabilityTracingDetail `yaml:"redis" jsonschema:"required"`
		Rabbit       ObservabilityTracingDetail `yaml:"rabbit" jsonschema:"required"`
	}

	// ObservabilityTracingDetail is the tracing detail of observability.
	ObservabilityTracingDetail struct {
		ServicePrefix string `yaml:"servicePrefix" jsonschema:"required"`
	}

	// ObservabilityMetric is the metric of observability.
	ObservabilityMetric struct {
		Request        ObservabilityMetricDetail `yaml:"request" jsonschema:"required"`
		JdbcStatement  ObservabilityMetricDetail `yaml:"jdbcStatement" jsonschema:"required"`
		JdbcConnection ObservabilityMetricDetail `yaml:"jdbcConnection" jsonschema:"required"`
		Rabbit         ObservabilityMetricDetail `yaml:"rabbit" jsonschema:"required"`
		Kafka          ObservabilityMetricDetail `yaml:"kafka" jsonschema:"required"`
		Redis          ObservabilityMetricDetail `yaml:"redis" jsonschema:"required"`
	}

	// ObservabilityMetricDetail is the metric detail of observability.
	ObservabilityMetricDetail struct {
		Enabled  bool   `yaml:"enabled" jsonschema:"required"`
		Interval int    `yaml:"interval" jsonschema:"required"`
		Topic    string `yaml:"topic" jsonschema:"required"`
	}

	// Tenant contains the information of tenant.
	Tenant struct {
		Name string `yaml:"name"`

		ServicesList []string `yaml:"servicesList"`
		CreateTime   int64    `yaml:"createTime"`
		Description  string   `yaml:"description"`
	}

	// Heartbeat contains the information of heartbeat from one serivce instance.
	Heartbeat struct {
		LastActiveTime int64 `yaml:"lastActiveTime"`
	}

	// ServiceInstance one registry info of serivce
	ServiceInstance struct {
		// Provide by registry client
		ServiceName string `yaml:"serviceName" jsonschema:"required"`
		InstanceID  string `yaml:"instanceID" jsonschema:"required"`
		IP          string `yaml:"IP" jsonschema:"required"`
		Port        uint32 `yaml:"port" jsonschema:"required"`
		Tenant      string `yaml:"tenat" jsonschema:"required"`

		// Set by heartbeat timer event or API
		Status       string `yaml:"status" jsonschema:"omitempty"`
		Leases       int64  `yaml:"timestamp" jsonschema:"omitempty"`
		RegistryTime int64  `yaml:"registryTime" jsonschema:"omitempty"`
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

// ToEgressPipelineSpec will transfer service spec for a engress pipeline
// about other rely serivce how to request this service. It needs service instance
// list for fill egress backend filter's IP pool
func (s *Service) ToEgressPipelineSpec(insList []*ServiceInstance) (*supervisor.Spec, error) {
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
		s.Sidecar.EgressPort)

	return httpsvcSpec
}

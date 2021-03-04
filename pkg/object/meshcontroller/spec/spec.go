package spec

import (
	"fmt"
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
		Name           string `yaml:"name" json:"name,omitempty"`
		RegisterTenant string `yaml:"registerTenant" json:"register_tenant,omitempty"`

		Resilience    Resilience    `yaml:"resilience" jsonschema:"omitempty"`
		Canary        Canary        `yaml:"canary" jsonschema:"canary"`
		LoadBalance   LoadBalance   `yaml:"loadBalance" jsonschema:"load"`
		Sidecar       Sidecar       `yaml:"sidecar"`
		Observability Observability `yaml:"observability"`
	}

	// Resilience is the spec of service resilience.
	Resilience struct{}

	// Canary is the spec of service canary.
	Canary struct{}

	// LoadBalance is the spec of service load balance.
	LoadBalance struct{}

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
		LastActiveTime int64 `yaml:"lastActive"`
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

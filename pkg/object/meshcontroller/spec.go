package meshcontroller

import "fmt"

const (
	meshRoleMaster = "master"
	meshRoleWorker = "worker"

	meshSerivceInstanceLocalIP = "127.0.0.1" // The Java process is always in the same host with sidecar

	// basic specs
	meshServicePrefix              = "/mesh/service/%s"               // +serviceName (its value is the basic mesh spec)
	meshServiceResiliencePrefix    = "/mesh/service/%s/resilience"    // +serviceName(its value is the mesh resilience spec)
	meshServiceCanaryPrefix        = "/mesh/service/%s/canary"        // + serviceName(its value is the mesh canary spec)
	meshServiceLoadBalancerPrefix  = "/mesh/service/%s/loadBalancer"  //+ serviceName(its value is the mesh loadBalance spec)
	meshServiceSidecarPrefix       = "/mesh/serivce/%s/sidecar"       // +serviceName (its value is the sidecar spec)
	meshServiceObservabilityPrefix = "/mesh/service/%s/observability" // + serviceName(its value is the observability spec)

	// traffic gate about
	meshServiceIngressHTTPServerPrefix = "/mesh/service/%s/ingress/httpserver" // +serviceName (its value is the ingress httpserver spec)
	meshServiceIngressPipelinePrefix   = "/mesh/service/%s/ingress/pipeline"   // +serviceName (its value is the ingress pipeline spec)
	meshServiceEgressHTTPServerPrefix  = "/mesh/service/%s/egress/httpserver"  // +serviceName (its value is the egress httpserver spec)
	meshServiceEgressPipelinePrefix    = "/mesh/service/%s/egress/pipeline/%s" //+serviceName + RPC target serviceName(which service's this pipeline point to)

	// registry about
	meshServiceInstancePrefix         = "/mesh/service/%s/instance/%s"           // +serviceName + instanceID( its value is one instance registry info)
	meshServiceInstanceHearbeatPrefix = "/mesh/service/%s/instance/%s/heartbeat" // + serviceName + instanceID (its value is one instance heartbeat info)
	meshTenantServiceListPrefix       = "/mesh/tenant/%s"                        // +tenantName (its value is a service name list belongs to this tenant)

	meshServiceInstanceEtcdLockPrefix = "/mesh/lock/instances/%s" // +instanceID, for locking one service instances's record
)

var (

	// ErrParamNotMatch means RESTful request URL's object name or other fields are not matched in this request's body
	ErrParamNotMatch = fmt.Errorf("param in url and body's spec not matched")
)

type (

	// Spec describes MeshController.
	Spec struct {

		// ServiceWatchInterval is the interval for watcing all service instance heartbeat record
		ServiceWatchInterval string `yaml:"watchInterval" jsonschema:"required,format=duration"`

		// Role as slave's configurations start -----
		// HeartbeatInterval is the interval for one service instance reports its hearbeat
		HeartbeatInterval string `yaml:"heartbeatInterval" jsonschema:"required,format=duration"`

		RegistryType string `yaml:"RegistryType" jsonschema:"required"`

		// SerivceName indicates which serivce the worker(sidecar) stands for
		ServiceName string `yaml:"serivceName"`
	}

	// MeshServiceSpec describes the mesh service basic info, its name, which tenant it belongs to
	MeshServiceSpec struct {
		// Which tenant this service belongs to
		Name           string `yaml:"name" jsonschema:"required"`
		RegisterTenant string `yaml:"resigtTenant" jsonschema:"required"`
	}

	// TenantSpec describes the tenant's basic info, and which services it has
	TenantSpec struct {
		Name string `yaml:"name" josnschema:"required"`

		ServicesList []string `yaml:"servicesList" jsonschema:"required"`
		CreateTime   int64    `yaml:"createTime" jsonschema:"omitempty"`
	}

	// Resilience

	// SidecarSpec is the sidecar spec
	SidecarSpec struct {
		discoveryType   string `yaml:"discoveryType" josnschema:"required"`
		Address         string `yaml:"address" jsonschema:"required"`
		IngressPort     int    `yaml:"ingressPort" jsonschema:"required"`
		IngressProtocol string `yaml:"ingressProtocol" jsonschema:"required"`
		EgressPort      int    `yaml:"egressPort" jsonschema:"required"`
		EgressProtocol  string `yaml:"egressProtocol" jsonschema:"required"`
	}
)

// Validate validates Spec.
func (spec *Spec) Validate() error {

	if spec.RegistryType != RegistryTypeConsul && spec.RegistryType != RegistryTypeEureka {
		return fmt.Errorf("unknow registry center type :%s", spec.RegistryType)
	}
	return nil
}

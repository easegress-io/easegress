package meshcontroller

import (
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/ingresscontroller"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/label"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/master"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/worker"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Category is the category of MeshController.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of MeshController.
	Kind = "MeshController"
)

type (
	// MeshController is a business controller to complete MegaEase Service Mesh.
	MeshController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *spec.Admin

		role              string
		master            *master.Master
		worker            *worker.Worker
		ingressController *ingresscontroller.IngressController
	}
)

func init() {
	supervisor.Register(&MeshController{})
}

// Category returns the category of MeshController.
func (mc *MeshController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of MeshController.
func (mc *MeshController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of MeshController.
func (mc *MeshController) DefaultSpec() interface{} {
	return &spec.Admin{
		HeartbeatInterval: spec.HeartbeatInterval,
		RegistryType:      spec.RegistryTypeEureka,
		APIPort:           spec.WorkerAPIPort,
	}
}

// Init initializes MeshController.
func (mc *MeshController) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	mc.superSpec, mc.spec, mc.super = superSpec, superSpec.ObjectSpec().(*spec.Admin), super
	mc.reload()
}

// Inherit inherits previous generation of MeshController.
func (mc *MeshController) Inherit(spec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	mc.Init(spec, super)
}

func (mc *MeshController) reload() {
	meshRole := mc.super.Options().Labels[label.KeyRole]
	serviceName := mc.super.Options().Labels[label.KeyServiceName]

	switch meshRole {
	case "", label.ValueRoleMaster, label.ValueRoleWorker:
		if serviceName == "" {
			meshRole = label.ValueRoleMaster
		} else {
			meshRole = label.ValueRoleWorker
		}
	case label.ValueRoleIngressController:
		// ingress controller does not care about service name
		break
	default:
		logger.Errorf("%s unsupported mesh role: %s (master, worker, ingressController)",
			mc.superSpec.Name(), meshRole)
		logger.Infof("%s use default mesh role: master", mc.superSpec.Name())
		meshRole = label.ValueRoleMaster
	}

	switch meshRole {
	case label.ValueRoleMaster:
		logger.Infof("%s running in master role", mc.superSpec.Name())
		mc.role = label.ValueRoleMaster
		mc.master = master.New(mc.superSpec, mc.super)

	case label.ValueRoleWorker:
		logger.Infof("%s running in worker role", mc.superSpec.Name())
		mc.role = label.ValueRoleWorker
		mc.worker = worker.New(mc.superSpec, mc.super)

	case label.ValueRoleIngressController:
		logger.Infof("%s running in ingress controller role", mc.superSpec.Name())
		mc.role = label.ValueRoleIngressController
		mc.ingressController = ingresscontroller.New(mc.superSpec, mc.super)
	}
}

// Status returns the status of MeshController.
func (mc *MeshController) Status() *supervisor.Status {
	if mc.master != nil {
		return mc.master.Status()
	}

	if mc.worker != nil {
		return mc.worker.Status()
	}

	return mc.ingressController.Status()
}

// Close closes MeshController.
func (mc *MeshController) Close() {
	if mc.master != nil {
		mc.master.Close()
		return
	}

	if mc.worker != nil {
		mc.worker.Close()
		return
	}

	if mc.ingressController != nil {
		mc.ingressController.Close()
		return
	}
}

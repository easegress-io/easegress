package meshcontroller

import (
	"github.com/megaease/easegateway/pkg/logger"
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

	meshRoleMaster = "master"
	meshRoleWorker = "worker"
)

type (
	// MeshController is a business controller to complete MegaEase Service Mesh.
	MeshController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *spec.Admin

		role   string
		master *master.Master
		worker *worker.Worker
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
	service := mc.super.Options().Labels["mesh-servicename"]

	if len(service) == 0 {
		logger.Infof("%s running in master role", mc.superSpec.Name())
		mc.role = meshRoleMaster
		mc.master = master.New(mc.superSpec, mc.super)
		return
	}

	logger.Infof("%s running in worker role", mc.superSpec.Name())
	mc.role = meshRoleWorker
	mc.worker = worker.New(mc.superSpec, mc.super)
}

// Status returns the status of MeshController.
func (mc *MeshController) Status() *supervisor.Status {
	if mc.master != nil {
		return mc.master.Status()
	}

	return mc.worker.Status()
}

// Close closes MeshController.
func (mc *MeshController) Close() {
	if mc.master != nil {
		mc.master.Close()
		return
	}

	mc.worker.Close()
}

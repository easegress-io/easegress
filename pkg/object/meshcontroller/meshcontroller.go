package meshcontroller

import (
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
		spec      *Spec

		done chan struct{}
	}

	// Spec describes MeshController.
	Spec struct {
	}
)

func init() {
	supervisor.Register(&MeshController{})
}

// Category returns the category of MeshController.
func (ssc *MeshController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of MeshController.
func (ssc *MeshController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of MeshController.
func (ssc *MeshController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes MeshController.
func (ssc *MeshController) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	ssc.superSpec, ssc.spec, ssc.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	ssc.reload()
}

// Inherit inherits previous generation of MeshController.
func (ssc *MeshController) Inherit(spec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	ssc.Init(spec, super)
}

func (ssc *MeshController) reload() {
	go ssc.run()
}

func (ssc *MeshController) run() {
	for {
		select {
		case <-ssc.done:
			return
		}
	}
}

// Status returns the status of MeshController.
func (ssc *MeshController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Close closes MeshController.
func (ssc *MeshController) Close() {
	close(ssc.done)
}

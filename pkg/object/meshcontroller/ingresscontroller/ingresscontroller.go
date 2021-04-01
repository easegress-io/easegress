package ingresscontroller

import (
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/supervisor"
)

type (
	// IngressController is the ingress controller.
	IngressController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *spec.Admin
	}
)

// New creates a mesh ingress controller.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *IngressController {
	return &IngressController{}
}

func (ic *IngressController) Close() {

}

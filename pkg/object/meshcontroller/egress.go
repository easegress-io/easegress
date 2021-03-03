package meshcontroller

import "github.com/megaease/easegateway/pkg/supervisor"

type (
	// EgressServer handle egress traffic gate
	EgressServer struct {
		super supervisor.Supervisor
		store MeshStorage
	}

	// EngressMsg is the engress notify message wrappered with basic
	// store message
	EngressMsg struct {
		storeMsg storeOpMsg // original store notify operatons

	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(store MeshStorage, super *supervisor.Supervisor) *IngressServer {
	return &IngressServer{
		store: store,
		super: super,
	}
}

func (es *EgressServer) updateEgress(specs map[string]string) {

	return
}

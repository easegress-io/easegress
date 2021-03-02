package meshcontroller

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/supervisor"
)

// genereate the EG running object name
func genIngressPipelineObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-service-ingress-%s-pipeline", serviceName)
	return name
}
func genIngressHTTPSvrObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-service-ingress-%s-httpserver", serviceName)
	return name
}

// generate the mesh spec pipeline name
func genIngreePipelineSpecName(serviceName string) string {
	return fmt.Sprintf(meshServiceIngressPipelinePrefix, serviceName)
}

func genHTTPServerSpecName(serviceName string) string {

	return fmt.Sprintf(meshServiceIngressHTTPServerPrefix, serviceName)
}

type (
	IngressMsg struct {
		storeMsg storeOpMsg // original store notify operatons

		// for creating request
		serviceName  string
		instancePort uint32
	}

	// IngressServer control one ingress pipeline and one HTTPServer
	IngressServer struct {
		store MeshStorage
		super *supervisor.Supervisor

		// only one ingress need to watch
		watchIngressPipelineName []string

		// InstanceRealPort is the Java process realy listening port
		InstanceRealPort uint32
	}
)

func NewDefualtIngressServer(store MeshStorage, super *supervisor.Supervisor) *IngressServer {
	return &IngressServer{
		store: store,
		super: super,
	}
}

func (ings *IngressServer) HandleIngressOpMsg(msg IngressMsg) error {
	switch msg.storeMsg.op {
	case opTypeCreate:
		err := ings.createIngress(msg)
		return err

	default:
	}

	return nil
}

// SetIngressPipelinePort sets the real Java process listening port provided by
// registry request
func (ings *IngressServer) SetIngressPipelinePort(port uint32) {
	ings.InstanceRealPort = port
}

// createIngress creates one default pipeline and httpservice for ingress
func (ings *IngressServer) createIngress(msg IngressMsg) error {
	var err error
	// get ingress pipeline spec

	//[TODO]: call supervisor to create pipeline if the Pipeline exist locally, do nothing

	//[TODO]: call supervisor to create HTTPServer, if the HTTPServer exist locally, do nothing

	return err
}

func (ings *IngressServer) updateIngress() error {
	var err error

	return err
}

func (ings *IngressServer) deleteIngress() error {
	var err error

	return err
}

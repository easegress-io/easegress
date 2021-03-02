package meshcontroller

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/supervisor"
)

func genIngreePipelineName(serviceName string) string {
	return fmt.Sprintf(meshServiceIngressPipelinePrefix, serviceName)
}

func genHTTPServerName(serviceName string) string {

	return fmt.Sprintf(meshServiceIngressHTTPServerPrefix, serviceName)
}

type (
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

// SetIngressPipelinePort sets the real Java process listening port provided by
// registry request
func (ings *IngressServer) SetIngressPipelinePort(port uint32) {
	ings.InstanceRealPort = port
}

// createIngress creates one default pipeline and httpservice for ingress
func (ings *IngressServer) createIngress(serviceName string, instanceID string, instancePort uint32) error {
	var (
		err            error
		pipelineSpec   string
		pipelineName   string = genIngreePipelineName(serviceName)
		HTTPServerSpec string
		HTTPServerName string = genHTTPServerName(serviceName)
	)

	if pipelineSpec, err = ings.store.Get(pipelineName); err != nil {
		logger.Errorf("read service %s's ingress pipeline spec failed, %v", serviceName, err)
		return err
	}

	logger.Debugf("get pipeline secp %s", pipelineSpec)
	// TODO: call supervisor to create pipeline if the Pipeline exist locally, do nothing

	// call supervisor to create httpservice
	if HTTPServerSpec, err = ings.store.Get(HTTPServerName); err != nil {
		logger.Errorf("read service %s's ingress HTTPServer spec failed, %v", serviceName, err)
		return err
	}

	logger.Debugf("get HTTP server spec %s", HTTPServerSpec)

	// TODO: call supervisor to create HTTPServer, if the HTTPServer exist locally, do nothing

	return err
}

func (ings *IngressServer) updateIngress(specs map[string]string) error {
	var err error

	return err
}

func (ings *IngressServer) deleteIngress() error {
	var err error

	return err
}

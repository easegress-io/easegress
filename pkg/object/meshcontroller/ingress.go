package meshcontroller

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"
)

func genIngreePipelineName(serviceName string) string {
	return fmt.Sprintf(meshServiceIngressPipelinePrefix, serviceName)
}

func genHTTPServerName(serviceName string) string {

	return fmt.Sprintf(meshServiceIngressHTTPServerPrefix, serviceName)
}

// IngressServer control one ingress pipeline and one HTTPServer
type IngressServer struct {
	store MeshStorage
}

// createIngress creates one default pipeline and httpservice for ingress
func (is *IngressServer) createIngress(serviceName string, instanceID string, instancePort uint32) error {
	var (
		err            error
		pipelineSpec   string
		pipelineName   string = genIngreePipelineName(serviceName)
		HTTPServerSpec string
		HTTPServerName string = genHTTPServerName(serviceName)
	)

	if pipelineSpec, err = is.store.Get(pipelineName); err != nil {
		logger.Errorf("read service %s's ingress pipeline spec failed, %v", serviceName, err)
		return err
	}

	logger.Debugf("get pipeline secp %s", pipelineSpec)
	// TODO: call supervisor to create pipeline if the Pipeline exist locally, do nothing

	// call supervisor to create httpservice
	if HTTPServerSpec, err = is.store.Get(HTTPServerName); err != nil {
		logger.Errorf("read service %s's ingress HTTPServer spec failed, %v", serviceName, err)
		return err
	}

	logger.Debugf("get HTTP server spec %s", HTTPServerSpec)

	// TODO: call supervisor to create HTTPServer, if the HTTPServer exist locally, do nothing

	return err
}

func (mb *MeshController) updateIngress(specs map[string]string) error {
	var err error

	return err
}

package gateway

import "cluster"

func (gc *GatewayCluster) handleWriteModeOperation(req *cluster.RequestEvent) {
	for {
		// TODO: when the request has been gossiped, make a new goroutine
		// to wait reponse then respond to rest server.
	}

}

func (gc *GatewayCluster) handleReadModeOperation(req *cluster.RequestEvent) {
	for {
		// TODO
	}
}

package nrw

import "cluster"

func (nrw *NRW) handleWriteModeOperation(req *cluster.RequestEvent) {
	for {
		// TODO: when the request has been gossiped, make a new goroutine
		// to wait reponse then respond to rest server.
	}

}

func (nrw *NRW) handleReadModeOperation(req *cluster.RequestEvent) {
	for {
		// TODO
	}
}

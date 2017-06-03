package gateway

import (
	"fmt"
	"time"

	"cluster"
	"logger"
	"model"
)

type Mode string

const (
	WriteMode Mode = "WriteMode"
	ReadMode  Mode = "ReadMode"
)

type Config struct {
	// MaxSeqGapToSync means ... TODO
	MaxSeqGapToSync   uint64
	SyncOPLogInterval time.Duration
}

type GatewayCluster struct {
	conf    *Config
	mod     *model.Model
	cluster *cluster.Cluster
	log     *opLog
	mode    Mode

	eventStream chan cluster.Event
}

func NewGatewayCluster(conf Config, mod *model.Model) (*GatewayCluster, error) {
	if mod == nil {
		return nil, fmt.Errorf("model is nil")
	}

	eventStream := make(chan cluster.Event)

	// TODO: choose config of under layer automatically
	basisConf := cluster.DefaultLANConfig()
	basisConf.EventStream = eventStream
	basisConf.NodeTags["group"] = "default"        // TODO: read from config
	basisConf.NodeTags["mode"] = string(WriteMode) // TODO: read from config

	basis, err := cluster.Create(*basisConf)
	if err != nil {
		return nil, err
	}

	log, err := newOPLog()
	if err != nil {
		return nil, err
	}

	gc := &GatewayCluster{
		conf:    &conf,
		mod:     mod,
		cluster: basis,
		log:     log,
		mode:    WriteMode, // TODO

		eventStream: eventStream,
	}

	go gc.syncOPLog()
	go gc.dispatch()

	return gc, nil
}

func (gc *GatewayCluster) Mode() Mode {
	return gc.mode
}

func (gc *GatewayCluster) dispatch() {
	for {
		event := <-gc.eventStream
		switch event := event.(type) {
		case *cluster.RequestEvent:
			if len(event.RequestPayload) < 1 {
				break
			}
			msgType := event.RequestPayload[0]

			switch MessageType(msgType) {
			case queryMaxSeqMessage:
				go gc.handleQueryMaxSeq(event)
			case operationMessage:
				if gc.Mode() == WriteMode {
					go gc.handleWriteModeOperation(event)
				} else if gc.Mode() == ReadMode {
					go gc.handleReadModeOperation(event)
				}
			case retrieveMessage:
				if gc.Mode() == WriteMode {
					go gc.handleWriteModeRetrieve(event)
				} else if gc.Mode() == ReadMode {
					go gc.handleReadModeRetrieve(event)
				}
			case statMessage:
				go gc.handleStat(event)
			case syncOPLogMessage:
				gc.handleSyncOPLog(event)
			}
		case *cluster.MemberEvent:
			// TODO: notify current executing updateBusiness or let it go
			// Maybe use observer pattern.
		}
	}
}

func (gc *GatewayCluster) handleQueryMaxSeq(req *cluster.RequestEvent) {
	ms := gc.log.maxSeq()
	payload, err := cluster.Pack(RespQueryMaxSeq(ms), uint8(queryMaxSeqMessage))
	if err != nil {
		logger.Errorf("[BUG: pack max sequence %d failed: %s]", ms, err)
		return
	}
	err = req.Respond(payload)
	if err != nil {
		logger.Errorf("[repond reqeust max sequence to request %s, node %s failed: %s]",
			req.RequestName, req.RequestNodeName, err)
	}
}

func (gc *GatewayCluster) Close() {
	gc.log.close()
}

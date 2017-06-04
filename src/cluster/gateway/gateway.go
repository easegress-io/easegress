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

	groupTagKey   = "group"
	nrwModeTagKey = "nrw_mode"
)

type Config struct {
	// MaxSeqGapToSync means ... TODO
	MaxSeqGapToSync   uint64
	SyncOPLogInterval time.Duration
}

type GatewayCluster struct {
	conf     *Config
	mod      *model.Model
	cluster  *cluster.Cluster
	log      *opLog
	mode     Mode
	stopChan chan struct{}

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
	basisConf.NodeTags[groupTagKey] = "default"           // TODO: read from config
	basisConf.NodeTags[nrwModeTagKey] = string(WriteMode) // TODO: read from config

	basis, err := cluster.Create(*basisConf)
	if err != nil {
		return nil, err
	}

	log, err := newOPLog()
	if err != nil {
		return nil, err
	}

	gc := &GatewayCluster{
		conf:     &conf,
		mod:      mod,
		cluster:  basis,
		log:      log,
		mode:     WriteMode, // TODO
		stopChan: make(chan struct{}),

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
loop:
	for {
		select {
		case <-gc.stopChan:
			break loop
		case event := <-gc.eventStream:
			switch event := event.(type) {
			case *cluster.RequestEvent:
				if len(event.RequestPayload) < 1 {
					break
				}
				msgType := event.RequestPayload[0]

				switch MessageType(msgType) {
				case queryGroupMaxSeqMessage:
					go gc.handleQueryGroupMaxSeq(event)

				case operationMessage:
					if gc.Mode() == WriteMode {
						go gc.handleOperation(event)
					}
					logger.Errorf("[BUG: read mode but received operationMessage]")
				case operationRelayedMessage:
					if gc.Mode() == ReadMode {
						go gc.handleOperationRelayed(event)
					}
					logger.Errorf("[BUG: write mode but received operationRelayedMessage]")

				case retrieveMessage:
					if gc.Mode() == WriteMode {
						go gc.handleRetrieve(event)
					}
					logger.Errorf("[BUG: read mode but received retrieveMessage]")
				case retrieveRelayedMessage:
					if gc.Mode() == ReadMode {
						go gc.handleRetrieveRelayed(event)
					}
					logger.Errorf("[BUG: write mode but received retrieveRelayedMessage]")

				case statMessage:
					if gc.Mode() == WriteMode {
						go gc.handleStat(event)
					}
					logger.Errorf("[BUG: read mode but received statMessage]")
				case statRelayedMessage:
					if gc.Mode() == ReadMode {
						go gc.handleStatRelayed(event)
					}
					logger.Errorf("[BUG: write mode but received statRelayedMessage]")

				case pullOPLogMessage:
					go gc.handlePullOPLog(event)
				}
			case *cluster.MemberEvent:
				// Do not handle MemberEvent for the time being.
			}
		}
	}
}

func (gc *GatewayCluster) GetOPLog() *opLog {
	return gc.log
}

func (gc *GatewayCluster) handleQueryGroupMaxSeq(req *cluster.RequestEvent) {
	ms := gc.log.maxSeq()
	payload, err := cluster.PackWithHeader(RespQueryGroupMaxSeq(ms), uint8(queryGroupMaxSeqMessage))
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

func (gc *GatewayCluster) Stop() error {
	err := gc.log.close()
	if err != nil {
		return err
	}

	err = gc.cluster.Stop()
	if err != nil {
		return err
	}

	close(gc.stopChan)

	return nil
}

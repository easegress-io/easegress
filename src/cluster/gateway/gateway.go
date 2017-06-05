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

	groupTagKey = "group"
	modeTagKey  = "mode"
)

type Config struct {
	// MaxSeqGapToSync means ... TODO
	MaxSeqGapToSync       uint64
	PullOPLogMaxCountOnce uint64
	SyncOPLogInterval     time.Duration
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
	basisConf.NodeTags[groupTagKey] = "default"        // TODO: read from config
	basisConf.NodeTags[modeTagKey] = string(WriteMode) // TODO: read from config

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

	go gc.dispatch()
	go gc.syncOPLog()

	return gc, nil
}

func (gc *GatewayCluster) Mode() Mode {
	return gc.mode
}

func (gc *GatewayCluster) dispatch() {
LOOP:
	for {
		select {
		case <-gc.stopChan:
			break LOOP
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
					logger.Errorf("[BUG: node with read mode received operationMessage]")
				case operationRelayMessage:
					if gc.Mode() == ReadMode {
						go gc.handleOperationRelay(event)
					}
					logger.Errorf("[BUG: node with write mode received operationRelayMessage]")
				case retrieveMessage:
					if gc.Mode() == WriteMode {
						go gc.handleRetrieve(event)
					}
					logger.Errorf("[BUG: node with read mode received retrieveMessage]")
				case retrieveRelayMessage:
					if gc.Mode() == ReadMode {
						go gc.handleRetrieveRelay(event)
					}
					logger.Errorf("[BUG: node with write mode received retrieveRelayMessage]")
				case statMessage:
					if gc.Mode() == WriteMode {
						go gc.handleStat(event)
					}
					logger.Errorf("[BUG: node with read mode received statMessage]")
				case statRelayMessage:
					if gc.Mode() == ReadMode {
						go gc.handleStatRelay(event)
					}
					logger.Errorf("[BUG: node with write mode received statRelayMessage]")
				case opLogPullMessage:
					go gc.handlePullOPLog(event)
				}
			case *cluster.MemberEvent:
				// Do not handle MemberEvent for the time being.
			}
		}
	}
}

func (gc *GatewayCluster) OPLog() *opLog {
	return gc.log
}

func (gc *GatewayCluster) localGroupName() string {
	return gc.cluster.GetConfig().NodeTags[groupTagKey]
}

func (gc *GatewayCluster) otherSameGroupMembers() []cluster.Member {
	totalMembers := gc.cluster.Members()

	members := make([]cluster.Member, 0)

	groupName := gc.localGroupName()
	for _, member := range totalMembers {
		if member.NodeTags[groupTagKey] == groupName {
			members = append(members, member)
		}
	}

	return members
}

func (gc *GatewayCluster) handleQueryGroupMaxSeq(req *cluster.RequestEvent) {
	ms := gc.log.maxSeq()
	payload, err := cluster.PackWithHeader(RespQueryGroupMaxSeq(ms), uint8(queryGroupMaxSeqMessage))
	if err != nil {
		logger.Errorf("[BUG: PackWithHeader max sequence %d failed: %v]", ms, err)
		return
	}
	err = req.Respond(payload)
	if err != nil {
		logger.Errorf("[repond max sequence to request %s, node %s failed: %v]",
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

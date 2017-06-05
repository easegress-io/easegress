package gateway

import (
	"fmt"
	"sync"
	"time"

	"cluster"
	"logger"
	"model"
)

type Mode string

func (m Mode) String() string {
	return string(m)
}

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
	SyncOPLogTimeout      time.Duration
}

type GatewayCluster struct {
	conf        *Config
	mod         *model.Model
	clusterConf *cluster.Config
	cluster     *cluster.Cluster
	log         *opLog
	mode        Mode

	statusLock sync.Mutex
	stopChan   chan struct{}
	stopped    bool

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
		conf:        &conf,
		mod:         mod,
		clusterConf: basisConf,
		cluster:     basis,
		log:         log,
		mode:        WriteMode, // TODO
		stopChan:    make(chan struct{}),

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
		case <-gc.stopChan:
			break LOOP
		}
	}
}

func (gc *GatewayCluster) OPLog() *opLog {
	return gc.log
}

func (gc *GatewayCluster) localGroupName() string {
	return gc.cluster.GetConfig().NodeTags[groupTagKey]
}

func (gc *GatewayCluster) nonAliveMember(nodeName string) bool {
	totalMembers := gc.cluster.Members()

	for _, member := range totalMembers {
		if member.NodeName == nodeName &&
			member.Status != cluster.MemberAlive {

			return true
		}
	}

	return false
}

func (gc *GatewayCluster) restAliveMembersInSameGroup() (ret []cluster.Member) {
	totalMembers := gc.cluster.Members()
	groupName := gc.localGroupName()

	for _, member := range totalMembers {
		if member.NodeTags[groupTagKey] == groupName &&
			member.Status == cluster.MemberAlive &&
			member.NodeName != gc.clusterConf.NodeName {
			ret = append(ret, member)
		}
	}

	return ret
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
	gc.statusLock.Lock()
	defer gc.statusLock.Unlock()

	if gc.stopped {
		return fmt.Errorf("already stopped")
	}

	err := gc.log.close()
	if err != nil {
		return err
	}

	err = gc.cluster.Stop()
	if err != nil {
		return err
	}

	close(gc.stopChan)

	gc.stopped = true

	return nil
}

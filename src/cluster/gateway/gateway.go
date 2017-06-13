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
	OPLogMaxSeqGapToPull  uint64
	OPLogPullMaxCountOnce uint64
	OPLogPullInterval     time.Duration
	OPLogPullTimeout      time.Duration
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

	if gc.Mode() == ReadMode {
		go gc.syncOPLog()
	}

	return gc, nil
}

func (gc *GatewayCluster) Mode() Mode {
	return gc.mode
}

func (gc *GatewayCluster) OPLog() *opLog {
	return gc.log
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
					go gc.handleStat(event)
				case statRelayMessage:
					go gc.handleStatRelay(event)
				case opLogPullMessage:
					go gc.handleOPLogPull(event)
				}
			case *cluster.MemberEvent:
				switch event.Type() {
				case cluster.MemberJoinEvent:
					logger.Infof("[member %s (group=%s, mode=%s) joined to the cluster]",
						event.Member.NodeName, event.Member.NodeTags[groupTagKey],
						event.Member.NodeTags[modeTagKey])
				case cluster.MemberLeftEvent:
					logger.Infof("[member %s (group=%s, mode=%s) left from the cluster]",
						event.Member.NodeName, event.Member.NodeTags[groupTagKey],
						event.Member.NodeTags[modeTagKey])
				case cluster.MemberFailedEvent:
					logger.Warnf("[member %s (group=%s, mode=%s) failed in the cluster]",
						event.Member.NodeName, event.Member.NodeTags[groupTagKey],
						event.Member.NodeTags[modeTagKey])
				case cluster.MemberUpdateEvent:
					logger.Infof("[member %s (group=%s, mode=%s) updated in the cluster]",
						event.Member.NodeName, event.Member.NodeTags[groupTagKey],
						event.Member.NodeTags[modeTagKey])
				case cluster.MemberCleanupEvent:
					logger.Debugf("[member %s (group=%s, mode=%s) record is cleaned up]",
						event.Member.NodeName, event.Member.NodeTags[groupTagKey],
						event.Member.NodeTags[modeTagKey])
				}
			}
		case <-gc.stopChan:
			break LOOP
		}
	}
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

// recordResp just records known response of member and ignore others.
// It does its best to record response, and just exits when GatewayCluster stopped
// or future got timeout, the caller could check membersRespBook to get the result.
func (gc *GatewayCluster) recordResp(requestName string, future *cluster.Future, membersRespBook map[string][]byte) {
	// The type is signed owe to the value could be -1 temporarily.
	var memberRespCount int = 0
LOOP:
	for ; memberRespCount < len(membersRespBook); memberRespCount++ {
		select {
		case memberResp, ok := <-future.Response():
			if !ok {
				break LOOP
			}

			payload, known := membersRespBook[memberResp.ResponseNodeName]
			if !known {
				logger.Warnf(
					"[received the response from an unexpexted node %s started durning the request %s]",
					memberResp.ResponseNodeName, fmt.Sprintf("%s_relayed", requestName))
				continue LOOP
			}

			if payload != nil {
				logger.Errorf("[received multiple response from node %s for request %s, skipped. "+
					"probably need to tune cluster configuration]",
					memberResp.ResponseNodeName, fmt.Sprintf("%s_relayed", requestName))
				memberRespCount--
				continue LOOP
			}

			if memberResp.Payload != nil {
				logger.Errorf("[BUG: received empty response from node %s for request %s]",
					memberResp.ResponseNodeName, fmt.Sprintf("%s_relayed", requestName))

				memberResp.Payload = []byte("")
			}

			membersRespBook[memberResp.ResponseNodeName] = memberResp.Payload
		case <-gc.stopChan:
			break LOOP
		}
	}

	return
}

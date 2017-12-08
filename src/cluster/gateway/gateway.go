package gateway

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/logutils"

	"cluster"
	"common"
	"logger"
	"model"
	"option"
)

type Mode string

func (m Mode) String() string {
	return string(m)
}

const (
	WriteMode Mode = "Write"
	ReadMode  Mode = "Read"

	groupTagKey = "group"
	modeTagKey  = "mode"
)

type Config struct {
	ClusterGroup      string
	ClusterMemberMode Mode
	ClusterMemberName string
	Peers             []string

	OPLogMaxSeqGapToPull  uint16
	OPLogPullMaxCountOnce uint16
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

	statusLock sync.RWMutex
	stopChan   chan struct{}
	stopped    bool

	syncOpLogLock sync.Mutex

	eventStream chan cluster.Event
}

func NewGatewayCluster(conf Config, mod *model.Model) (*GatewayCluster, error) {
	if mod == nil {
		return nil, fmt.Errorf("model is nil")
	}

	switch {
	case len(conf.ClusterGroup) == 0:
		return nil, fmt.Errorf("empty group")
	case conf.OPLogMaxSeqGapToPull == 0:
		return nil, fmt.Errorf("oplog_max_seq_gap_to_pull must be greater then 0")
	case conf.OPLogPullMaxCountOnce == 0:
		return nil, fmt.Errorf("oplog_pull_max_count_once must be greater then 0")
	case conf.OPLogPullInterval == 0:
		return nil, fmt.Errorf("oplog_pull_interval must be greater than 0")
	case conf.OPLogPullTimeout.Seconds() < 10:
		return nil, fmt.Errorf("oplog_pull_timeout must be greater than or equals to 10")
	}

	eventStream := make(chan cluster.Event)

	// TODO: choose config of under layer automatically
	basisConf := cluster.DefaultLANConfig()
	basisConf.NodeName = conf.ClusterMemberName
	basisConf.NodeTags[groupTagKey] = conf.ClusterGroup
	basisConf.NodeTags[modeTagKey] = conf.ClusterMemberMode.String()
	basisConf.BindAddress = option.ClusterHost
	basisConf.AdvertiseAddress = option.ClusterHost

	if common.StrInSlice(basisConf.AdvertiseAddress, []string{"127.0.0.1", "localhost", "0.0.0.0"}) {
		return nil, fmt.Errorf("invalid advertise address %s, it should be reachable from peer",
			basisConf.AdvertiseAddress)
	}

	var minLogLevel logutils.LogLevel
	if option.Stage == "debug" {
		minLogLevel = logutils.LogLevel("DEBUG")
	} else {
		minLogLevel = logutils.LogLevel("WARN")
	}
	basisConf.LogOutput = &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARN", "ERROR"},
		MinLevel: minLogLevel,
		Writer:   logger.Writer(),
	}
	basisConf.EventStream = eventStream

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
		mode:        conf.ClusterMemberMode,
		stopChan:    make(chan struct{}),

		eventStream: eventStream,
	}

	go func() {
		select {
		case <-gc.stopChan:
			return
		case <-gc.cluster.Stopped():
			logger.Warnf("[stop the gateway cluster internally due to basis cluster is gone]")
			gc.internalStop(false)
		}
	}()

	go gc.dispatch()

	if len(conf.Peers) > 0 {
		logger.Infof("[start to join peer member(s): %v]", conf.Peers)

		connected, err := basis.Join(conf.Peers)
		if err != nil {
			logger.Errorf("[join peer member(s) failed, running in standalone mode: %v]", err)
		} else {
			logger.Infof("[peer member(s) joined, connected to %d member(s) totally]", connected)
		}
	}

	if gc.Mode() == ReadMode {
		go gc.syncOpLogLoop()
	}

	return gc, nil
}

func (gc *GatewayCluster) NodeName() string {
	return gc.clusterConf.NodeName
}

func (gc *GatewayCluster) Mode() Mode {
	return gc.mode
}

func (gc *GatewayCluster) OPLog() *opLog {
	return gc.log
}

func (gc *GatewayCluster) Stop() error {
	return gc.internalStop(true)
}

func (gc *GatewayCluster) internalStop(stopBasis bool) error {
	gc.statusLock.Lock()
	defer gc.statusLock.Unlock()

	if gc.stopped {
		return fmt.Errorf("already stopped")
	}

	close(gc.stopChan)

	if stopBasis {
		err := gc.cluster.Leave()
		if err != nil {
			return err
		}

		for gc.cluster.NodeStatus() != cluster.NodeLeft {
			time.Sleep(100 * time.Millisecond)
		}

		err = gc.cluster.Stop()
		if err != nil {
			return err
		}
	}

	err := gc.log.close()
	if err != nil {
		return err
	}

	gc.stopped = true

	return nil
}

func (gc *GatewayCluster) Stopped() bool {
	gc.statusLock.RLock()
	defer gc.statusLock.RUnlock()

	return gc.stopped
}

func (gc *GatewayCluster) dispatch() {
LOOP:
	for {
		select {
		case event := <-gc.eventStream:
			switch event := event.(type) {
			case *cluster.RequestEvent:
				if len(event.RequestPayload) == 0 {
					break
				}

				switch MessageType(event.RequestPayload[0]) {
				case queryGroupMaxSeqMessage:
					logger.Debugf("[member %s received queryGroupMaxSeqMessage message]",
						gc.clusterConf.NodeName)

					go gc.handleQueryGroupMaxSeq(event)
				case operationMessage:
					if gc.Mode() == WriteMode {
						logger.Debugf("[member %s received operationMessage message]",
							gc.clusterConf.NodeName)

						go gc.handleOperation(event)
					} else {
						logger.Errorf("[BUG: member with read mode received operationMessage]")
					}
				case operationRelayMessage:
					if gc.Mode() == ReadMode {
						logger.Debugf("[member %s received operationRelayMessage message]",
							gc.clusterConf.NodeName)

						go gc.handleOperationRelay(event)
					} else {
						logger.Errorf(
							"[BUG: member with write mode received operationRelayMessage]")
					}
				case retrieveMessage:
					if gc.Mode() == WriteMode {
						logger.Debugf("[member %s received retrieveMessage message]",
							gc.clusterConf.NodeName)

						go gc.handleRetrieve(event)
					} else {
						logger.Errorf("[BUG: member with read mode received retrieveMessage]")
					}
				case retrieveRelayMessage:
					if gc.Mode() == ReadMode {
						logger.Debugf("[member %s received retrieveRelayMessage message]",
							gc.clusterConf.NodeName)

						go gc.handleRetrieveRelay(event)
					} else {
						logger.Errorf(
							"[BUG: member with write mode received retrieveRelayMessage]")
					}
				case statMessage:
					logger.Debugf("[member %s received statMessage message]",
						gc.clusterConf.NodeName)

					go gc.handleStat(event)
				case statRelayMessage:
					logger.Debugf("[member %s received statRelayMessage message]",
						gc.clusterConf.NodeName)

					go gc.handleStatRelay(event)
				case opLogPullMessage:
					logger.Debugf("[member %s received opLogPullMessage message]",
						gc.clusterConf.NodeName)

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

func (gc *GatewayCluster) RestAliveMembersInSameGroup() (ret []cluster.Member) {
	totalMembers := gc.cluster.Members()

	groupName := gc.localGroupName()

	var members []string

	for _, member := range totalMembers {
		if member.NodeTags[groupTagKey] == groupName &&
			member.Status == cluster.MemberAlive &&
			member.NodeName != gc.clusterConf.NodeName {

			ret = append(ret, member)
		}

		members = append(members, fmt.Sprintf("%s (%s:%d) %s, %v",
			member.NodeName, member.Address, member.Port, member.Status.String(), member.NodeTags))
	}

	logger.Debugf("[total members in cluster (count=%d): %v]", len(members), strings.Join(members, ", "))

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
		logger.Errorf("[respond max sequence to request %s, node %s failed: %v]",
			req.RequestName, req.RequestNodeName, err)
	}

	logger.Debugf("[member %s responded queryGroupMaxSeqMessage message]", gc.clusterConf.NodeName)
}

// recordResp just records known response of member and ignore others.
// It does its best to record response, and just exits when GatewayCluster stopped
// or future got timeout, the caller could check membersRespBook to get the result.
func (gc *GatewayCluster) recordResp(requestName string, future *cluster.Future, membersRespBook map[string][]byte) {
	memberRespCount := 0
LOOP:
	for memberRespCount < len(membersRespBook) {
		select {
		case memberResp, ok := <-future.Response():
			if !ok {
				break LOOP
			}

			payload, known := membersRespBook[memberResp.ResponseNodeName]
			if !known {
				logger.Warnf("[received the response from an unexpexted node %s "+
					"started durning the request %s]", memberResp.ResponseNodeName,
					fmt.Sprintf("%s_relayed", requestName))
				continue LOOP
			}

			if payload != nil {
				logger.Errorf("[received multiple responses from node %s "+
					"for request %s, skipped. probably need to tune cluster configuration]",
					memberResp.ResponseNodeName, fmt.Sprintf("%s_relayed", requestName))
				continue LOOP
			}

			if memberResp.Payload == nil {
				logger.Errorf("[BUG: received empty response from node %s for request %s]",
					memberResp.ResponseNodeName, fmt.Sprintf("%s_relayed", requestName))
				memberResp.Payload = []byte("")
			}

			membersRespBook[memberResp.ResponseNodeName] = memberResp.Payload
			memberRespCount++
		case <-gc.stopChan:
			break LOOP
		}
	}

	return
}

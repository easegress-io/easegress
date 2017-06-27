package cluster

import (
	"io"
	"log"
	"math"
	"os"
	"time"

	"github.com/hashicorp/memberlist"
)

type Config struct {
	ProtocolVersion uint8

	NodeName string
	NodeTags map[string]string

	BindAddress, AdvertiseAddress string
	BindPort, AdvertisePort       uint16

	GossipInterval, GossipToTheDeadTime time.Duration

	ProbeTimeout time.Duration

	PushPullInterval, ProbeInterval time.Duration

	TCPTimeout time.Duration

	EnableCompression bool

	ProbeIntervalLimit uint

	GossipNodes, IndirectCheckNodes uint

	UDPBufferSize int

	MessageSendTimeout, FailedMemberReconnectTimeout, MemberLeftRecordTimeout time.Duration
	RecentMemberOperationTimeout                                              time.Duration
	FailedMemberReconnectInterval, MemberCleanupInterval                      time.Duration
	RecentRequestBookSize                                                     uint

	// Gossip message retransmits = GossipRetransmitMult * log(N+1)
	// Gateway cluster Message retransmits equals to MessageRetransmitMult * log(N+1)
	// Request timeout equals to GossipInterval * RequestTimeoutMult * log(N+1)
	GossipRetransmitMult, MessageRetransmitMult, RequestTimeoutMult uint

	// Member suspicion timeout equals to MemberSuspicionMult * log(N+1) * ProbeInterval
	// Max member suspicion timeout equals to member suspicion timeout * SuspicionMaxTimeoutMult
	MemberSuspicionMult, MemberSuspicionMaxTimeoutMult uint

	LogOutput io.Writer
	Logger    *log.Logger

	EventStream chan<- Event
}

func createMemberListConfig(conf *Config, eventDelegate memberlist.EventDelegate,
	conflictDelegate memberlist.ConflictDelegate, messageDelegate memberlist.Delegate) *memberlist.Config {

	if conf == nil {
		return nil
	}

	probeInterval := time.Duration(1)
	if conf.ProbeInterval > 0 {
		probeInterval = conf.ProbeInterval
	}

	gossipNodes := 1
	if conf.GossipNodes > 0 {
		gossipNodes = int(conf.GossipNodes)
	}

	indirectCheckNodes := 1
	if conf.IndirectCheckNodes > 0 {
		indirectCheckNodes = int(conf.IndirectCheckNodes)
	}

	gossipRetransmitMult := 1
	if conf.GossipRetransmitMult > 0 {
		gossipRetransmitMult = int(conf.GossipRetransmitMult)
	}

	memberSuspicionMult := 1
	if conf.MemberSuspicionMult > 0 {
		memberSuspicionMult = int(conf.MemberSuspicionMult)
	}

	memberSuspicionMaxTimeoutMult := 1
	if conf.MemberSuspicionMaxTimeoutMult > 0 {
		memberSuspicionMaxTimeoutMult = int(conf.MemberSuspicionMaxTimeoutMult)
	}

	udpBufferSize := conf.UDPBufferSize
	if udpBufferSize <= 0 {
		udpBufferSize = 1400
	}

	ret := &memberlist.Config{
		ProtocolVersion:         memberlist.ProtocolVersion2Compatible,
		DelegateProtocolMin:     ProtocolVersionMin,
		DelegateProtocolMax:     ProtocolVersionMax,
		DelegateProtocolVersion: conf.ProtocolVersion,
		Events:                  eventDelegate,
		Conflict:                conflictDelegate,
		Delegate:                messageDelegate,
		// TODO: add TCP pull/push merge and node "alive" message hooks to notify upper layer when cares
		Merge:                   nil,
		Alive:                   nil,
		Name:                    conf.NodeName,
		BindAddr:                conf.BindAddress,
		BindPort:                int(conf.BindPort),
		AdvertiseAddr:           conf.AdvertiseAddress,
		AdvertisePort:           int(conf.AdvertisePort),
		ProbeTimeout:            conf.ProbeTimeout,
		PushPullInterval:        conf.PushPullInterval,
		ProbeInterval:           probeInterval,
		GossipNodes:             gossipNodes,
		GossipInterval:          conf.GossipInterval,
		GossipToTheDeadTime:     conf.GossipToTheDeadTime,
		TCPTimeout:              conf.TCPTimeout,
		EnableCompression:       conf.EnableCompression,
		AwarenessMaxMultiplier:  int(math.Ceil(float64(conf.ProbeIntervalLimit) / float64(probeInterval))),
		IndirectChecks:          indirectCheckNodes,
		DisableTcpPings:         false,
		DNSConfigPath:           "/etc/resolv.conf",
		HandoffQueueDepth:       1024,
		UDPBufferSize:           udpBufferSize,
		RetransmitMult:          gossipRetransmitMult,
		SuspicionMult:           memberSuspicionMult,
		SuspicionMaxTimeoutMult: memberSuspicionMaxTimeoutMult,

		LogOutput: conf.LogOutput,
		Logger:    conf.Logger,
	}

	return ret
}

func DefaultLANConfig() *Config {
	hostname, _ := os.Hostname()

	ret := &Config{
		ProtocolVersion:               1,
		NodeName:                      hostname,
		NodeTags:                      make(map[string]string),
		BindAddress:                   "0.0.0.0",
		BindPort:                      9099,
		AdvertisePort:                 9099,
		ProbeTimeout:                  500 * time.Millisecond,
		PushPullInterval:              30 * time.Second,
		ProbeInterval:                 1 * time.Second,
		GossipNodes:                   3,
		GossipInterval:                200 * time.Millisecond,
		GossipToTheDeadTime:           30 * time.Second,
		TCPTimeout:                    10 * time.Second,
		EnableCompression:             true,
		ProbeIntervalLimit:            5,
		IndirectCheckNodes:            3,
		MessageSendTimeout:            5 * time.Second,
		FailedMemberReconnectTimeout:  24 * time.Hour,
		MemberLeftRecordTimeout:       24 * time.Hour,
		RecentMemberOperationTimeout:  5 * time.Minute,
		RecentRequestBookSize:         500,
		FailedMemberReconnectInterval: 30 * time.Second,
		MemberCleanupInterval:         15 * time.Second,
		GossipRetransmitMult:          4,
		MessageRetransmitMult:         4,
		RequestTimeoutMult:            15,
		MemberSuspicionMult:           5,
		MemberSuspicionMaxTimeoutMult: 6,
		UDPBufferSize:                 4000,
	}

	return ret
}

func DefaultWANConfig() *Config {
	ret := DefaultLANConfig()

	ret.TCPTimeout = 30 * time.Second
	ret.ProbeTimeout = 3 * time.Second
	ret.PushPullInterval = 60 * time.Second
	ret.ProbeInterval = 5 * time.Second
	ret.MemberSuspicionMult = 6
	ret.GossipNodes = 4
	ret.GossipInterval = 1 * time.Second
	ret.GossipToTheDeadTime = 60 * time.Second
	ret.UDPBufferSize = 1400

	return ret
}

func DefaultLocalConfig() *Config {
	ret := DefaultLANConfig()

	ret.TCPTimeout = time.Second
	ret.IndirectCheckNodes = 1
	ret.GossipRetransmitMult = 2
	ret.MessageRetransmitMult = 2
	ret.MemberSuspicionMult = 3
	ret.PushPullInterval = 15 * time.Second
	ret.ProbeTimeout = 200 * time.Millisecond
	ret.ProbeInterval = time.Second
	ret.GossipInterval = 100 * time.Millisecond
	ret.GossipToTheDeadTime = 15 * time.Second
	ret.UDPBufferSize = 8000

	return ret
}

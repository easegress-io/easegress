package cluster

import (
	"time"

	"github.com/hashicorp/memberlist"
)

type Config struct {
	ProtocolVersion uint8

	NodeName string
	NodeTags map[string]string

	MessageSendTimeout, FailedMemberReconnectTimeout, MemberLeftRecordTimeout, RecentOperationTimeout time.Duration
	FailedMemberReconnectInterval, MemberLeftRecordCleanupInterval                                    time.Duration

	// Message retransmits equals to MessageRetransmitMult * log(N+1)
	// Request timeout equals to GossipInterval * RequestTimeoutMult * log(N+1)
	MessageRetransmitMult, RequestTimeoutMult int

	EventStream chan<- Event

	RequestSizeLimit, ResponseSizeLimit int

	GossipInterval time.Duration
}

func createMemberListConfig(conf *Config, eventDelegate memberlist.EventDelegate,
	conflictDelegate memberlist.ConflictDelegate, messageDelegate memberlist.Delegate) *memberlist.Config {

	if conf == nil {
		return nil
	}

	ret := &memberlist.Config{
		Name:                    conf.NodeName,
		ProtocolVersion:         memberlist.ProtocolVersion2Compatible,
		DelegateProtocolMin:     ProtocolVersionMin,
		DelegateProtocolMax:     ProtocolVersionMax,
		DelegateProtocolVersion: conf.ProtocolVersion,
		Events:                  eventDelegate,
		Conflict:                conflictDelegate,
		Delegate:                messageDelegate,
		// TODO: add TCP pull/push merge and node "alive" message hooks to notify upper layer when cares
		Merge: nil,
		Alive: nil,
	}

	return ret
}

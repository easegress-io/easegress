package cluster

import (
	"net"
	"regexp"
	"time"

	"github.com/hashicorp/memberlist"

	"logger"
)

type messageType uint8

const (
	memberJoinMessage messageType = iota
	memberLeaveMessage
	requestMessage
	responseMessage
	messageRelayMessage
	statePushPullMessage
	memberConflictResolvingRequestMessage
	memberConflictResolvingResponseMessage
)

////

type messageMemberJoin struct {
	joinTime logicalTime
	nodeName string
}

////

type messageMemberLeave struct {
	leaveTime logicalTime
	nodeName  string
}

////

type messagePushPull struct {
	memberClockTime, requestClockTime logicalTime
	memberLastMessageTimes            map[string]logicalTime
	leftMemberNames                   []string
}

////

type requestFilterType uint8

const (
	nodeNameFilter requestFilterType = iota
	nodeTagsFilter
)

type requestFlagType uint32

const (
	ackRequestFlag requestFlagType = 1 << iota
	notifyRequestFlag
	unicastRequestFlag
)

type requestNodeTagFilter struct {
	name, valueRegex string
}

type messageRequest struct {
	requestId   uint64
	requestName string
	requestTime logicalTime

	requestNodeName string
	// used to respond directly
	requestNodeAddress net.IP
	requestNodePort    uint16

	// options
	requestFilters     [][]byte
	requestFlags       uint32
	responseRelayCount uint
	requestTimeout     time.Duration

	// request payload of upper layer
	requestPayload []byte
}

func (mr *messageRequest) applyFilters(param *RequestParam) error {
	if param == nil {
		return nil
	}

	var filters [][]byte

	if len(param.TargetNodeNames) > 0 {
		buff, err := PackWithHeader(param.TargetNodeNames, uint8(nodeNameFilter))
		if err != nil {
			return err
		}

		filters = append(filters, buff)
	}

	for name, valueRegex := range param.TargetNodeTags {
		filter := requestNodeTagFilter{
			name, valueRegex,
		}

		buff, err := PackWithHeader(&filter, uint8(nodeTagsFilter))
		if err != nil {
			return err
		}

		filters = append(filters, buff)
	}

	mr.requestFilters = filters

	return nil
}

func (mr *messageRequest) flag(flag requestFlagType) bool {
	return (mr.requestFlags & uint32(flag)) != 0
}

func (mr *messageRequest) filter(conf *Config) bool {
	if conf == nil {
		return false
	}

	for _, filter := range mr.requestFilters {
		filterType := requestFilterType(filter[0])

		switch filterType {
		case nodeNameFilter:
			var nodeNames []string

			err := Unpack(filter[1:], nodeNames)
			if err != nil {
				logger.Errorf("[unpack node name filter of request message failed: %v]", err)
				return false
			}

			for _, nodeName := range nodeNames {
				if conf.NodeName == nodeName {
					return true
				}
			}

			return false
		case nodeTagsFilter:
			var tags map[string]string

			err := Unpack(filter[1:], tags)
			if err != nil {
				logger.Errorf("[unpack tag filter of request message failed: %v]", err)
				return false
			}

			for tagName, tagValueRegex := range tags {
				if len(tagValueRegex) == 0 {
					tagValueRegex = `.*`
				}

				matched, _ := regexp.MatchString(tagValueRegex, conf.NodeTags[tagName])
				if matched {
					return true
				}
			}

			return false
		default:
			logger.Errorf("[BUG: invalid request filter type %v, ignored]", filterType)
			return false
		}
	}

	return false
}

////

type responseFlagType uint32

const (
	ackResponseFlag responseFlagType = 1 << iota
)

////

type messageResponse struct {
	requestId   uint64
	requestName string
	requestTime logicalTime

	// options
	responseFlags       uint32
	responseNodeName    string
	responseNodeAddress net.IP
	responseNodePort    uint16

	// response payload of upper layer
	responsePayload []byte
}

func (mr *messageResponse) flag(flag responseFlagType) bool {
	return (mr.responseFlags & uint32(flag)) != 0
}

////

type messageRelay struct {
	sourceNodeName string

	targetNodeAddress net.IP
	targetNodePort    uint16

	relayPayload []byte
}

////

type messageMemberConflictResolvingRequest struct {
	conflictNodeName string
}

////

type messageMemberConflictResolvingResponse struct {
	member *Member
}

////

type messageFanout struct {
	message []byte
	notify  chan<- struct{}
}

func (mf *messageFanout) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (mf *messageFanout) Message() []byte {
	return mf.message
}

func (mf *messageFanout) Finished() {
	if mf.notify != nil {
		close(mf.notify)
	}
}

func fanoutBuffer(q *memberlist.TransmitLimitedQueue, buff []byte, sentNotify chan<- struct{}) {
	q.QueueBroadcast(&messageFanout{
		message: buff,
		notify:  sentNotify,
	})
}

func fanoutMessage(q *memberlist.TransmitLimitedQueue, msg interface{},
	msgType messageType, sentNotify chan<- struct{}) error {

	buff, err := PackWithHeader(msg, uint8(msgType))
	if err != nil {
		return err
	}

	fanoutBuffer(q, buff, sentNotify)

	return nil
}

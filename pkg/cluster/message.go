package cluster

import (
	"net"
	"regexp"
	"time"

	"github.com/hexdecteam/easegateway/pkg/logger"

	"github.com/hashicorp/memberlist"
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
	JoinTime logicalTime
	NodeName string
}

////

type messageMemberLeave struct {
	LeaveTime logicalTime
	NodeName  string
}

////

type messagePushPull struct {
	MemberClockTime, RequestClockTime logicalTime
	MemberLastMessageTimes            map[string]logicalTime
	LeftMemberNames                   []string
}

////

type requestFilterType uint8

const (
	nodeNameFilter requestFilterType = iota
	nodeTagFilter
)

type requestFlagType uint32

const (
	ackRequestFlag requestFlagType = 1 << iota
	notifyRequestFlag
	unicastRequestFlag
)

type requestNodeNameFilter struct {
	Name string
}

type requestNodeTagFilter struct {
	Name, ValueRegex string
}

type messageRequest struct {
	RequestId   uint64
	RequestName string
	RequestTime logicalTime

	RequestNodeName string
	// used to respond directly
	RequestNodeAddress net.IP
	RequestNodePort    uint16

	// options
	RequestFilters     [][]byte
	RequestFlags       uint32
	ResponseRelayCount uint
	RequestTimeout     time.Duration

	// request payload of upper layer
	RequestPayload []byte
}

func (mr *messageRequest) applyFilters(param *RequestParam) error {
	if param == nil {
		return nil
	}

	var filters [][]byte

	for _, name := range param.TargetNodeNames {
		filter := requestNodeNameFilter{
			Name: name,
		}

		buff, err := PackWithHeader(&filter, uint8(nodeNameFilter))
		if err != nil {
			return err
		}

		filters = append(filters, buff)
	}

	for name, valueRegex := range param.TargetNodeTags {
		filter := requestNodeTagFilter{
			Name:       name,
			ValueRegex: valueRegex,
		}

		buff, err := PackWithHeader(&filter, uint8(nodeTagFilter))
		if err != nil {
			return err
		}

		filters = append(filters, buff)
	}

	mr.RequestFilters = filters

	return nil
}

func (mr *messageRequest) flag(flag requestFlagType) bool {
	return (mr.RequestFlags & uint32(flag)) != 0
}

func (mr *messageRequest) filter(conf *Config) bool {
	if conf == nil {
		return false
	}

	nameFilter := false
	nameMatched := false

	for _, filter := range mr.RequestFilters {
		if requestFilterType(filter[0]) == nodeNameFilter {
			nameFilter = true

			nameFilter := new(requestNodeNameFilter)

			err := Unpack(filter[1:], nameFilter)
			if err != nil {
				logger.Errorf("[unpack node name filter of request message failed: %v]", err)
				continue
			}

			if conf.NodeName == nameFilter.Name {
				nameMatched = true
				break
			}
		}
	}

	if nameFilter && !nameMatched {
		return false
	}

	tagMatched := true

	for _, filter := range mr.RequestFilters {
		if requestFilterType(filter[0]) == nodeTagFilter {
			tagFilter := new(requestNodeTagFilter)

			err := Unpack(filter[1:], tagFilter)
			if err != nil {
				logger.Errorf("[unpack tag filter of request message failed: %v]", err)
				continue
			}

			if len(tagFilter.ValueRegex) == 0 {
				tagFilter.ValueRegex = `.*`
			}

			ok, _ := regexp.MatchString(tagFilter.ValueRegex, conf.NodeTags[tagFilter.Name])
			if !ok {
				tagMatched = false
				break
			}
		}
	}

	if !nameFilter {
		nameMatched = true
	}

	return nameMatched && tagMatched
}

////

type responseFlagType uint32

const (
	ackResponseFlag responseFlagType = 1 << iota
)

////

type messageResponse struct {
	RequestId   uint64
	RequestName string
	RequestTime logicalTime

	// options
	ResponseFlags       uint32
	ResponseNodeName    string
	ResponseNodeAddress net.IP
	ResponseNodePort    uint16

	// response payload of upper layer
	ResponsePayload []byte
}

func (mr *messageResponse) flag(flag responseFlagType) bool {
	return (mr.ResponseFlags & uint32(flag)) != 0
}

////

type messageRelay struct {
	SourceNodeName string

	TargetNodeAddress net.IP
	TargetNodePort    uint16

	RelayPayload []byte
}

////

type messageMemberConflictResolvingRequest struct {
	ConflictNodeName string
}

////

type messageMemberConflictResolvingResponse struct {
	Member *Member
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

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
	memberConflictResponseMessage
)

////

type messageMemberJoin struct {
	time     logicalTime
	nodeName string
}

////

type messageMemberLeave struct {
	time     logicalTime
	nodeName string
}

////

type messagePushPull struct {
	memberClockTime, requestClockTime logicalTime
	memberLastMessageTimes            map[string]logicalTime
	leftMembers                       []string
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
	id   uint64
	name string
	time logicalTime

	nodeName string
	// used to respond directly
	nodeAddress net.IP
	nodePort    uint16

	// options
	filters    [][]byte
	flags      uint32
	relayCount uint
	timeout    time.Duration

	// payload of upper layer
	payload []byte
}

func (mr *messageRequest) applyFilters(param *RequestParam) error {
	if param == nil {
		return nil
	}

	var filters [][]byte

	if len(param.NodeNames) > 0 {
		buff, err := pack(param.NodeNames, uint8(nodeNameFilter))
		if err != nil {
			return err
		}

		filters = append(filters, buff)
	}

	for name, valueRegex := range param.NodeTags {
		filter := requestNodeTagFilter{
			name, valueRegex,
		}

		buff, err := pack(&filter, uint8(nodeTagsFilter))
		if err != nil {
			return err
		}

		filters = append(filters, buff)
	}

	mr.filters = filters

	return nil
}

func (mr *messageRequest) flag(flag requestFlagType) bool {
	return (mr.flags & uint32(flag)) != 0
}

func (mr *messageRequest) filter(conf *Config) bool {
	if conf == nil {
		return false
	}

	for _, filter := range mr.filters {
		filterType := requestFilterType(filter[0])

		switch filterType {
		case nodeNameFilter:
			var nodeNames []string

			err := unpack(filter[1:], nodeNames)
			if err != nil {
				logger.Errorf("[failed to unpack node name filter of request message: %s]", err)
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

			err := unpack(filter[1:], tags)
			if err != nil {
				logger.Errorf("[failed to unpack tag filter of request message: %s]", err)
				return false
			}

			for tagName, tagValueRegex := range tags {
				matched, _ := regexp.MatchString(tagValueRegex, conf.NodeTags[tagName])
				if matched {
					return true
				}
			}

			return false
		default:
			logger.Errorf("[BUG: invalid request filter type, ignored: %v]", filterType)
			return false
		}
	}

	return false
}

////

type messageResponse struct {
	requestId   uint64
	name        string
	time        logicalTime
	flags       uint32
	nodeName    string
	nodeAddress net.IP
	nodePort    uint16
	payload     []byte // response payload for upper layer
}

func (mr *messageResponse) flag(flag requestFlagType) bool {
	return (mr.flags & uint32(flag)) != 0
}

func (mr *messageResponse) send() bool {
	// TODO
	return false
}

////

type messageRelay struct {
	sourceNodeName string
	time           logicalTime

	nodeAddress net.IP
	nodePort    uint16

	payload []byte
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

	buff, err := pack(msg, uint8(msgType))
	if err != nil {
		return err
	}

	fanoutBuffer(q, buff, sentNotify)

	return nil
}

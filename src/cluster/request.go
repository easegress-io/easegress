package cluster

import (
	"fmt"
	"math"
	"sync"
	"time"

	"logger"
	"common"
)

type RequestParam struct {
	TargetNodeNames    []string
	TargetNodeTags     map[string]string // support regular expression
	RequireAck         bool
	ResponseRelayCount uint
	Timeout            time.Duration
}

func defaultRequestParam(aliveMemberCount int, requestTimeoutMult uint, gossipInterval time.Duration) *RequestParam {
	return &RequestParam{
		TargetNodeNames: nil, // no filter
		TargetNodeTags:  nil, // no filter
		RequireAck:      true,
		Timeout:         defaultRequestTimeout(aliveMemberCount, requestTimeoutMult, gossipInterval),
	}
}

func defaultRequestTimeout(aliveMemberCount int, requestTimeoutMult uint, gossipInterval time.Duration) time.Duration {
	return time.Duration(
		float64(gossipInterval) * float64(requestTimeoutMult) *
			math.Ceil(math.Log10(float64(aliveMemberCount+1))))
}

////

type MemberResponse struct {
	ResponseNodeName string
	Payload          []byte
}

////

type Future struct {
	sync.Mutex

	requestId       uint64
	requestTime     logicalTime
	requestDeadline time.Time
	closed          bool

	ackBook, responseBook map[string]struct{}

	ackStream      chan string
	responseStream chan *MemberResponse
}

func createFuture(requestId uint64, requestTime logicalTime, memberCount int,
	param *RequestParam, cleanup func()) *Future {

	future := &Future{
		requestId:       requestId,
		requestTime:     requestTime,
		requestDeadline: common.Now().Add(param.Timeout),
		responseBook:    make(map[string]struct{}),
		responseStream:  make(chan *MemberResponse, memberCount*2),
		ackStream:       make(chan string, memberCount*2),
	}

	if param.RequireAck {
		future.ackBook = make(map[string]struct{})
	}

	// close and cleanup response automatically when request timeout
	time.AfterFunc(param.Timeout, func() {
		future.Lock()
		if future.closed {
			return
		}

		future.closed = true
		future.Unlock()

		cleanup()

		future.Lock()
		defer future.Unlock()

		if future.responseStream != nil {
			close(future.responseStream)
		}

		if future.ackStream != nil {
			close(future.ackStream)
		}
	})

	return future
}

func (f *Future) Ack() <-chan string {
	return f.ackStream
}

func (f *Future) Response() <-chan *MemberResponse {
	return f.responseStream
}

func (f *Future) Closed() bool {
	f.Lock()
	defer f.Unlock()
	return f.closed

}

func (f *Future) ack(nodeName string) (bool, error) {
	f.Lock()
	defer f.Unlock()

	if f.closed {
		return false, nil
	}

	_, triggered := f.ackBook[nodeName]
	if triggered {
		// ack is handled, ignore it
		return false, nil
	}

	select {
	case f.ackStream <- nodeName:
		f.ackBook[nodeName] = struct{}{}
	default:
		return false, fmt.Errorf("write response ack to channel failed")
	}

	return true, nil
}

func (f *Future) response(response *MemberResponse) (bool, error) {
	f.Lock()
	defer f.Unlock()

	if f.closed {
		return false, nil
	}

	_, triggered := f.responseBook[response.ResponseNodeName]
	if triggered {
		// response is handled, ignore it
		return false, nil
	}

	select {
	case f.responseStream <- response:
		f.responseBook[response.ResponseNodeName] = struct{}{}
	default:
		return false, fmt.Errorf("write response to channel failed")
	}

	return true, nil
}

////

type requestOperations struct {
	requestTime logicalTime
	requestIds  []uint64
}

type requestOperationBook struct {
	size       uint
	operations []*requestOperations
}

func createRequestOperationBook(size uint) *requestOperationBook {
	return &requestOperationBook{
		size:       size,
		operations: make([]*requestOperations, size),
	}
}

func (rob *requestOperationBook) save(requestId uint64, msgTime, requestClock logicalTime) bool {
	if requestClock > logicalTime(rob.size) && msgTime < requestClock-logicalTime(rob.size) {
		// message is too old, ignore it
		return false
	}

	idx := msgTime % logicalTime(rob.size)

	operations := rob.operations[idx]
	if operations != nil && operations.requestTime == msgTime {
		for _, id := range operations.requestIds {
			if id == requestId {
				// request is handled, ignored it
				return false
			}
		}
	} else {
		operations = &requestOperations{
			requestTime: msgTime,
		}
		rob.operations[idx] = operations
	}

	operations.requestIds = append(operations.requestIds, requestId)

	return true
}

////

type internalRequestType string

const (
	memberConflictResolvingInternalRequest internalRequestType = "memberConflictResolvingInternalRequest"
	memberPingInternalRequest              internalRequestType = "memberPingInternalRequest"
)

func (t internalRequestType) String() string {
	return string(t)
}

////

type internalRequestHandler struct {
	c *Cluster

	in  chan Event
	out chan<- Event
}

func newInternalRequestHandler(c *Cluster, eventStream chan<- Event) chan<- Event {
	in := make(chan Event, 1024)

	h := &internalRequestHandler{
		c:   c,
		in:  in,
		out: eventStream,
	}

	go h.handleInternalRequest()

	return in
}

func (h *internalRequestHandler) handleInternalRequest() {
	for {
		select {
		case event := <-h.in:
			if event.Type() != RequestReceivedEvent {
				if h.out != nil {
					h.out <- event
				}
				continue
			}

			request, _ := event.(*RequestEvent)

			switch request.RequestName {
			case memberConflictResolvingInternalRequest.String():
				h.handleMemberConflict(request)
			case memberPingInternalRequest.String():
				// eat tne event and nothing to do
				// ack response is enough to show member is reach-able
			default:
				if h.out != nil {
					h.out <- event
				}
			}
		case <-h.c.Stopped():
			return
		}
	}
}

func (h *internalRequestHandler) handleMemberConflict(request *RequestEvent) {
	if len(request.RequestPayload) == 0 {
		logger.Errorf("[BUG: received illegal member conflict resolving request message, ignored]")
		return
	}

	msgType := messageType(request.RequestPayload[0])
	if msgType != memberConflictResolvingRequestMessage {
		logger.Errorf("[BUG: received illegal member conflict resolving request message, ignored: %d]", msgType)
		return
	}

	requestMsg := messageMemberConflictResolvingRequest{}

	err := Unpack(request.RequestPayload[1:], &requestMsg)
	if err != nil {
		logger.Errorf("[unpack member conflict resolving request message failed: %v]", err)
		return
	}

	if requestMsg.ConflictNodeName == h.c.conf.NodeName {
		return
	}

	logger.Debugf("[received conflict resolution request for member %s]", requestMsg.ConflictNodeName)

	responseMsg := messageMemberConflictResolvingResponse{}

	h.c.membersLock.Lock()
	ms, ok := h.c.members[requestMsg.ConflictNodeName]
	if ok {
		responseMsg.Member = &ms.Member
	}
	h.c.membersLock.Unlock()

	buff, err := PackWithHeader(&responseMsg, uint8(memberConflictResolvingResponseMessage))
	if err != nil {
		logger.Errorf("[pack member conflict resolving response message failed: %v]", err)
		return
	}

	err = request.Respond(buff)
	if err != nil {
		logger.Errorf("[respond member conflict resolving response message failed: %v]", err)
	}
}

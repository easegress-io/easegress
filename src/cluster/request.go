package cluster

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type RequestParam struct {
	NodeNames  []string
	NodeTags   map[string]string // support regular expression
	Ack        bool
	RelayCount uint
	Timeout    time.Duration
}

func defaultRequestParam(aliveMemberCount, requestTimeoutMult int, gossipInterval time.Duration) *RequestParam {
	return &RequestParam{
		NodeNames: nil, // no filter
		NodeTags:  nil, // no filter
		Ack:       true,
		Timeout:   defaultRequestTimeout(aliveMemberCount, requestTimeoutMult, gossipInterval),
	}
}

func defaultRequestTimeout(aliveMemberCount, requestTimeoutMult int, gossipInterval time.Duration) time.Duration {
	return time.Duration(float64(gossipInterval) * float64(requestTimeoutMult) * math.Ceil(math.Log10(float64(aliveMemberCount+1))))
}

////

type MemberResponse struct {
	NodeName string
	Payload  []byte
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
		requestDeadline: time.Now().Add(param.Timeout),
		responseBook:    make(map[string]struct{}),
		responseStream:  make(chan *MemberResponse, memberCount*2),
		ackStream:       make(chan string, memberCount*2),
	}

	if param.Ack {
		future.ackBook = make(map[string]struct{})
	}

	// close and cleanup response automatically when request timeout
	time.AfterFunc(param.Timeout, func() {
		cleanup()

		future.Lock()
		defer future.Unlock()

		if future.closed {
			return
		}

		if future.responseStream != nil {
			close(future.responseStream)
		}

		if future.ackStream != nil {
			close(future.ackStream)
		}

		future.closed = true
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
		return false, nil
	}

	select {
	case f.ackStream <- nodeName:
		f.ackBook[nodeName] = struct{}{}
	default:
		return false, fmt.Errorf("write response ack event to channel failed")
	}

	return true, nil
}

func (f *Future) response(response *MemberResponse) (bool, error) {
	f.Lock()
	defer f.Unlock()

	if f.closed {
		return false, nil
	}

	_, triggered := f.responseBook[response.NodeName]
	if triggered {
		return false, nil
	}

	select {
	case f.responseStream <- response:
		f.responseBook[response.NodeName] = struct{}{}
	default:
		return false, fmt.Errorf("write response event to channel failed")
	}

	return true, nil
}

////

type requestOperation struct {
	messageTime logicalTime
	receiveTime time.Time // local wall clock time, for cleanup
}

type requestOperationBook struct {
	sync.Mutex
	operations map[uint64]*requestOperation
	timeout    time.Duration
}

func createRequestOperationBook(timeout time.Duration) *requestOperationBook {
	return &requestOperationBook{
		operations: make(map[uint64]*requestOperation),
		timeout:    timeout,
	}
}

func (rob *requestOperationBook) save(requestId uint64, msgTime logicalTime) bool {
	rob.Lock()
	defer rob.Unlock()

	request, known := rob.operations[requestId]
	if !known {
		request = &requestOperation{
			messageTime: msgTime,
			receiveTime: time.Now(),
		}
		rob.operations[requestId] = request

		return true
	}

	if request.messageTime > msgTime {
		// message is too old, ignore it
		return false
	}

	request.messageTime = msgTime
	request.receiveTime = time.Now()

	return true
}

func (rob *requestOperationBook) cleanup(now time.Time) {
	rob.Lock()
	defer rob.Unlock()

	for requestId, operation := range rob.operations {
		if now.Sub(operation.receiveTime) > rob.timeout {
			delete(rob.operations, requestId)
		}
	}
}

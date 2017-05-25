package cluster

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
)

type EventType int8

const (
	MemberJoinEvent EventType = iota
	MemberLeftEvent
	MemberFailedEvent
	MemberUpdateEvent
	MemberCleanupEvent
	RequestReceivedEvent
)

func (t EventType) String() string {
	switch t {
	case MemberJoinEvent:
		return "MemberJoin"
	case MemberLeftEvent:
		return "MemberLeft"
	case MemberFailedEvent:
		return "MemberFailed"
	case MemberUpdateEvent:
		return "MemberUpdate"
	case MemberCleanupEvent:
		return "MemberCleanup"
	case RequestReceivedEvent:
		return "RequestReceived"
	}

	return "Unknow"
}

////

type Event interface {
	Type() EventType
}

////

type MemberEvent struct {
	Typ    EventType
	Member member
}

func createMemberEvent(t EventType, member *member) *MemberEvent {
	return &MemberEvent{
		Typ:    t,
		Member: *member,
	}
}

func (e *MemberEvent) Type() EventType {
	return e.Typ
}

////

type RequestEvent struct {
	sync.Mutex
	RequestName     string
	RequestPayload  []byte
	RequestNodeName string

	c *cluster

	requestId            uint64
	requestTime          logicalTime
	requestFlags         uint32
	requestNodeAddress   net.IP
	requestNodePort      uint16
	responseRelayCount   uint
	acknowledged, closed bool
}

func createRequestEvent(c *cluster, msg *messageRequest) *RequestEvent {
	ret := &RequestEvent{
		RequestName:        msg.requestName,
		RequestPayload:     msg.requestPayload,
		RequestNodeName:    msg.requestNodeName,
		c:                  c,
		requestId:          msg.requestId,
		requestTime:        msg.requestTime,
		requestFlags:       msg.requestFlags,
		requestNodeAddress: msg.requestNodeAddress,
		requestNodePort:    msg.requestNodePort,
		responseRelayCount: msg.responseRelayCount,
	}

	time.AfterFunc(msg.requestTimeout, func() {
		ret.Lock()
		defer ret.Unlock()
		ret.closed = true
	})

	return ret
}

func (e *RequestEvent) Type() EventType {
	return RequestReceivedEvent
}

func (e *RequestEvent) flag(flag requestFlagType) bool {
	return (e.requestFlags & uint32(flag)) != 0
}

func (e *RequestEvent) Respond(payload []byte) error {
	e.Lock()
	defer e.Unlock()

	if e.closed {
		return fmt.Errorf("request is closed")
	}

	if e.flag(notifyRequestFlag) {
		return fmt.Errorf("notification can not respond")
	}

	responder := e.c.memberList.LocalNode()

	msg := messageResponse{
		requestId:           e.requestId,
		requestName:         e.RequestName,
		requestTime:         e.requestTime,
		responseNodeName:    responder.Name,
		responseNodeAddress: responder.Addr,
		responseNodePort:    responder.Port,
		responsePayload:     payload,
	}

	buff, err := pack(&msg, uint8(responseMessage))
	if err != nil {
		return fmt.Errorf("pack response message failed: %s", err)
	}

	if len(buff) > int(e.c.conf.ResponseSizeLimit) {
		return fmt.Errorf("response is too big (%d bytes)", len(buff))
	}

	var requester *memberlist.Node

	for _, member := range e.c.memberList.Members() {
		if member.Addr.Equal(e.requestNodeAddress) && member.Port == e.requestNodePort {
			requester = member
			break
		}
	}

	if requester == nil {
		return fmt.Errorf("request source node is not available")
	}

	err = e.c.memberList.SendBestEffort(requester, buff)
	if err != nil {
		return err
	}

	err = e.relay(&msg, requester)
	if err != nil {
		return err
	}

	e.closed = true

	return nil
}

func (e *RequestEvent) relay(msg *messageResponse, requester *memberlist.Node) error {
	if e.responseRelayCount == 0 {
		// nothing to do
		return nil
	}

	// TODO(zhiyan): to send relay message out
	// TODO(zhiyan): merge ack/response event for upper layer on duplicated messages

	return nil
}

func (e *RequestEvent) ack() error {
	e.Lock()
	defer e.Unlock()

	if e.closed {
		return fmt.Errorf("request is closed")
	}

	if e.acknowledged {
		return fmt.Errorf("request is acknowledged")
	}

	if !e.flag(ackRequestFlag) {
		return fmt.Errorf("request need not be acknowledged")
	}

	responder := e.c.memberList.LocalNode()

	msg := messageResponse{
		requestId:           e.requestId,
		requestName:         e.RequestName,
		requestTime:         e.requestTime,
		responseFlags:       uint32(ackResponseFlag),
		responseNodeName:    responder.Name,
		responseNodeAddress: responder.Addr,
		responseNodePort:    responder.Port,
	}

	buff, err := pack(&msg, uint8(responseMessage))
	if err != nil {
		return fmt.Errorf("pack response message failed: %s", err)
	}

	if len(buff) > int(e.c.conf.ResponseSizeLimit) {
		return fmt.Errorf("response is too big (%d bytes)", len(buff))
	}

	var requester *memberlist.Node

	for _, member := range e.c.memberList.Members() {
		if member.Addr.Equal(e.requestNodeAddress) && member.Port == e.requestNodePort {
			requester = member
			break
		}
	}

	if requester == nil {
		return fmt.Errorf("request source node is not available")
	}

	err = e.c.memberList.SendBestEffort(requester, buff)
	if err != nil {
		return err
	}

	err = e.relay(&msg, requester)
	if err != nil {
		return err
	}

	e.acknowledged = true

	return nil
}

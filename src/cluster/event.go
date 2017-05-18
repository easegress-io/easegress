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
	RequestEvent
)

func (t *EventType) String() string {
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
	case RequestEvent:
		return "Request"
	}

	return "Unknow"
}

////

type Event interface {
	Type() EventType
}

////

type MemberEvent struct {
	Type   EventType
	Member member
}

func createMemberEvent(t EventType, member *member) *MemberEvent {
	return &MemberEvent{
		Type:   t,
		Member: *member,
	}
}

func (e *MemberEvent) Type() EventType {
	return e.Type
}

////

type RequestEvent struct {
	sync.Mutex
	Name     string
	Payload  []byte
	NodeName string

	c *cluster

	id                   uint64
	time                 logicalTime
	flags                uint32
	nodeAddress          net.IP
	nodePort             uint16
	relayCount           uint
	acknowledged, closed bool
}

func createRequestEvent(c *cluster, msg *messageRequest) *RequestEvent {
	ret := &RequestEvent{
		Name:        msg.name,
		Payload:     msg.payload,
		NodeName:    msg.nodeName,
		c:           c,
		id:          msg.id,
		time:        msg.time,
		flags:       msg.flags,
		nodeAddress: msg.nodeAddress,
		nodePort:    msg.nodePort,
		relayCount:  msg.relayCount,
	}

	time.AfterFunc(msg.timeout, func() {
		ret.Lock()
		defer ret.Unlock()
		ret.closed = true
	})

	return ret
}

func (e *RequestEvent) EventType() EventType {
	return RequestEvent
}

func (e *RequestEvent) flag(flag requestFlagType) bool {
	return (e.flags & flag) != 0
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

	source := e.c.memberList.LocalNode()

	msg := messageResponse{
		id:       e.id,
		time:     e.time,
		flags:    e.flags,
		nodeName: source.Name,
		payload:  payload,
	}

	buff, err := pack(&msg, responseMessage)
	if err != nil {
		return fmt.Errorf("pack response message failed: %s", err)
	}

	if len(buff) > e.c.conf.ResponseSizeLimit {
		return fmt.Errorf("response is too big (%d bytes)", len(buff))
	}

	var requester *memberlist.Node

	for _, member := range e.c.memberList.Members() {
		if member.Addr.Equal(e.nodeAddress) && member.Port == e.nodePort {
			requester = member
			break
		}
	}

	if requester == nil {
		return fmt.Errorf("source node is not available")
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

func (e *RequestEvent) relay(msg *messageResponse, target *memberlist.Node) error {
	if e.relayCount == 0 {
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

	source := e.c.memberList.LocalNode()

	msg := messageResponse{
		id:       e.id,
		time:     e.time,
		flags:    ackRequestFlag,
		nodeName: source.Name,
	}

	buff, err := pack(&msg, responseMessage)
	if err != nil {
		return fmt.Errorf("pack response message failed: %s", err)
	}

	if len(buff) > e.c.conf.ResponseSizeLimit {
		return fmt.Errorf("response is too big (%d bytes)", len(buff))
	}

	var requester *memberlist.Node

	for _, member := range e.c.memberList.Members() {
		if member.Addr.Equal(e.nodeAddress) && member.Port == e.nodePort {
			requester = member
		}
	}

	if requester == nil {
		return fmt.Errorf("source node is not available")
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

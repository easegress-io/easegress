package cluster

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"

	"logger"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

////

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
	Member Member
}

func createMemberEvent(t EventType, member *Member) *MemberEvent {
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

	c *Cluster

	requestId            uint64
	requestTime          logicalTime
	requestFlags         uint32
	requestNodeAddress   net.IP
	requestNodePort      uint16
	responseRelayCount   uint
	acknowledged, closed bool
}

func createRequestEvent(c *Cluster, msg *messageRequest) *RequestEvent {
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

	buff, err := pack(&msg, uint8(responseMessage))
	if err != nil {
		return fmt.Errorf("pack response message failed: %s", err)
	}

	err = e.c.memberList.SendReliable(requester, buff)
	if err != nil {
		return fmt.Errorf("send response message failed: %s", err)
	}

	err = e.relay(responder, requester, buff)
	if err != nil {
		return err
	}

	e.closed = true

	return nil
}

func (e *RequestEvent) relay(responder, requester *memberlist.Node, responseMsgBuff []byte) error {
	if e.responseRelayCount == 0 {
		// nothing to do
		return nil
	}

	members := e.c.Members()
	if len(members) < int(e.responseRelayCount)+1 {
		// need not to do
		return nil
	}

	msg := messageRelay{
		sourceNodeName:    responder.Name,
		targetNodeAddress: requester.Addr,
		targetNodePort:    requester.Port,
		relayPayload:      responseMsgBuff,
	}

	buff, err := pack(&msg, uint8(messageRelayMessage))
	if err != nil {
		return fmt.Errorf("pack relay message failed: %s", err)
	}

	var relayMembers []*memberlist.Node

LOOP:
	for times := 0; len(relayMembers) < int(e.responseRelayCount) && times < len(members)*5; times++ {
		idx := rand.Intn(len(members))
		member := members[idx]

		if member.nodeName == responder.Name {
			// skip myself
			continue LOOP
		}

		if member.status != MemberAlive {
			// skip the node as non-alive member
			continue LOOP
		}

		for _, m := range relayMembers {
			if m.Name == member.nodeName {
				// skip selected member
				continue LOOP
			}
		}

		for _, node := range e.c.memberList.Members() {
			if node.Addr.Equal(member.address) && node.Port == member.port {
				relayMembers = append(relayMembers, node)
				continue LOOP
			}
		}
	}

	if len(relayMembers) != int(e.responseRelayCount) {
		logger.Warnf("[only %d member(s) can be selected to relay message, request requires %d relier]",
			len(relayMembers), e.responseRelayCount)
	}

	// Relay to a random set of peers.
	for _, m := range relayMembers {
		err = e.c.memberList.SendReliable(m, buff)
		if err != nil {
			return fmt.Errorf("send relay message failed: %s", err)
		}

	}

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

	buff, err := pack(&msg, uint8(responseMessage))
	if err != nil {
		return fmt.Errorf("pack response ack message failed: %s", err)
	}

	err = e.c.memberList.SendReliable(requester, buff)
	if err != nil {
		return fmt.Errorf("send response ack message failed: %s", err)
	}

	err = e.relay(responder, requester, buff)
	if err != nil {
		return err
	}

	e.acknowledged = true

	return nil
}

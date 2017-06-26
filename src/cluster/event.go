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
		RequestName:        msg.RequestName,
		RequestPayload:     msg.RequestPayload,
		RequestNodeName:    msg.RequestNodeName,
		c:                  c,
		requestId:          msg.RequestId,
		requestTime:        msg.RequestTime,
		requestFlags:       msg.RequestFlags,
		requestNodeAddress: msg.RequestNodeAddress,
		requestNodePort:    msg.RequestNodePort,
		responseRelayCount: msg.ResponseRelayCount,
	}

	time.AfterFunc(msg.RequestTimeout, func() {
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
		RequestId:           e.requestId,
		RequestName:         e.RequestName,
		RequestTime:         e.requestTime,
		ResponseNodeName:    responder.Name,
		ResponseNodeAddress: responder.Addr,
		ResponseNodePort:    responder.Port,
		ResponsePayload:     payload,
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

	buff, err := PackWithHeader(&msg, uint8(responseMessage))
	if err != nil {
		return fmt.Errorf("pack response message failed: %v", err)
	}

	err = e.c.memberList.SendReliable(requester, buff)
	if err != nil {
		return fmt.Errorf("send response message failed: %v", err)
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
		SourceNodeName:    responder.Name,
		TargetNodeAddress: requester.Addr,
		TargetNodePort:    requester.Port,
		RelayPayload:      responseMsgBuff,
	}

	buff, err := PackWithHeader(&msg, uint8(messageRelayMessage))
	if err != nil {
		return fmt.Errorf("pack relay message failed: %v", err)
	}

	var relayMembers []*memberlist.Node

LOOP:
	for times := 0; len(relayMembers) < int(e.responseRelayCount) && times < len(members)*5; times++ {
		idx := rand.Intn(len(members))
		member := members[idx]

		if member.NodeName == responder.Name {
			// skip myself
			continue LOOP
		}

		if member.Status != MemberAlive {
			// skip the node as non-alive member
			continue LOOP
		}

		for _, m := range relayMembers {
			if m.Name == member.NodeName {
				// skip selected member
				continue LOOP
			}
		}

		for _, node := range e.c.memberList.Members() {
			if node.Addr.Equal(member.Address) && node.Port == member.Port {
				relayMembers = append(relayMembers, node)
				continue LOOP
			}
		}
	}

	if len(relayMembers) != int(e.responseRelayCount) {
		logger.Warnf("[only %d member(s) can be selected to relay message but request requires %d relier," +
			"relay skipped]", len(relayMembers), e.responseRelayCount)
	}

	// Relay to a random set of peers.
	for _, m := range relayMembers {
		err = e.c.memberList.SendReliable(m, buff)
		if err != nil {
			return fmt.Errorf("send relay message failed: %v", err)
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
		RequestId:           e.requestId,
		RequestName:         e.RequestName,
		RequestTime:         e.requestTime,
		ResponseFlags:       uint32(ackResponseFlag),
		ResponseNodeName:    responder.Name,
		ResponseNodeAddress: responder.Addr,
		ResponseNodePort:    responder.Port,
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

	buff, err := PackWithHeader(&msg, uint8(responseMessage))
	if err != nil {
		return fmt.Errorf("pack response ack message failed: %v", err)
	}

	err = e.c.memberList.SendReliable(requester, buff)
	if err != nil {
		return fmt.Errorf("send response ack message failed: %v", err)
	}

	err = e.relay(responder, requester, buff)
	if err != nil {
		return err
	}

	e.acknowledged = true

	return nil
}

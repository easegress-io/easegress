package cluster

import (
	"github.com/hashicorp/memberlist"

	"common"
	"logger"
)

//
// Node notification hooks about members joining leaving and updating
//
type eventDelegate struct {
	c *cluster
}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	ed.c.joinNode(node)
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	ed.c.leaveNode(node)
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	ed.c.updateNode(node)
}

//
// Node notification hooks about members conflicting
//

type conflictDelegate struct {
	c *cluster
}

func (cd *conflictDelegate) NotifyConflict(knownNode, otherNode *memberlist.Node) {
	cd.c.resolveNodeConflict(knownNode, otherNode)
}

//
// Gossip messaging handling on gateway message
//

type messageDelegate struct {
	c *cluster
}

func (md *messageDelegate) NodeMeta(limit int) []byte {
	return md.c.nodeTags
}

func (md *messageDelegate) NotifyMsg(buff []byte) {
	if len(buff) == 0 {
		// defensive, nothing to do
		return
	}

	var messageQueue *memberlist.TransmitLimitedQueue
	forward := false

	msgType := messageType(buff[0])
	switch msgType {
	case memberJoinMessage:
		var msg messageMemberJoin
		err := unpack(buff[1:], &msg)
		if err != nil {
			logger.Errorf("[unpack member join message failed: %s]", err)
			break
		}

		logger.Debugf("[received member join memssage from node %s at logical clock %d]",
			msg.nodeName, msg.time)

		messageQueue = md.c.memberMessageSendQueue
		forward = md.c.operateNodeJoin(&msg)
	case memberLeaveMessage:
		var msg messageMemberLeave
		err := unpack(buff[1:], &msg)
		if err != nil {
			logger.Errorf("[unpack member leave message failed: %s]", err)
			break
		}

		logger.Debugf("[received member leave memssage from node %s at logical clock %d]",
			msg.nodeName, msg.time)

		messageQueue = md.c.memberMessageSendQueue
		forward = md.c.operateNodeLeave(&msg)
	case requestMessage:
		var msg messageRequest
		err := unpack(buff[1:], &msg)
		if err != nil {
			logger.Errorf("[unpack request message failed: %s]", err)
			break
		}

		logger.Debugf("[received request memssage from node %s at logical clock %d]",
			msg.nodeName, msg.time)

		messageQueue = md.c.requestMessageSendQueue
		forward = md.c.operateRequest(&msg)
	case responseMessage:
		var msg messageResponse
		err := unpack(buff[1:], &msg)
		if err != nil {
			logger.Errorf("[unpack response message failed: %s]", err)
			break
		}

		logger.Debugf("[received response memssage from node %s at logical clock %d]",
			msg.nodeName, msg.time)

		messageQueue = nil
		forward = md.c.operateResponse(&msg)
	case messageRelayMessage:
		var msg messageRelay
		err := unpack(buff[1:], &msg)
		if err != nil {
			logger.Errorf("[unpack relay message failed: %s]", err)
			break
		}

		logger.Debugf("[received relay memssage from node %s at logical clock %d]",
			msg.sourceNodeName, msg.time)

		messageQueue = nil
		forward = md.c.operateRelay(&msg)
	default:
		logger.Errorf("[cluster receives unknown message type, ignored: %d]", msgType)
	}

	if forward {
		dup := make([]byte, len(buff))
		copy(dup, buff)
		fanoutBuffer(messageQueue, dup, nil)
	}
}

func (md *messageDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	msgList := md.c.memberMessageSendQueue.GetBroadcasts(overhead, limit)

	size := overhead * len(msgList)
	for _, msg := range msgList {
		size += len(msg)
	}

	requestMessageList := md.c.requestMessageSendQueue.GetBroadcasts(overhead, limit-size)
	msgList = append(msgList, requestMessageList...)

	return msgList
}

func (d *messageDelegate) LocalState(join bool) []byte {
	d.c.membersLock.RLock()
	defer d.c.membersLock.RUnlock()

	msg := messagePushPull{
		memberClockTime:        d.c.memberClock.Time(),
		requestClockTime:       d.c.requestClock.Time(),
		memberLastMessageTimes: make(map[string]logicalTime, len(d.c.members)),
	}

	for name, ms := range d.c.members {
		msg.memberLastMessageTimes[name] = ms.lastMessageTime
	}

	msg.leftMembers = append(msg.leftMembers, d.c.leftMembers.names()...)

	buff, err := pack(&msg, uint8(statePushPullMessage))
	if err != nil {
		logger.Errorf("[pack state push/pull message failed, ignored: %s]", err)
		return nil
	}

	logger.Debugf("[prepared local state push/pull message]")

	return buff
}

func (d *messageDelegate) MergeRemoteState(buff []byte, isJoin bool) {
	if len(buff) == 0 {
		// defensive, nothing to do
		return
	}

	msgType := messageType(buff[0])

	if msgType != statePushPullMessage {
		logger.Errorf("[cluster receives illegal state push/pull message, ignored: %d]", msgType)
	}

	msg := messagePushPull{}

	err := unpack(buff[1:], &msg)
	if err != nil {
		logger.Errorf("[unpack state push/pull message failed: %s]", err)
		return
	}

	logger.Debugf("[received state push/pull memssage]")

	if msg.memberClockTime > 0 {
		d.c.memberClock.Update(msg.memberClockTime - 1)
	}

	if msg.requestClockTime > 0 {
		d.c.requestClock.Update(msg.requestClockTime - 1)
	}

	var leftMemberNames []string

	for _, name := range msg.leftMembers {
		leftMemberNames = append(leftMemberNames, name)

		d.c.operateNodeLeave(&messageMemberLeave{
			time:     msg.memberLastMessageTimes[name],
			nodeName: name,
		})
	}

	for name, lastMessageTime := range msg.memberLastMessageTimes {
		if !common.StrInSlice(name, leftMemberNames) {
			d.c.operateNodeJoin(&messageMemberJoin{
				time:     lastMessageTime,
				nodeName: name,
			})
		}
	}
}

package cluster

import (
	"fmt"
	"hash/fnv"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"

	"common"
	"logger"
)

const (
	ProtocolVersionMin uint8 = 1
	ProtocolVersionMax uint8 = 1
)

type cluster struct {
	conf     *Config
	nodeTags []byte // do not do local copy when supporting dynamic node tag update

	nodeJoinLock   sync.Mutex
	nodeStatusLock sync.RWMutex
	nodeStatus     NodeStatus

	memberList *memberlist.Memberlist

	membersLock                sync.RWMutex
	members                    map[string]*memberStatus
	leftMembers, failedMembers *memberStatusBook
	memberOperations           *memberOperationBook

	requestLock       sync.RWMutex
	futures           map[logicalTime]*Future
	requestOperations *requestOperationBook

	memberMessageSendQueue, requestMessageSendQueue *memberlist.TransmitLimitedQueue

	memberClock, requestClock logicalClock

	stop chan struct{}
}

func Create(conf *Config) (*cluster, error) {
	if conf == nil {
		return nil, fmt.Errorf("empty config")
	}

	if conf.ProtocolVersion < ProtocolVersionMin || conf.ProtocolVersion > ProtocolVersionMax {
		return nil, fmt.Errorf("invalid cluster protocol version %d", conf.ProtocolVersion)

	}

	c := &cluster{
		conf:              conf,
		nodeStatus:        NodeAlive,
		members:           make(map[string]*memberStatus),
		leftMembers:       createMemberStatusBook(conf.MemberLeftRecordTimeout),
		failedMembers:     createMemberStatusBook(conf.FailedMemberReconnectTimeout),
		memberOperations:  createMemberOperationBook(conf.RecentMemberOperationTimeout),
		requestOperations: createRequestOperationBook(conf.RecentRequestBookSize),
		stop:              make(chan struct{}),
	}

	var err error

	if conf.NodeTags == nil {
		conf.NodeTags = make(map[string]string)
	}

	c.nodeTags, err = packNodeTags(conf.NodeTags)
	if err != nil {
		return nil, err
	}

	if len(c.nodeTags) > memberlist.MetaMaxSize {
		return nil, fmt.Errorf("tags of the node is too much")
	}

	c.memberMessageSendQueue = &memberlist.TransmitLimitedQueue{
		NumNodes:       c.GetMemberCount,
		RetransmitMult: int(conf.MessageRetransmitMult),
	}
	c.requestMessageSendQueue = &memberlist.TransmitLimitedQueue{
		NumNodes:       c.GetMemberCount,
		RetransmitMult: int(conf.MessageRetransmitMult),
	}

	// logical clock starts from 1
	c.memberClock.Increase()
	c.requestClock.Increase()

	memberListConf := createMemberListConfig(conf,
		&eventDelegate{c: c}, &conflictDelegate{c: c}, &messageDelegate{c: c})

	c.memberList, err = memberlist.Create(memberListConf)
	if err != nil {
		return nil, fmt.Errorf("create memberlist failed: %s", err.Error())
	}

	go c.cleanupMember()
	go c.reconnectFailedMembers()

	return c, nil
}

func (c *cluster) Join(peerNodeNames []string) (int, error) {
	c.nodeJoinLock.Lock()
	defer c.nodeJoinLock.Unlock()

	if c.NodeStatus() != NodeAlive {
		return 0, fmt.Errorf("invalid node status")
	}

	contacted, err := c.memberList.Join(peerNodeNames)
	if contacted > 0 && err == nil {
		return contacted, c.broadcastMemberJoinMessage()
	}

	return 0, err
}

func (c *cluster) Leave() error {
	err := func() error {
		c.nodeStatusLock.Lock()
		defer c.nodeStatusLock.Unlock()

		switch c.nodeStatus {
		case NodeLeaving:
			return nil // in progress
		case NodeLeft:
			return nil // already left
		case NodeShutdown:
			return fmt.Errorf("invalid node status")
		}

		c.nodeStatus = NodeLeaving

		return nil
	}()
	if err != nil {
		return err
	}

	err = c.broadcastMemberLeaveMessage(c.conf.NodeName)
	if err != nil {
		return err
	}

	err = c.memberList.Leave(c.conf.MessageSendTimeout)
	if err != nil {
		return err
	}

	c.nodeStatusLock.Lock()
	defer c.nodeStatusLock.Unlock()

	// prevent concurrent calls on cluster interface
	if c.nodeStatus != NodeShutdown {
		c.nodeStatus = NodeLeft
	}

	return nil
}

func (c *cluster) ForceLeave(nodeName string) error {
	if nodeName == c.conf.NodeName {
		// should go normal Leave() case
		return fmt.Errorf("invalid node")
	}

	return c.broadcastMemberLeaveMessage(nodeName)
}

func (c *cluster) Stop() error {
	c.nodeStatusLock.Lock()
	defer c.nodeStatusLock.Unlock()

	switch c.nodeStatus {
	case NodeShutdown:
		return nil // already stop
	case NodeAlive:
		fallthrough
	case NodeLeaving:
		return fmt.Errorf("invalid node status")
	}

	err := c.memberList.Shutdown()
	if err != nil {
		return err
	}

	c.nodeStatus = NodeShutdown

	close(c.stop)

	return nil
}

func (c *cluster) Request(name string, payload []byte, param *RequestParam) (*Future, error) {
	if param == nil {
		param = defaultRequestParam(
			c.memberList.NumMembers(), c.conf.RequestTimeoutMult, c.conf.GossipInterval)
	} else if param.Timeout < 1 {
		param.Timeout = defaultRequestTimeout(
			c.memberList.NumMembers(), c.conf.RequestTimeoutMult, c.conf.GossipInterval)
	}

	uuid, err := common.UUID()
	if err != nil {
		return nil, err
	}

	h := fnv.New64a()
	h.Write([]byte(uuid))

	requestId := h.Sum64()
	requestTime := c.requestClock.Time()

	future := createFuture(requestId, requestTime, c.memberList.NumMembers(), param,
		func() {
			c.requestLock.Lock()
			delete(c.futures, requestTime)
			c.requestLock.Unlock()
		})

	// book for asynchronous response
	c.requestLock.Lock()
	c.futures[requestTime] = future
	c.requestLock.Unlock()

	err = c.broadcastRequestMessage(requestId, name, requestTime, payload, param)
	if err != nil {
		return nil, err
	}

	return future, nil
}

func (c *cluster) Stopped() <-chan struct{} {
	return c.stop
}

func (c *cluster) GetMemberCount() int {
	c.membersLock.RLock()
	defer c.membersLock.RUnlock()
	return len(c.members)
}

func (c *cluster) NodeStatus() NodeStatus {
	c.nodeStatusLock.RLock()
	defer c.nodeStatusLock.RUnlock()
	return c.nodeStatus
}

func (s *cluster) Members() []Member {
	s.membersLock.RLock()
	defer s.membersLock.RUnlock()

	var ret []Member
	for _, ms := range s.members {
		ret = append(ret, ms.Member)
	}

	return ret
}

////

func (c *cluster) joinNode(node *memberlist.Node) {
	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	tags, err := unpackNodeTags(node.Meta)
	if err != nil {
		logger.Errorf("[unpack node tags from metadata failed, tags are ignored: %s]", err)
	}

	var originalStatus MemberStatus

	ms, existing := c.members[node.Name]
	if !existing {
		originalStatus = MemberNone

		ms = &memberStatus{
			Member: Member{
				nodeName: node.Name,
				nodeTags: tags,
				address:  node.Addr,
				port:     node.Port,
				status:   MemberAlive,
			},
		}

		existing, messageTime := c.memberOperations.get(node.Name, memberJoinMessage)
		if existing {
			ms.lastMessageTime = messageTime
		}

		existing, messageTime = c.memberOperations.get(node.Name, memberLeaveMessage)
		if existing {
			ms.status = MemberLeaving
			ms.lastMessageTime = messageTime
		}

		c.members[node.Name] = ms
	} else {
		originalStatus = ms.status

		ms.nodeTags = tags
		ms.address = node.Addr
		ms.port = node.Port
		ms.goneTime = time.Time{}
		ms.status = MemberAlive
	}

	ms.memberListProtocolMin = node.PMin
	ms.memberListProtocolMax = node.PMax
	ms.memberListProtocolCurrent = node.PCur
	ms.clusterProtocolMin = node.DMin
	ms.clusterProtocolMax = node.DMax
	ms.clusterProtocolCurrent = node.DCur

	if originalStatus == MemberFailed {
		c.failedMembers.remove(ms.nodeName)
	} else if originalStatus == MemberLeft {
		c.leftMembers.remove(ms.nodeName)
	}

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		MemberJoinEvent.String(), ms.nodeName, ms.address, ms.port)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- createMemberEvent(MemberJoinEvent, &ms.Member)

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			MemberJoinEvent.String(), ms.nodeName, ms.address, ms.port)
	}
}

func (c *cluster) leaveNode(node *memberlist.Node) {
	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	ms, ok := c.members[node.Name]
	if !ok {
		logger.Warnf("[node %s is leaving but the membership did not hear, ignored]", node.Name)
		return
	}

	var event EventType
	now := time.Now()

	switch ms.status {
	case MemberLeaving:
		ms.status = MemberLeft
		ms.goneTime = now

		c.leftMembers.add(ms)

		event = MemberLeftEvent
	case MemberAlive:
		ms.status = MemberFailed
		ms.goneTime = now

		c.failedMembers.add(ms)

		event = MemberFailedEvent
	default:
		logger.Errorf("[BUG: invalid member status when the node leave, ignored: %s]", ms.status.String())
		return
	}

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		event.String(), ms.nodeName, ms.address, ms.port)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- createMemberEvent(event, &ms.Member)

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			event.String(), ms.nodeName, ms.address, ms.port)
	}
}

func (c *cluster) updateNode(node *memberlist.Node) {
	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	ms, known := c.members[node.Name]
	if !known {
		logger.Warnf("[node %s is updating but the membership did not hear, ignored]", node.Name)
		return
	}

	tags, err := unpackNodeTags(node.Meta)
	if err != nil {
		logger.Errorf("[unpack node tags from metadata failed, tags are ignored: %s]", err)
	}

	ms.nodeTags = tags
	ms.address = node.Addr
	ms.port = node.Port
	ms.memberListProtocolMin = node.PMin
	ms.memberListProtocolMax = node.PMax
	ms.memberListProtocolCurrent = node.PCur
	ms.clusterProtocolMin = node.DMin
	ms.clusterProtocolMax = node.DMax
	ms.clusterProtocolCurrent = node.DCur

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		MemberUpdateEvent.String(), ms.nodeName, ms.address, ms.port)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- createMemberEvent(MemberUpdateEvent, &ms.Member)

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			MemberUpdateEvent.String(), ms.nodeName, ms.address, ms.port)
	}
}

func (c *cluster) resolveNodeConflict(knownNode, otherNode *memberlist.Node) {
	// TODO(zhiyan): implementation
}

////

func (c *cluster) operateNodeJoin(msg *messageMemberJoin) bool {
	c.memberClock.Update(msg.joinTime)

	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	member, known := c.members[msg.nodeName]
	if !known {
		return c.memberOperations.save(memberJoinMessage, msg.nodeName, msg.joinTime)
	}

	if member.lastMessageTime > msg.joinTime {
		// message is too old, ignore it
		return false
	}

	member.lastMessageTime = msg.joinTime

	if member.status == MemberLeaving {
		member.status = MemberAlive
	}

	return true
}

func (c *cluster) operateNodeLeave(msg *messageMemberLeave) bool {
	c.memberClock.Update(msg.leaveTime)

	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	ms, known := c.members[msg.nodeName]
	if !known {
		return c.memberOperations.save(memberLeaveMessage, msg.nodeName, msg.leaveTime)
	}

	if ms.lastMessageTime > msg.leaveTime {
		// message is too old, ignore it
		return false
	}

	if msg.nodeName == c.conf.NodeName && c.NodeStatus() == NodeAlive {
		go c.broadcastMemberJoinMessage()
		return false
	}

	switch ms.status {
	case MemberAlive:
		ms.status = MemberLeaving
		ms.lastMessageTime = msg.leaveTime
		return true
	case MemberFailed:
		ms.status = MemberLeft
		ms.lastMessageTime = msg.leaveTime

		c.failedMembers.remove(ms.nodeName)
		c.leftMembers.add(ms)

		logger.Debugf("[event %s happened for member %s (%s:%d)]",
			MemberLeftEvent.String(), ms.nodeName, ms.address, ms.port)

		if c.conf.EventStream != nil {
			c.conf.EventStream <- createMemberEvent(MemberLeftEvent, &ms.Member)

			logger.Debugf("[event %s triggered for member %s (%s:%d)]",
				MemberLeftEvent.String(), ms.nodeName, ms.address, ms.port)
		}

		return true
	}

	return false
}

func (c *cluster) operateRequest(msg *messageRequest) bool {
	c.requestClock.Update(msg.requestTime)

	c.requestLock.Lock()
	defer c.requestLock.Unlock()

	care := c.requestOperations.save(msg.requestId, msg.requestTime, c.requestClock.Time())
	if !care {
		return false
	}

	ret := !msg.flag(unicastRequestFlag)

	care = msg.filter(c.conf)
	if !care {
		return ret
	}

	event := createRequestEvent(c, msg)

	if msg.flag(ackRequestFlag) {
		err := event.ack()
		if err != nil {
			logger.Errorf("[send ack to source node failed: %s]", err)
		}
	}

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		RequestReceivedEvent, msg.requestNodeName, msg.requestNodeAddress, msg.requestNodePort)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- event

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			RequestReceivedEvent, msg.requestNodeName, msg.requestNodeAddress, msg.requestNodePort)
	}

	return ret
}

func (c *cluster) operateResponse(msg *messageResponse) bool {
	c.requestLock.RLock()
	defer c.requestLock.RUnlock()

	future, known := c.futures[msg.requestTime]
	if !known {
		logger.Warnf("[BUG: request %s is responded but the request did not send, ignored]", msg.requestName)
		return false
	}

	if future.requestId != msg.requestId {
		logger.Warnf("[BUG: request id %d is mismatch with response %d, ignored]",
			future.requestId, msg.requestId)
		return false
	}

	if future.Closed() {
		return false
	}

	if msg.flag(ackResponseFlag) {
		triggered, err := future.ack(msg.responseNodeName)
		if err != nil {
			logger.Errorf("[trigger response ack event failed: %s]", err)
		}

		if triggered {
			logger.Debugf("[ack of request %s triggered for member %s (%s:%d)]",
				msg.requestName, msg.responseNodeName, msg.responseNodePort)
		}
	} else {
		response := MemberResponse{
			ResponseNodeName: msg.responseNodeName,
			Payload:          msg.responsePayload,
		}

		triggered, err := future.response(&response)
		if err != nil {
			logger.Errorf("[trigger response event failed: %s]", err)
		}

		if triggered {
			logger.Debugf("[response of request %s triggered for member %s (%s:%d)]",
				msg.requestName, msg.responseNodeName, msg.responseNodeAddress, msg.responseNodePort)
		}
	}

	return false
}

func (c *cluster) operateRelay(msg *messageRelay) bool {
	var target *memberlist.Node

	for _, member := range c.memberList.Members() {
		if member.Addr.Equal(msg.targetNodeAddress) && member.Port == msg.targetNodePort {
			target = member
			break
		}
	}

	if target == nil {
		logger.Warnf("[target member (%s:%d) is not available, ignored]",
			msg.targetNodeAddress, msg.targetNodePort)
		return false
	}

	err := c.memberList.SendReliable(target, msg.relayPayload)
	if err != nil {
		logger.Warnf("[forward a relay message to target member (%s:%s) failed, ignored: %s]",
			msg.targetNodeAddress, msg.targetNodePort, err)
		return false
	}

	logger.Debugf("[forward a relay message to target member (%s:%d)]",
		msg.targetNodeAddress, msg.targetNodePort)

	return false
}

////

func (c *cluster) broadcastMemberJoinMessage() error {
	msg := messageMemberJoin{
		nodeName: c.conf.NodeName,
		joinTime: c.memberClock.Time(),
	}

	// handle operation message locally
	c.operateNodeJoin(&msg)

	// send out the node join message
	sentNotify := make(chan struct{})
	defer close(sentNotify)

	err := fanoutMessage(c.memberMessageSendQueue, &msg, memberJoinMessage, sentNotify)
	if err != nil {
		logger.Errorf("[failed to broadcast member join message: %s]", err)
		return err
	}

	select {
	case <-sentNotify:
	case <-time.After(c.conf.MessageSendTimeout):
		return fmt.Errorf("broadcast member join message timtout")
	case <-c.stop:
		return fmt.Errorf("cluster stopped")
	}

	return nil
}

func (c *cluster) broadcastMemberLeaveMessage(nodeName string) error {
	msg := messageMemberLeave{
		nodeName:  nodeName,
		leaveTime: c.memberClock.Time(),
	}

	// handle operation message locally
	c.operateNodeLeave(&msg)

	if !c.anyAlivePeerMembers() {
		// no peer cares
		return nil
	}

	// send out the node leave message
	sentNotify := make(chan struct{})
	defer close(sentNotify)

	err := fanoutMessage(c.memberMessageSendQueue, &msg, memberLeaveMessage, sentNotify)
	if err != nil {
		logger.Errorf("[failed to broadcast member leave message: %s]", err)
		return err
	}

	select {
	case <-sentNotify:
	case <-time.After(c.conf.MessageSendTimeout):
		return fmt.Errorf("broadcast member leave message timtout")
	case <-c.stop:
		return fmt.Errorf("cluster stopped")
	}

	return nil
}

func (c *cluster) broadcastRequestMessage(requestId uint64, name string, requestTime logicalTime,
	payload []byte, param *RequestParam) error {

	var flag uint32
	if param.RequireAck {
		flag |= uint32(ackRequestFlag)
	}

	source := c.memberList.LocalNode()

	msg := messageRequest{
		requestId:          requestId,
		requestName:        name,
		requestTime:        requestTime,
		requestNodeName:    source.Name,
		requestNodeAddress: source.Addr,
		requestNodePort:    source.Port,
		requestFlags:       flag,
		responseRelayCount: param.ResponseRelayCount,
		requestTimeout:     param.Timeout,
		requestPayload:     payload,
	}

	err := msg.applyFilters(param)
	if err != nil {
		return err
	}

	// handle operation message locally
	c.operateRequest(&msg)

	if !c.anyAlivePeerMembers() {
		// no peer can respond
		return nil
	}

	err = fanoutMessage(c.requestMessageSendQueue, &msg, requestMessage, nil) // need not to care if sending is done
	if err != nil {
		logger.Errorf("[failed to broadcast request message: %s]", err)
		return err
	}

	return nil
}

func (c *cluster) aliveMembers(peer bool) []*Member {
	c.membersLock.RLock()
	defer c.membersLock.RUnlock()

	ret := make([]*Member, 0, len(c.members))

	for _, ms := range c.members {
		if ms.nodeName == c.conf.NodeName && peer {
			continue
		}

		if ms.status == MemberAlive {
			ret = append(ret, &ms.Member)
		}
	}

	return ret
}

func (c *cluster) anyAlivePeerMembers() bool {
	return len(c.aliveMembers(true)) > 0
}

////

func (c *cluster) cleanupMember() {
	_cleanup := func(members []*memberStatus) {
		for _, ms := range members {
			delete(c.members, ms.nodeName)

			logger.Debugf("[event %s happened for member %s (%s:%d)]",
				MemberCleanupEvent.String(), ms.nodeName, ms.address, ms.port)

			if c.conf.EventStream != nil {
				c.conf.EventStream <- createMemberEvent(MemberCleanupEvent, &ms.Member)

				logger.Debugf("[event %s triggered for member %s (%s:%d)]",
					MemberCleanupEvent.String(), ms.nodeName, ms.address, ms.port)
			}
		}
	}

	for {
		select {
		case <-time.After(c.conf.MemberCleanupInterval):
			now := time.Now()

			c.membersLock.Lock()
			removedMembers := c.failedMembers.cleanup(now)
			_cleanup(removedMembers)
			removedMembers = c.leftMembers.cleanup(now)
			_cleanup(removedMembers)
			c.memberOperations.cleanup(now)
			c.membersLock.Unlock()
		case <-c.stop:
			break
		}
	}
}

func (c *cluster) reconnectFailedMembers() {
	for {
		select {
		case <-time.After(c.conf.FailedMemberReconnectInterval):
			c.membersLock.RLock()

			if c.failedMembers.Count() == 0 {
				c.membersLock.RUnlock()
				return
			}

			member := c.failedMembers.randGet()

			c.membersLock.RUnlock()

			addr := &net.UDPAddr{IP: member.address, Port: int(member.port)}
			peer := addr.String()

			c.memberList.Join([]string{peer})
		case <-c.stop:
			return
		}
	}
}

package cluster

import (
	"fmt"
	"hash/fnv"
	"net"
	"strings"
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

type Cluster struct {
	conf *Config

	nodeJoinLock   sync.Mutex
	nodeStatusLock sync.RWMutex
	nodeStatus     NodeStatus

	memberList *memberlist.Memberlist

	membersLock                sync.RWMutex
	members                    map[string]*memberStatus
	leftMembers, failedMembers *memberStatusBook
	memberOperations           *memberOperationBook

	requestSendLock   sync.Mutex
	requestLock       sync.RWMutex
	futures           map[logicalTime]*Future
	requestOperations *requestOperationBook

	memberMessageSendQueue, requestMessageSendQueue *memberlist.TransmitLimitedQueue

	memberClock, requestClock logicalClock

	stop chan struct{}
}

func Create(conf Config) (*Cluster, error) {
	if conf.ProtocolVersion < ProtocolVersionMin || conf.ProtocolVersion > ProtocolVersionMax {
		return nil, fmt.Errorf("invalid cluster protocol version %d", conf.ProtocolVersion)

	}

	if len(strings.TrimSpace(conf.NodeName)) == 0 {
		return nil, fmt.Errorf("invalid node name")
	}

	if len(strings.TrimSpace(conf.BindAddress)) == 0 {
		conf.BindAddress = "0.0.0.0"
	}

	if len(strings.TrimSpace(conf.AdvertiseAddress)) == 0 {
		conf.AdvertiseAddress = conf.BindAddress
	}

	if conf.BindPort == 0 {
		conf.BindPort = 9099
	}

	if conf.AdvertisePort == 0 {
		conf.BindPort = conf.AdvertisePort
	}

	if conf.BindPort > 65535 {
		return nil, fmt.Errorf("invalid bind port")
	}

	if conf.AdvertisePort > 65535 {
		return nil, fmt.Errorf("invalid advertise port")
	}

	c := &Cluster{
		conf:              &conf,
		nodeStatus:        NodeAlive,
		members:           make(map[string]*memberStatus),
		leftMembers:       createMemberStatusBook(conf.MemberLeftRecordTimeout),
		failedMembers:     createMemberStatusBook(conf.FailedMemberReconnectTimeout),
		memberOperations:  createMemberOperationBook(conf.RecentMemberOperationTimeout),
		requestOperations: createRequestOperationBook(conf.RecentRequestBookSize),
		futures:           make(map[logicalTime]*Future),
		stop:              make(chan struct{}),
	}

	if conf.NodeTags == nil {
		conf.NodeTags = make(map[string]string)
	}

	nodeTags, err := PackNodeTags(conf.NodeTags)
	if err != nil {
		return nil, err
	}

	if len(nodeTags) > memberlist.MetaMaxSize {
		return nil, fmt.Errorf("tags of the node is too much")
	}

	conf.EventStream = newInternalRequestHandler(c, conf.EventStream)

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

	memberListConf := createMemberListConfig(&conf,
		&eventDelegate{c: c}, &conflictDelegate{c: c}, &messageDelegate{c: c})

	c.memberList, err = memberlist.Create(memberListConf)
	if err != nil {
		return nil, fmt.Errorf("create memberlist failed: %v", err)
	}

	go c.cleanupMember()
	go c.reconnectFailedMembers()

	return c, nil
}

func (c *Cluster) GetConfig() Config {
	// not allow to change Cluster.Config in runtime directly.
	return *c.conf
}

func (c *Cluster) Join(peerNodeNames []string) (int, error) {
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

func (c *Cluster) Leave() error {
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

func (c *Cluster) ForceLeave(nodeName string) error {
	if nodeName == c.conf.NodeName {
		// should go normal Leave() case
		return fmt.Errorf("invalid node")
	}

	return c.broadcastMemberLeaveMessage(nodeName)
}

func (c *Cluster) Stop() error {
	return c.internalStop(true)
}

func (c *Cluster) Request(name string, payload []byte, param *RequestParam) (*Future, error) {
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

	c.requestSendLock.Lock()
	defer c.requestSendLock.Unlock()

	requestTime := c.requestClock.Time()

	msg, err := c.createRequestMessage(requestId, name, requestTime, payload, param)
	if err != nil {
		return nil, err
	}

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

	err = c.broadcastRequestMessage(msg)
	if err != nil {
		return nil, err
	}

	return future, nil
}

func (c *Cluster) Stopped() <-chan struct{} {
	return c.stop
}

func (c *Cluster) GetMemberCount() int {
	c.membersLock.RLock()
	defer c.membersLock.RUnlock()
	return len(c.members)
}

func (c *Cluster) NodeStatus() NodeStatus {
	c.nodeStatusLock.RLock()
	defer c.nodeStatusLock.RUnlock()
	return c.nodeStatus
}

func (c *Cluster) Members() []Member {
	c.membersLock.RLock()
	defer c.membersLock.RUnlock()

	var ret []Member
	for _, ms := range c.members {
		ret = append(ret, ms.Member)
	}

	return ret
}

func (c *Cluster) UpdateTags(tags map[string]string) error {
	nodeTags, err := PackNodeTags(tags)
	if err != nil {
		return err
	}

	if len(nodeTags) > memberlist.MetaMaxSize {
		return fmt.Errorf("tags of the node is too much")
	}

	c.conf.NodeTags = tags

	return c.memberList.UpdateNode(c.conf.MessageSendTimeout)
}

////

func (c *Cluster) joinNode(node *memberlist.Node) {
	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	tags, err := UnpackNodeTags(node.Meta)
	if err != nil {
		logger.Errorf("[unpack node tags from metadata failed, tags are ignored: %v]", err)
	}

	var originalStatus MemberStatus

	ms, existing := c.members[node.Name]
	if !existing {
		originalStatus = MemberNone

		ms = &memberStatus{
			Member: Member{
				NodeName: node.Name,
				NodeTags: tags,
				Address:  node.Addr,
				Port:     node.Port,
				Status:   MemberAlive,
			},
		}

		existing, messageTime := c.memberOperations.get(node.Name, memberJoinMessage)
		if existing {
			ms.lastMessageTime = messageTime
		}

		existing, messageTime = c.memberOperations.get(node.Name, memberLeaveMessage)
		if existing {
			ms.Status = MemberLeaving
			ms.lastMessageTime = messageTime
		}

		c.members[node.Name] = ms
	} else {
		originalStatus = ms.Status

		ms.NodeTags = tags
		ms.Address = node.Addr
		ms.Port = node.Port
		ms.Status = MemberAlive
		ms.goneTime = time.Time{}
	}

	ms.memberListProtocolMin = node.PMin
	ms.memberListProtocolMax = node.PMax
	ms.memberListProtocolCurrent = node.PCur
	ms.clusterProtocolMin = node.DMin
	ms.clusterProtocolMax = node.DMax
	ms.clusterProtocolCurrent = node.DCur

	if originalStatus == MemberFailed {
		c.failedMembers.remove(ms.NodeName)
	} else if originalStatus == MemberLeft {
		c.leftMembers.remove(ms.NodeName)
	}

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		MemberJoinEvent.String(), ms.NodeName, ms.Address, ms.Port)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- createMemberEvent(MemberJoinEvent, &ms.Member)

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			MemberJoinEvent.String(), ms.NodeName, ms.Address, ms.Port)
	}
}

func (c *Cluster) leaveNode(node *memberlist.Node) {
	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	ms, ok := c.members[node.Name]
	if !ok {
		logger.Warnf("[node %s is leaving but the membership did not hear, ignored]", node.Name)
		return
	}

	var event EventType
	now := common.Now()

	switch ms.Status {
	case MemberLeaving:
		ms.Status = MemberLeft
		ms.goneTime = now

		c.leftMembers.add(ms)

		event = MemberLeftEvent
	case MemberAlive:
		ms.Status = MemberFailed
		ms.goneTime = now

		c.failedMembers.add(ms)

		event = MemberFailedEvent
	default:
		logger.Errorf("[BUG: invalid member status when the node leave, ignored: %s]", ms.Status.String())
		return
	}

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		event.String(), ms.NodeName, ms.Address, ms.Port)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- createMemberEvent(event, &ms.Member)

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			event.String(), ms.NodeName, ms.Address, ms.Port)
	}
}

func (c *Cluster) updateNode(node *memberlist.Node) {
	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	ms, known := c.members[node.Name]
	if !known {
		logger.Warnf("[node %s is updating but the membership did not hear, ignored]", node.Name)
		return
	}

	tags, err := UnpackNodeTags(node.Meta)
	if err != nil {
		logger.Errorf("[unpack node tags from metadata failed, tags are ignored: %v]", err)
	}

	ms.NodeTags = tags
	ms.Address = node.Addr
	ms.Port = node.Port
	ms.memberListProtocolMin = node.PMin
	ms.memberListProtocolMax = node.PMax
	ms.memberListProtocolCurrent = node.PCur
	ms.clusterProtocolMin = node.DMin
	ms.clusterProtocolMax = node.DMax
	ms.clusterProtocolCurrent = node.DCur

	logger.Debugf("[event %s happened for member %s (%s:%d)]",
		MemberUpdateEvent.String(), ms.NodeName, ms.Address, ms.Port)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- createMemberEvent(MemberUpdateEvent, &ms.Member)

		logger.Debugf("[event %s triggered for member %s (%s:%d)]",
			MemberUpdateEvent.String(), ms.NodeName, ms.Address, ms.Port)
	}
}

func (c *Cluster) resolveNodeConflict(knownNode, otherNode *memberlist.Node) {
	if c.conf.NodeName != knownNode.Name {
		logger.Warnf("[detected node name %s conflict at %s:%d and %s:%d]", knownNode.Name,
			knownNode.Addr, knownNode.Port, otherNode.Addr, otherNode.Port)
		return
	}

	logger.Errorf("[my node name conflicts with another node at %s:%d]", otherNode.Addr, otherNode.Port)

	logger.Debugf("[start to resolve node name conflict automatically]")

	go c.handleNodeConflict()
}

////

func (c *Cluster) operateNodeJoin(msg *messageMemberJoin) bool {
	c.memberClock.Update(msg.JoinTime)

	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	member, known := c.members[msg.NodeName]
	if !known {
		return c.memberOperations.save(memberJoinMessage, msg.NodeName, msg.JoinTime)
	}

	if msg.JoinTime <= member.lastMessageTime {
		// message is too old, ignore it
		return false
	}

	member.lastMessageTime = msg.JoinTime

	if member.Status == MemberLeaving {
		member.Status = MemberAlive
	}

	return true
}

func (c *Cluster) operateNodeLeave(msg *messageMemberLeave) bool {
	c.memberClock.Update(msg.LeaveTime)

	c.membersLock.Lock()
	defer c.membersLock.Unlock()

	ms, known := c.members[msg.NodeName]
	if !known {
		return c.memberOperations.save(memberLeaveMessage, msg.NodeName, msg.LeaveTime)
	}

	if ms.lastMessageTime > msg.LeaveTime {
		// message is too old, ignore it
		return false
	}

	if msg.NodeName == c.conf.NodeName && c.NodeStatus() == NodeAlive {
		go c.broadcastMemberJoinMessage()
		return false
	}

	switch ms.Status {
	case MemberAlive:
		ms.Status = MemberLeaving
		ms.lastMessageTime = msg.LeaveTime
		return true
	case MemberFailed:
		ms.Status = MemberLeft
		ms.lastMessageTime = msg.LeaveTime

		c.failedMembers.remove(ms.NodeName)
		c.leftMembers.add(ms)

		logger.Debugf("[event %s happened for member %s (%s:%d)]",
			MemberLeftEvent.String(), ms.NodeName, ms.Address, ms.Port)

		if c.conf.EventStream != nil {
			c.conf.EventStream <- createMemberEvent(MemberLeftEvent, &ms.Member)

			logger.Debugf("[event %s triggered for member %s (%s:%d)]",
				MemberLeftEvent.String(), ms.NodeName, ms.Address, ms.Port)
		}

		return true
	}

	return false
}

func (c *Cluster) operateRequest(msg *messageRequest) bool {
	c.requestClock.Update(msg.RequestTime)

	c.requestLock.Lock()
	defer c.requestLock.Unlock()

	care := c.requestOperations.save(msg.RequestId, msg.RequestTime, c.requestClock.Time())
	if !care {
		return false
	}

	ret := !msg.flag(unicastRequestFlag)

	care = msg.filter(c.conf)
	if !care {
		logger.Debugf("[received the request from member %s (%s:%d) but do not care]",
			msg.RequestNodeName, msg.RequestNodeAddress, msg.RequestNodePort)
		return ret
	}

	event := createRequestEvent(c, msg)

	if msg.flag(ackRequestFlag) {
		err := event.ack()
		if err != nil {
			logger.Errorf("[send ack to source node failed: %v]", err)
		}
	}

	logger.Debugf("[event %s happened for the request from member %s (%s:%d)]",
		RequestReceivedEvent, msg.RequestNodeName, msg.RequestNodeAddress, msg.RequestNodePort)

	if c.conf.EventStream != nil {
		c.conf.EventStream <- event

		logger.Debugf("[event %s triggered for the request from member %s (%s:%d)]",
			RequestReceivedEvent, msg.RequestNodeName, msg.RequestNodeAddress, msg.RequestNodePort)
	}

	return ret
}

func (c *Cluster) operateResponse(msg *messageResponse) bool {
	c.requestLock.RLock()
	defer c.requestLock.RUnlock()

	future, known := c.futures[msg.RequestTime]
	if !known {
		logger.Debugf("[response returned after request %s timeout, ignored]", msg.RequestName)
		return false
	}

	if future.requestId != msg.RequestId {
		logger.Warnf("[request id %d is mismatch with the id in response %d, ignored]",
			future.requestId, msg.RequestId)
		return false
	}

	if future.Closed() {
		return false
	}

	if msg.flag(ackResponseFlag) {
		triggered, err := future.ack(msg.ResponseNodeName)
		if err != nil {
			logger.Errorf("[trigger response ack event failed: %v]", err)
		}

		if triggered {
			logger.Debugf("[ack from member %s (%s:%d) for request %s is triggered]",
				msg.ResponseNodeName, msg.ResponseNodeAddress, msg.ResponseNodePort, msg.RequestName)
		}
	} else {
		response := MemberResponse{
			ResponseNodeName: msg.ResponseNodeName,
			Payload:          msg.ResponsePayload,
		}

		triggered, err := future.response(&response)
		if err != nil {
			logger.Errorf("[trigger response event failed: %v]", err)
		}

		if triggered {
			logger.Debugf("[response from member %s (%s:%d) for request %s is triggered]",
				msg.ResponseNodeName, msg.ResponseNodeAddress, msg.ResponseNodePort, msg.RequestName)
		}
	}

	return false
}

func (c *Cluster) operateRelay(msg *messageRelay) bool {
	var target *memberlist.Node

	for _, member := range c.memberList.Members() {
		if member.Addr.Equal(msg.TargetNodeAddress) && member.Port == msg.TargetNodePort {
			target = member
			break
		}
	}

	if target == nil {
		logger.Warnf("[target member (%s:%d) is not available, ignored]",
			msg.TargetNodeAddress, msg.TargetNodePort)
		return false
	}

	err := c.memberList.SendReliable(target, msg.RelayPayload)
	if err != nil {
		logger.Warnf("[forward a relay message to target member (%s:%d) failed, ignored: %v]",
			msg.TargetNodeAddress, msg.TargetNodePort, err)
		return false
	}

	logger.Debugf("[forward a relay message to target member (%s:%d)]",
		msg.TargetNodeAddress, msg.TargetNodePort)

	return false
}

////

func (c *Cluster) broadcastMemberJoinMessage() error {
	msg := messageMemberJoin{
		NodeName: c.conf.NodeName,
		JoinTime: c.memberClock.Time(),
	}

	// handle operation message locally
	c.operateNodeJoin(&msg)

	// send out the node join message
	sentNotify := make(chan struct{})

	err := fanoutMessage(c.memberMessageSendQueue, &msg, memberJoinMessage, sentNotify)
	if err != nil {
		logger.Errorf("[broadcast member join message failed: %v]", err)
		return err
	}

	timer := time.NewTimer(c.conf.MessageSendTimeout)
	defer timer.Stop()

	select {
	case <-sentNotify:
		logger.Debugf("[member join message is sent to the peers]")
	case <-timer.C:
		return fmt.Errorf("broadcast member join message timeout")
	case <-c.stop:
		return fmt.Errorf("cluster stopped")
	}

	return nil
}

func (c *Cluster) broadcastMemberLeaveMessage(nodeName string) error {
	msg := messageMemberLeave{
		NodeName:  nodeName,
		LeaveTime: c.memberClock.Time(),
	}

	// handle operation message locally
	c.operateNodeLeave(&msg)

	if !c.anyAlivePeerMembers() {
		// no peer cares
		return nil
	}

	// send out the node leave message
	sentNotify := make(chan struct{})

	err := fanoutMessage(c.memberMessageSendQueue, &msg, memberLeaveMessage, sentNotify)
	if err != nil {
		logger.Errorf("[broadcast member leave message failed: %v]", err)
		return err
	}

	timer := time.NewTimer(c.conf.MessageSendTimeout)
	defer timer.Stop()

	select {
	case <-sentNotify:
		logger.Debugf("[member leave message is sent to the peers]")
	case <-timer.C:
		return fmt.Errorf("broadcast member leave message timeout")
	case <-c.stop:
		return fmt.Errorf("cluster stopped")
	}

	return nil
}

func (c *Cluster) createRequestMessage(requestId uint64, name string, requestTime logicalTime,
	payload []byte, param *RequestParam) (*messageRequest, error) {

	var flag uint32
	if param.RequireAck {
		flag |= uint32(ackRequestFlag)
	}

	source := c.memberList.LocalNode()

	msg := &messageRequest{
		RequestId:          requestId,
		RequestName:        name,
		RequestTime:        requestTime,
		RequestNodeName:    source.Name,
		RequestNodeAddress: source.Addr,
		RequestNodePort:    source.Port,
		RequestFlags:       flag,
		ResponseRelayCount: param.ResponseRelayCount,
		RequestTimeout:     param.Timeout,
		RequestPayload:     payload,
	}

	err := msg.applyFilters(param)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (c *Cluster) broadcastRequestMessage(msg *messageRequest) error {
	// handle operation message locally
	c.operateRequest(msg)

	if !c.anyAlivePeerMembers() {
		logger.Warnf("[no peer can respond, request ignored]")
		// no peer can respond
		return nil
	}

	sentNotify := make(chan struct{})

	err := fanoutMessage(c.requestMessageSendQueue, msg, requestMessage, sentNotify)
	if err != nil {
		logger.Errorf("[broadcast request message failed: %v]", err)
		return err
	}

	go func() {
		timer := time.NewTimer(msg.RequestTimeout)
		defer timer.Stop()

		select {
		case <-sentNotify:
			logger.Debugf("[request %s is sent to the peers]", msg.RequestName)
		case <-timer.C:
			logger.Warnf("[request %s is timeout before send to the peers]", msg.RequestName)
		case <-c.stop:
			// exits
		}
	}()

	return nil
}

func (c *Cluster) aliveMembers(peer bool) []Member {
	c.membersLock.RLock()
	defer c.membersLock.RUnlock()

	ret := make([]Member, 0)

	for _, ms := range c.members {
		if ms.NodeName == c.conf.NodeName && peer {
			continue
		}

		if ms.Status == MemberAlive {
			ret = append(ret, ms.Member)
		}
	}

	return ret
}

func (c *Cluster) anyAlivePeerMembers() bool {
	return len(c.aliveMembers(true)) > 0
}

func (c *Cluster) handleNodeConflict() {
	msg := messageMemberConflictResolvingRequest{
		ConflictNodeName: c.conf.NodeName,
	}

	buff, err := PackWithHeader(&msg, uint8(memberConflictResolvingRequestMessage))
	if err != nil {
		logger.Errorf("[pack member conflict resolving message failed: %v]", err)
		return
	}

	future, err := c.Request(memberConflictResolvingInternalRequest.String(), buff, nil)
	if err != nil {
		logger.Errorf("[send member conflict resolving request failed: %v]", err)
		return
	}

	var responses, vote int

	local := c.memberList.LocalNode()

LOOP:
	for {
		select {
		case response, ok := <-future.Response():
			if !ok {
				// timeout, vote finished
				break LOOP
			}

			if len(response.Payload) == 0 {
				logger.Errorf("[BUG: received illegal member conflict resolving response message, " +
					"ignored]")
				continue LOOP
			}

			msgType := messageType(response.Payload[0])
			if msgType != memberConflictResolvingResponseMessage {
				logger.Errorf("[BUG: received illegal member conflict resolving response message, "+
					"ignored: %d]", msgType)
				continue LOOP
			}

			msg := messageMemberConflictResolvingResponse{}

			err := Unpack(response.Payload[1:], &msg)
			if err != nil {
				logger.Errorf("[unpack member conflict resolving response message failed: %v]", err)
				continue LOOP
			}

			if msg.Member.Address.Equal(local.Addr) && msg.Member.Port == local.Port {
				vote++
			}

			responses++
		case <-c.stop:
			return
		}
	}

	if vote >= (responses/2)+1 {
		logger.Infof("[I won the vote of node name conflict resolution]")
		return
	} else {
		logger.Infof("[I lost the vote of node name conflict resolution, quit myself from the cluster]")

		err := c.internalStop(false)
		if err != nil {
			logger.Errorf("[quit myself from the cluster failed: %v", err)
		}
	}
}

func (c *Cluster) internalStop(needLeave bool) error {
	c.nodeStatusLock.Lock()
	defer c.nodeStatusLock.Unlock()

	switch c.nodeStatus {
	case NodeShutdown:
		return nil // already stop
	case NodeAlive:
		if needLeave {
			// need to Leave() first
			return fmt.Errorf("invalid node status")
		} else {
			logger.Warnf("[stop cluster without leave]")
		}
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

////

func (c *Cluster) cleanupMember() {
	_cleanup := func(members []*memberStatus) {
		for _, ms := range members {
			delete(c.members, ms.NodeName)

			logger.Debugf("[event %s happened for member %s (%s:%d)]",
				MemberCleanupEvent.String(), ms.NodeName, ms.Address, ms.Port)

			if c.conf.EventStream != nil {
				c.conf.EventStream <- createMemberEvent(MemberCleanupEvent, &ms.Member)

				logger.Debugf("[event %s triggered for member %s (%s:%d)]",
					MemberCleanupEvent.String(), ms.NodeName, ms.Address, ms.Port)
			}
		}
	}

	ticker := time.NewTicker(c.conf.MemberCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := common.Now()

			c.membersLock.Lock()
			removedMembers := c.failedMembers.cleanup(now)
			_cleanup(removedMembers)
			removedMembers = c.leftMembers.cleanup(now)
			_cleanup(removedMembers)
			c.memberOperations.cleanup(now)
			c.membersLock.Unlock()
		case <-c.stop:
			return
		}
	}
}

func (c *Cluster) reconnectFailedMembers() {
	ticker := time.NewTicker(c.conf.FailedMemberReconnectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.membersLock.RLock()

			if c.failedMembers.Count() == 0 {
				c.membersLock.RUnlock()
				return
			}

			member := c.failedMembers.randGet()

			c.membersLock.RUnlock()

			addr := &net.UDPAddr{IP: member.Address, Port: int(member.Port)}
			peer := addr.String()

			c.memberList.Join([]string{peer})
		case <-c.stop:
			return
		}
	}
}

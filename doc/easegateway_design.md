# Table of Contents

* [1. Introduction](#1-introduction)
* [2. Architecture](#2-architecture)
  * [2.1 Groups](#2.1-groups)
* [3. Implementation](#3-implementation)
  * [3.1 Write Requests](#3.1-write-requests)
  * [3.2 Membership Detection And Failure Detection](#3.2-membership-detection-and-failure-detection)
  * [3.3 Fault Tolerance](#3.3-fault-tolerance)
  * [3.4 Replication](#3.4-replication)
  * [3.5 High Availability](#3.5-high-availability)
  * [3.6 Communication Layer](#3.6-communication-layer)
  * [3.7 High Performance](#3.7-high-performance)
  * [3.8 Scaling The Cluster](#3.8-scaling-the-cluster)
  * [3.9 Load Balancing](#3.9-load-balancing)
  * [3.10 System Monitoring](#3.10-system-monitoring)
  * [3.11 System Operations](#3.11-system-operations)

## 1. Introduction
EaseGateway is a decentralized distributed gateway for serving the very large amount of requests coming from different areas while providing highly available service. Gateway provides many [built-in plugins](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md) to complete specific work, users can [create pipelines](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) by assembling different kinds of plugins to complete a business logic.


EaseGateway currently supports many [use cases](https://github.com/hexdecteam/easegateway/blob/master/doc/pipeline_practice_guide.md).

### User defined plugins
In addition to built-in plugins, EaseGateway supports user-defined plugins.

[TODO]  add more details about user-defined plugins

## 2. Architecture
EaseGateway is client/server architecture. There are two kinds of client: admin clients and ordinary clients. Admin clients are used to send administration commands to EaseGateway clusters, such as create pipeline, modify pipeline configurations, retrieve cluster statistics and so on. Ordinary clients are unaware of the existence of the EaseGateway cluster, they just send the requests and get the response as if they are directly communicating with upstream servers.

EaseGateway is designed to handle large requests across multiple nodes with no single point of failure. Its architecture is based on the perception that system and hardware failures always happen. EaseGateway addresses the problem of failures by employing a peer-to-peer distributed system across homogeneous nodes. Data is replicated reasonably among nodes in the cluster. So this mechanism ensures service availability and data durability. EaseGateway is an AP system as regards to the CAP theorem. This implies that it sacrifices some consistency in order to achieve high availability and partition tolerance. Below is the EaseGateway cluster architecture.

![cluster architecture](./diagrams/cluster_architecture.png)

Let's break down the above diagram and describe each piece. First of all, we see that there are two groups, labeled "Beijing" and "Shanghai". Within each group, we have a mixture of one leader and multiple followers. All the nodes in the cluster communicate with each other in gossip protocol. This serves two purposes(@shengdong, I'm little confused about below descriptions):
1. the work of membership and node failure detection is not placed on specific servers but is distributed. This makes failure detection much more scalable than naive heartbeat schemes.
2. it is used as a messaging layer to notify when important events such as leader election take place, or to broadcast application level messages.

### 2.1 Groups
As consensus gets progressively slower as more machines are added, we solve this problem by introducing group. EaseGateway supports multiple groups out of the box, and it can be used to purposely satisfy some use cases (such as flash sale) which need different configurations among groups. What's more, each group elects an independent leader and maintains a disjoint peer set. This design aims to handle write request with lower latency and higher availability without sacrificing much consistency. Requests are forwarded to remote groups if necessary; Connectivity issues between groups do not affect availability within a group. Additionally, the crash of one group does not affect the services of any other groups.

The leader and follower nodes in each group obtain strong consistency by using raft protocol(NOT IMPLEMENTED YET). The observers in the same group obtain eventual consistency by using gossip protocol with copy configurations from leaders/followers.

## 3. Implementation
### 3.1 Write Requests(state changes)

Critical to the design of EaseGateway is the observation that each state change is incremental with respect to the previous state, so there is an implicit dependency on the order of the state changes. State changes are idempotent and applying the same state change multiple times does not lead to inconsistencies as long as the application order is consistent with the delivery order. Each gateway process is built around a single replicated log, we called `Operation Log`. This may be some kind of a single bottleneck at first sight, but since EaseGateway stores pipeline configuration, it's not big application data. Configurations are very small, so this will not be a big problem.

![write ahead log](./diagrams/write_ahead_log.png)

EaseGateway uses [write-ahead-logging(WAL)](https://en.wikipedia.org/wiki/Write-ahead_logging) in which any update details is first written to a log that contains the update information, then write it to `model` and local storage. Only when writing finished successfully, EaseGateway then responds to the client that operation is handled successfully. This ensures that, given a process failure of any sort, EaseGateway can recover from the failure. So WAL enforces atomicity and durability on a standalone EaseGateway. In EaseGateway cluster, we need other mechanisms to ensure atomicity.


Every EaseGateway server serves client requests, as we can see from the above EaseGateway architecture. Clients connect to exactly one server to send requests. Requests are served by each server, write requests are processed by an agreement protocol. As part of the agreement protocol, all write requests from admin clients are forwarded to a single server, called the leader. The rest of the EaseGateway servers, called followers, receive update message proposals from the leader and agree upon message delivery. The atomic broadcast message protocol takes care of replacing leaders on failures and syncing followers with leaders.

### 3.2 Membership Detection And Failure Detection
EaseGateway manages cluster membership and member failure detection using an SWIM-based gossip protocol. We choose third-party library [memberlist](https://github.com/hashicorp/memberlist) as the gossip layer implementation. EaseGateway uses gossip based protocol to sovle three major problems:
1. Membership: EaseGateway maintains cluster membership lists
2. Failure detection and Recovery: EaseGateway can detect failed nodes within seconds, and executes callback processes to handle these events.
3. Application layer message propagation: EaseGateway can disseminate messages through the cluster.

#### SWIM Protocol Overview
Gossip is done over UDP with a configurable but fixed fanout and interval. Each node contains a peer chosen at random every timer interval and the two nodes exchange their membership data. Every node maintains a persistent view of the membership. This ensures that network usage is constant with regards to the amount of nodes. Every node's data is eventually consistent but converges quickly on average.

On top the SWIM-based gossip layer, EaseGateway built an application communication layer, used to send some custom message types, please see section [Network Communication]()

#### Failure Detection
Failure detection is a mechanism by which a node can locally determining if any other node in the system is up or down. We rely on 'memberlist' component to detect node failures.  Failure detection in memberlist is done by periodic random probing using a configurable interval. If the nodes failed to ack within a reasonable time, then an indirect probe as well as a direct TCP probe are attempted. If one node is considered dead by node j, then node j will gossip this state to the cluster. EaseGateway can distinguish between node failure and node leaving and act accordingly.

### 3.3 Fault Tolerance
#### Failure Recovery

When a replica node comes back up after crashing, it gains membership information through gossip and synchronizes data exactly from where it left before the crash by relying on stable storage. It synchronizes data by pulling from the current leader until it is caught up enough. So an EaseGateway node can self-healing.

Upon leader crashes, EaseGateway processes execute a recovery protocol:
1. Agree upon a common consistent state before resuming regular operation
2. Establish a new primary to broadcast state changes

The new leader must come from the follower carrying with newest configurations. To exercise the primary role, a process must have the support of a quorum of follower processes.

EaseGateway can tolerate the maximum of f node failures if the group have 2f + 1 nodes(not include observer nodes).

#### Network partition failure
Network partitions are partially tolerated by attempting to communicate to potentially dead nodes through other routes. You can find more details on section [Network Communication]().

### 3.4 Replication
EaseGateway uses replication to avoid a single point of failure, ensure fault tolerance and achieve high availability. It's [primary-backup schema(master-slave scheme)](https://en.wikipedia.org/wiki/Master-slave_(computers)). In EaseGateway, we have leader and followers basically. The leader is selected from a group of eligible nodes, with the other nodes acting in the role of follower.

![replication mechanism](./diagrams/replication_mechanism.png)

#### Replica Synchronization
EaseGateway uses a leader-based atomic broadcast protocol, just like 'zookeeper atomic broad casting' to synchronize write updates.  It's kind of passive replication, only the write leader executes the incoming client requests, executes them and propagates the resulting non-commutative, incremental state changes to the write followers. Atomically broadcasting protocol will ensure that nodes deliver the same messages and they deliver them in the same order (totally ordered). Because the updates are deterministic, then the state across all replicas is guaranteed to be always consistent. Those updates are idempotent.

#### Quorum-based Writing
we use [quorum](https://en.wikipedia.org/wiki/Quorum_(distributed_computing)) based voting when the process writes request to enforce serial consistency. One disadvantage of majority quorums is their requirement of at least (n+1)/2 acknowledgments, so it grows with the number of replicas n. So this architecture makes it hard to scale out to a huge number of servers. The problem is that as we add more servers, the write performance drops. This is due to the fact that a write operation requires majority consensus before a response is generated[the agreement of (in general) at least half of the nodes in an ensemble] and therefore the cost of a vote can increase significantly as more voters are added. So we introduced a new type of EaseGateway node called an Observer which helps address this problem and further improves scalability of EaseGateway. Observers are non-voting members of an ensemble which only hear the results of votes. Other than this simple distinction, Observers function exactly the same as Followers - clients may connect to them and send requests and write requests to them. Observers forward write requests to the Leader just like Followers do, but they are simply waiting to hear the results of the vote. Because of this, we can increase the number of Observers as much as we like without harming the performance of votes.

### 3.5 High Availability

EaseGateway supports high availability through replication and fast failure recoveries

### 3.6 Communication Layer
As mentioned in above sections, SWIM-based gossip protocol is not only used for membership and failure detection but also to disseminate other messages like requests or responses.

#### Gossip layer
The implementation of gossip protocol Memberlist provided two kinds of transport interface to transport messages:
1. Broadcast to the cluster(via gossip): QueueBroadcast()
2. send to specific node by TCP: sendReliable()

Currently, we have member related messages(LeaveMessage, memberJoinMessage etc), request/response /relay related message. These messages may need to be broadcast to randomly chosen other members according to the needs, such as member related message.

#### Application communication layer
We expose `Request(name string, payload, param) Future` an asynchronous interface to the application layer. App uses this method to communicate with the destination node. Use nodename, group in parm to specify the destinate nodes. The response is always sent by TCP, if failed, then choose a random set of peers to relay this message through TCP. We don't use gossip it's less efficient and we don't need to let the whole group knows this response message.

NotifyMsg is called when a user-data message is received.

Because application level events are sent along the gossip layer, which uses UDP, the payload and entire message framing must fit within a single UDP packet.

When a node joins the cluster, EaseGateway sends a join message. The purpose of this message is sole to attach a logical clock time to a join message so that this node's message can be ordered properly.

When a node gracefully leaves the cluster, this node will send a leave intent through the gossip layer. Because the underlying gossip layer makes no differentiation between a node leaving the cluster and a node being detected as failed, this allows the higher level EaseGateway layer to detect a failure versus a graceful leave.

##### RequestMessage
Request message is broadcast through gossip, easegateway stores request message's context on 'RequestOperationBook', use Request.RequestTime as a stub.

RequestId: uuid,
RequestTime: requestClock.Time() This id will be global unique(cluster wide)
RequestFlags, RequestFilters, RequestPayload

##### ResponseMessage
It's used to carry response payload. It will usually send through TCP if send failed, then EaseGateway will try to send it over relay message.

##### RelayMessage
We relay the message by configurable hops(currently set 1), and then the receiver will send this message through TCP.
RelayMessage structure:
SourceNodeName
TargetNodeAddress
TargetNodePort
RelayPlayload

##### Request Operation Book
We record every Request Message on a book[requestId, time, requestClockTime, two kinds of time, we call it request operation book.

##### Future
requestId
requestTime: it's used to identify it self in requester.
requestDeadline
closed
ackBook: record whether this future needs ack, and which node's ack received
responseBook: record which node's response received
ackStream: output response to observer
responseStream: output response to observer

#### Event

### 3.7 High performance
#### Staged Event drived Architecture

![event driven architecture](./diagrams/event_driven_architecture.png)

### 3.8 Scaling the cluster
We can easily scale the cluster by adding more nodes to the cluster.
#### Bootstraping
When a node needs to join a cluster, it get a list of a few peers specified by options. We call these initial contact peers, seeds of the cluster. Then this node will go through below two phases:

i) synchronization
    Synchronizes the memberlist and gain its data by pulling from the current leader.
ii) became follower or observer

#### Message ordering
EaseGateway makes heavy use of logical clocks(Lamport clocks) to maintain some notion of message ordering despite being eventually consistent. Every message sent by EaseGateway contains a logical clock time.

When a node joins the cluster, EaseGateway sends a join message. The purpose of this message is solely attached a logical clock time to a join so that it can be ordered properly in case a leave comes out of order.

Request time, Member clock time

### 3.9 Load Balancing
Gateway cluster depends on external load balancer such as DNS to do load balancing. As for gateway upstream, we support some route policy like round-robin and weighted round robin policy in [upstreams output plugin](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#upstream-output-plugin).

### 3.10 System Monitoring
EaseGateway exposes system statistics through [Health statistics API](https://github.com/hexdecteam/easegateway/blob/master/doc/statistics_api_ref.swagger.yaml).


### 3.11 System Operations

References:

[Kafka ISR]()

[Zab- High-performance broadcast for primary-backup systems.pdf]

[Scalable Weakly-consistent Infection-style Process Group Membership Protocol.pdf]

[SEDA: An Architecture for Well-Conditioned, Scalable Internet Services]

[The Origin of Quorum Systems.pdf]

[Weighting voting for replicated data.pdf]

[Observer]
[Consul's architecture](https://www.consul.io/docs/internals/architecture.html)
[Cassandra](http://docs.datastax.com/en/cassandra/3.0/cassandra/architecture/archGossipAbout.html)
[Cassandra Architecture](http://docs.datastax.com/en/cassandra/3.0/cassandra/architecture/archGossipAbout.html)
[Dynamo]
[Amazon S3]

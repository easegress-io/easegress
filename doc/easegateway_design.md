# Table of Contents

* [1. Introduction](#1-introduction)
* [2. Architecture](#2-architecture)
  * [2.1 Configuration CRUD](#2.1-configuration-crud)
  * [2.2 Membership Detection And Failure Detection](#2.2-membership-detection-and-failure-detection)
  * [2.3 Fault Tolerance](#2.3-fault-tolerance)
  * [2.4 Replication](#2.4-replication)
  * [2.5 High Availability](#2.5-high-availability)
  * [2.6 Network Communication](#2.6-network-communication)
  * [2.7 High Performance](#2.7-high-performance)
  * [2.8 Scaling The Cluster](#2.8-scaling-the-cluster)
  * [2.9 Load Balancing](#2.9-load-balancing)
  * [2.10 System Monitoring](#2.10-system-monitoring)

## 1. Introduction
Gateway is a decentralized distributed gateway for managing very large amount of requests coming from different areas, while providing highly available service. Gateway provides many [built-in plugins](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md) to complete specific work, user can [create pipelines](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) by assemble different kinds of plugins to complete a business logic.

Easegateway is a AP system with regard to the CAP theorem. This implies that it sacrifices consistency in order to achieve high availability and partition tolerance.

Ease gateway currently support many [use cases](https://github.com/hexdecteam/easegateway/blob/master/doc/pipeline_practice_guide.md).

### Built in plugins

### User defined plugin

## 2. Architecture
### 2.1 Configuration CRUD
Each gateway process is built around a single replicated operation log, we called Operation log. This may be some kind of a single bottleneck at first sight, but because easegateway stores configs, it's not application data. Configs are very small, so this will not be a big problem. Ease gateway uses [write-ahead-logging(WAL)](https://en.wikipedia.org/wiki/Write-ahead_logging
) in which any update details is first written to a log that contains the update information. Only when writing finished successfully, gateway then response to the client that operation finished successfully. This ensures that, given a process failure of any sort, gateway can check the log and compare its contents to the state of the gateway. So WAL enforce atomicity and durability on a standalone Easegateway. In Ease gateway cluster, we need other mechanisms to ensure atomicity.

Critical to the design of Easegateway is the observation that each state change is incremental with respect to the previous state, so there is an implicit dependence on the order of the state changes. State changes are idempotent and applying the same state change multiple times does not lead to inconsistencies as long as the application order is consistent with the deliver order.

#### Local Persistence
Needs a graph to show WAL and local config storage



### 2.2 Membership Detection And Failure Detection
We choose [membership library](https://github.com/hashicorp/memberlist) that manages cluster membership and member failure detection using a gossip based protocol. It's not only used for membership but also to dissemnate other system related control state and application related request or responses. 

Gossip is done over UDP with a configurable but fixed fanout and interval. This ensures that network usage is constant with regards to number of nodes. Memberlist is eventually consistent but converges quickly on average.

#### Failure Detection
Failure detection is a mechanism by which a node can locally determining if any other node int the system is up or down. We rely 'membership' component to detect failures. Easegateway can distinguish between node failure and node leaving. Failure detection in memberlist is done by periodic random probing using a configurable interval. If the nodes fails to ack within a reasonable time, then an indirect probe as well as a direct TCP probe are attempted. If one node is considered dead by node j, then node j will gossip this state to the cluster.

### 2.3 Fault Tolerance

#### Writer Leader failure
Upon primary crashes, Ease gateway processes execute a recovery protocol:

1. Agree upon a common consistent state before resuming regular operation
2. Establish a new primary to broadcast state changes

New write leader must come from the lastest follower. To exercise the primary role, a process must have the support of a quorum of follower processes. 


Writer follower failure
Reader failure
#### Network partition failure
Network partitions are partially tolerated by attempting to communicate to potentially dead nodes through other routes.

#### Failure Recovery

When a replica node comes back up after crashing, it gain membership information through gossip and synchronizes data exactly from where it left before crash by relying on stable storage. It sychronizes data by pulling from the current Write leader/Write follower until it is caught up enough. 

#### Failure Threshold
If f is the maximum number of replica processes that may fail, then we should have 2f+1 nodes.

#### Configuration Management

#### Update Conflicts[Concurrency Control]

### 2.4 Replication
Every Easegateway node is a duplicate, they are the same state machine running with same data?
Gateway uses replication to achieve high availability and durability. It's [primary-backup schema(master-slave scheme)](https://en.wikipedia.org/wiki/Master-slave_(computers)). In easegateway, we have Writer/Reader, writers are selected from a group of eligible nodes, with the other nodes acting in the role of readers. There are three roles Easegateway can perform: writer leader, writer and reader. 
Gateway use replication to ensure fault tolerance and avoid a single point of failure. 

#### Quorum-based Writing
we use Quorum based voting when process writes request.
[Quorum](https://en.wikipedia.org/wiki/Quorum_(distributed_computing))


Can enforce consistency by means of quorum, i.e. a number of synchronously written replicas (W) and a number of replicas to read (R) should be chosen in such a way that W+R>N where N
One disadvantage of majority quorums is their requirement of at least (n+1)/2 acknowledgements, so it grows with the number of replicas n. 

Every transaction collects a read quorum of r votes to read a file, and a write quorum of w votes to write a file such that r+w is greater than the total number of votes assigned to the file. Version numbers make it possible to determine which copies are current. The reliability and performance characteristics of a replicated file can be controlled by appropriately choosing r, w and the file's voting configuration. The algorithm guarantees  serial consistency.

#### Replica Synchronization
Easegateway admin user CRUD with configs, it's small and writes are infrequent.

Easegateway uses a leader-based atomic broadcast protocol, just like 'zookeeper atomic broad casting' to synchronize configs updates.  It's kind of passive replication, only the write leader executes the incoming client requests, executes them and propagates the resulting non-commutative, incremental state changes to the write followers. Atomically boradcasting protocol will ensure that nodes deliver the same messages and they deliver them in the same order (totally ordered). Because the updates are deterministic, then the state across all replicas is guaranteed to be always consistent. Those updates are idempotent. 

### 2.5 High Availability
#### Request Routing

#### Overload Handling

### 2.6 Network Communication
Memberlist library is not only used for membership but also to dissemnate other system related control state and application related request or responses. 

#### Memberlist based communication layer
We use Memberlist provided two kinds of transport interface to transport gateway message:
1. Broadcast to the cluster(via gossip): QueueBroadcast()
2. TCP : sendReliable()

Currently we have member related messages(LeaveMessage, memberJoinMessage etc), request/response /relay related message. These messages may need to be broadcast to randomly choosed other members according to the needs, such as member related message.

NotifyMsg is called when a user-data message is received.
#### Application communication layer
We expose `Request(name string, payload, param) Future` an asynchronous interface to application layer. App use this method to communicate with destination node. 

##### RequestMessage 
Request message is broadcast through gossip, easegateway stores request message's context on 'RequestOperationBook', use Request.RequestTime as a stub.
 
RequestId: uuid, 
RequestTime: requestClock.Time() This id will be global unique(cluster wide)
RequestFlags, RequestFilters, RequestPayload

##### ResponseMessage
It's used to carray response payload. It will usually send through TCP, if send failed, then easegateway will try to send it over relay message.

##### RelayMessage
We only relay message just one hop, and then the receiver will send this message through TCP

##### Request Operation Book
We record every Request Message on a book[requestId, time, requestClockTime ? two kinds of time], we call it request operation book.

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

### 2.7 High performance
#### Staged Event drived Architecture

### 2.8 Scaling the cluster
We can easily scale the cluster by adding more nodes to the cluster.
#### Bootstraping
When a node needs to join a cluster, it get a list of a few peers specified by options. We call these initial contact peers, seeds of the cluster. Then this node will go through below two phases:

i) synchronization
	Synchronizes the memberlist and gain its data by pulling from the current leader.
	
ii) became follower


#### Logical Times
Request time, Member clock time

### 2.9 Load Balancing
Gateway cluster depends on external load balancer such as DNS to do load balancing. As for gateway upstreams, we support some route policy like round-robin and weighted round robin policy in [upstreams output plugin](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#upstream-output-plugin).

### 2.10 System Monitoring
Easegateway expose system staticsics through [Health staticsics API](https://github.com/hexdecteam/easegateway/blob/master/doc/statistics_api_ref.swagger.yaml).


References:

[Zab- High-performance broadcast for primary-backup systems.pdf]

[Scalable Weakly-consistent Infection-style Process Group Membership Protocol.pdf]

[SEDA: An Architecture for Well-Conditioned, Scalable Internet Services]

[The Origin of Quorum Systems.pdf]

[Weighting voting for replicated data.pdf]

[Observer]

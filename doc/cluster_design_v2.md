# Cluster Design of Ease Gateway

## Background
In milestone 2, we wanna deploy `Ease Gateway` with capacity to handle large scale use cases such as flash sale. So it's necessary to solve following cluster issues.

## Design
Overall, we need run lots of instances(we call nodes later) of `Ease Gateway` all around the WAN, and each node need to know the whole status of the cluster at a moment. Next we will show the design of cluster architecture from external and internal perspective in order to achieve the goal.

### External View
#### Group
In the case of `Ease Gateway`, many cases need different configure in different nodes. For example in the scenario of flash sale, since different amounts of online users in different region waiting to rush to buy goods, for fairness we should give different percents of pass for the crowd in corresponding region. It's hard to manually administrate so many nodes carrying with different configure, thus we're supposed to design a general architecture for `Ease Gateway` to automate this kind of problems.

We design an essential logic concept which is `Group` which classifies nodes into different groups. Every group involves one or more nodes. So below diagram shows an overall layout:

![cluster_external_view](**TODO：PICTURE 1**)

In this layout, node `A`, `B` and`C` belong to `Group1`, node `D`, `E`, `F` and `G` belong to Group2. In theory, the business configure of `A` `B` `C` will be the same eventually, so are `D`, `E`, `F`, `G`.


#### Mode
The plain design Group built relationship between nodes carrying with the same configure. But the change of configure is kind of sequential [Delta Update](https://en.wikipedia.org/wiki/Delta_update). For example, a operation deleting a plugin `http_intput.test` will fail if there isn't an successful operation creating a plugin `http_input.test`. So we conclude that the operation of applying configure must obey 2 rules:

1. It does not happen concurrently.
2. It bases on the last version configure of the group.

So we add another concept `Mode` to work with the rules. Mode means the role of a node within its group, there are just two modes `WriteMode` and `ReadMode`. There is one and only one node running on WriteMode and others on ReadMode in a group.

The action of applying configure always happens in the WriteMode node in the corresponding group, and the rest ReadMode nodes just synchronize the action records we call it OPLog(Operation Log) from WriteMode node then apply the OPLog to their local configure. We can regard WriteMode Node the leader of a group, it always keeps last configure and most complete OPLog. The design guaranteed most availability and efficiency but sacrificed some consistency, but we also give the power to administrators for specifying whether to return success until all nodes synchronized the operation.

The most difficult problem about applying configure has been fixed, and we also need cluster provide features about retrieving configure and statistics information. There are a little difference between the three APIs, so we use three vivid diagrams in order to achieve better illustration.

General Rules:

- Any node in the cluster could be REST server.
- A group owns one and only one WriteMode node.
- A group owns zero or more ReadMode nodes.
- Any request has a timeout.

##### Update Configure - Operation
![issue_operation](**TODO：PICTURE 2**)

Particular Rules:
- The request is sent to the WriteMode node directly.
- If the administrator specified flag `consistent`, the request will be responded by the WriteMode node until all nodes of the group updated the operation.

##### Retrieve Configure - Retrieve
![issue_retrieve](**TODO：PICTURE 3**)
Particular Rules:
- The request is sent to the WriteMode node directly.
- If the administrator specified flag `consistent`, the request will be responded by the WriteMode node until the WriteMode Node collected corresponding configure from all ReadMode nodes. And the error will be returned if the configure are different between all node.

##### Retrieve Statistics - Statistics
![issue_statistics](**TODO：PICTURE 3**)
Particular Rules:
- The request is sent to any one ReadMode node if there is, otherwise sent to the WriteMode node. The WriteMode node always keep the last version of configure so the Operation and Retrieve must be sent to it, but the statistics has no version concept. So it's a load-balance decision for reduce load pressure of WriteMode node.
- The request is responded until the chosen node aggregated statistics of all nodes.

### Internal View
In internal architecture, we split the cluster feature into two layers: `Upper Layer` and `Lower Layer`.
![cluster_internal_view](**TODO：PICTURE 4**)

Lower layer provides two APIs:
1. Request: This is a very flexible and powerful API, the upper layer plays with it most of the time. Callers could use it to broadcast requests to particular nodes such as all nodes in the cluster, all nodes in Group1, the WriteMode node in Group2, all ReadMode nodes in Group2, etc.
2. Nodes' up, down and update：The status of any node of cluster will be notified through this API. The `update` means the metadata of a node changed, for example its group attribute is updated from Group1 to Group2.

Upper layer implements the business logic, just as diagrams of external view there are mainly three kinds of exported APIs(Operation, Retrieve, Statistics), and internally plus one kind of not-exported API synchronize OPLog. These APIs are implemented based on `Request` of lower layer. Moreover, the upper layer also provides fine-grained business APIs such as creating a plugin, deleting a pipeline and so on in a group, generally speaking the REST server as a typical client will invoke these APIs directly.

### Clock Synchronization
The physical/wall clock can not fix global synchronization in distributed system. There have been already multiple ways to solve it. In `Ease Gateway`, there is no need to use complex vector clock, and then logical clock(lamport clock) satisfies the case enough. The original and complete source about lamport clock is the paper [Time, Clocks and the Ordering of Events in a Distributed System](http://lamport.azurewebsites.net/pubs/time-clocks.pdf).

Here we just introduce it in a simple way. Lamport clock is a mechanism for capturing chronological and causal relationships in a distributed system. According to [Wikipedia](https://en.wikipedia.org/wiki/Lamport_timestamps), it follows 3 simple rules:

> 1. A process increments its counter before each event in that process;
> 2. When a process sends a message, it includes its counter value with the message;
> 3. On receiving a message, the counter of the recipient is updated, if necessary, to the greater of its current counter and the timestamp in the received message. The counter is then incremented by 1 before the message is considered received.

The implementation could be found at [logic_lock](**TODO: code link**)

### Data Persistence
The configure of business keep the same with data of standalone version. The OPLog stores all operations about updating configure in json format sequentially which is friendly to debug.

### Serialization
We choose [msgpack](https://github.com/ugorji/go) to serialize switch message, it costs less bits than json and be more friendly to the type system of Golang.

## API Specification (TODO)

## References
Sorted by occurrence time.

1. [Time, Clocks and the Ordering of Events in a Distributed System](http://lamport.azurewebsites.net/pubs/time-clocks.pdf)
2. [Wikipedia: Lamport Clock](https://en.wikipedia.org/wiki/Lamport_timestamps)

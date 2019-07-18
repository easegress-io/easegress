# 1 集群

做为网关集群，需要满足以下特性：

- 弹性集群，能动态增减节点数目
- 数据强一致性
- 强一致节点需能自动选举，避免单点故障
- 任一节点均暴露API，方便运维
- RESTful API进行集群管理，利于多平台集成
- 集群可观测，利用API能获取集群的健康、心跳、负载等信息

集群节点分为两类角色：

- Writer：存储配置、监控数据、运行时等数据的节点，强一致；根据集群大小，一般为1、3、5、7个
- Reader：作为无状态节点Obeserve Writer节点；用于大量水平扩容的节点

## 1.1 组件

Writer为了满足强一致分布式存储，嵌入了Etcd，所以Writer节点的分布式存储集群本质是一个Etcd集群。Etcd使用了Raft协议，在协议层上，集群的每一个节点都必须知道集群的其他所有节点的信息，所以Etcd本身大多时候是静态部署，即先规划再部署，然后通过Member API去实现动态扩缩集群。

Reader作为Writer的观察者，只需简单地注册自己，观察存储过程即可，本质上就是Etcd的一个个客户端。

EaseGateway作为流量接入中间件，不可避免地需要动态扩缩容，所以我们要将其自动化，让运维人员无需做复杂的手工去扩缩。在详细展示扩缩流程前，我们先定义每个用到的内部组件，使此后的流程描述更加清晰。

- Server：Etcd Server，只运行在Writer节点上
- Client：Etcd Client，运行在每一个节点上
- Lease：每个节点有且仅有一个的长期租约（285年：）
- KnownMembers：当前节点已知所有Writer节点的集群信息，只增不减，会持久化到本地磁盘
- ClusterMembers：当前节点已知集群当前运行着的所有Writer节点，可增可减，会持久化到本地磁盘

## 1.2 扩容

扩容的运维流程很简单，填写好争取的启动配置文件，启动EaseGateway即可。EaseGateway会自动化所有启动流程，其内部逻辑为：
如果为Reader，其中任意一步失败，跳转到步骤1：

1. 获取Client
2. 初始化Lease

如果为Writer：

1. 如果KnownMembers多于1个，则获取Client，如果成功则将自己添加到集群中
   1. 获取MemberList
   2. 如果自己已在MemberList（ID一致）则返回，判断时要考虑ID为0的情况
   3. 如果MemberList有相同Name但ID不一致则将老的Member删除，判断时要考虑ID为0的情况
   4. 将自己添加到MemberList
   5. 更新ClusterMembers
   6. 备份并清空Server的Data目录
2. 异步启动Server
3. 如果Server启动完成，则确认Client已获取
4. 如果Server启动超时，则跳转到步骤1
5. 初始化Lease

步骤1里将自己添加到集群，在所有节点down掉的情况下，第一个重新启动的节点会在此步骤阻塞一段时间并失败，但并不影响整个集群的重新启动。

## 1.3 缩容

缩容需提前将该节点停掉，然后用Purge Member API，吊销其Lease，使与该Lease挂钩的所有数据清除掉即可。

## 1.4 心跳

在每个节点的集群组件正常启动后，会每隔一段时间（5s）向分布式存储通过自己的状态，还有主动获取Member的变化，用于更新ClusterMembers/KnownMembers。
节点同步自己的状态主要包括：

- 静态的启动参数：方便管理员快速远程查看
- 此次心跳时间：间隔为5s，客户端可以认为如果获取的心跳时间已经在10s之前，则该节点出现故障，需要排查
- Etcd状态（仅Writer）：ID，启动时间、State（Leader，PreCandidate，Candidate，Follower）

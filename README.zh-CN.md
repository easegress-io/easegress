# Easegress

<a href="https://megaease.com/easegress">
    <img src="./doc/easegress.svg"
        alt="Easegress logo" title="Easegress" height="175" width="175" align="right"/>
</a>

- [Easegress](#easegress)
  - [什么是Easegress](#什么是easegress)
  - [功能](#功能)
  - [用户案例](#用户案例)
  - [入门](#入门)
    - [设置 Easegress](#设置-easegress)
    - [测试](#测试)
    - [更多过滤器](#更多过滤器)
  - [文档](#文档)
  - [路线图](#路线图)
  - [社区](#社区)
  - [许可证](#许可证)

## Easegress 简介

`Easegress`是一个云原生流量协调系统，旨在提供：

- **高可用性：** 内置Raft共识和选主算法，提供99.99%的可用性。
- **流量编排：** 支持多种流量过滤器，轻松编排流量处理流程（Pipeline）。
- **高性能：** 基础功能采用轻量级方法实现，性能优异。
- **可观察性：** 周期性报告多种统计数据，系统状态尽在掌握。
- **可扩展性：** 良好的 API 设计，不必知道底层细节，也能自己开发过滤器和控制器。
- **集成性：** 接口简单，易于与其他系统集成，如Kubernetes Ingress、[EaseMesh](https://github.com/megaease/easemesh) 边车、工作流等。

Easegress的架构。

![架构](./doc/architecture.png)

## 功能

- **服务管理**
	- **多种协议：**
		- HTTP/1.1
		- HTTP/2
		- HTTP/3(QUIC)
		- MQTT(即将推出)
	- **丰富的路由规则：** 精确路径、路径前缀、路径的正则表达式、方法、头文件。
	- **弹性和容错**。
		- **断路器：** 暂时阻止可能的故障。
		- **速率限制器：** 限制传入请求的速率。
		- **重播器：** 重复失败的执行。
		- **时间限制器：** 限制执行的时间。
	- **部署管理**
		- **蓝绿策略：** 一次性切换流量。
		- **金丝雀策略：** 按着色安排流量。
	- **API管理**
		- **API聚合：** 聚合多个API的结果。
		- **API协调：** 协调API的流动。
	- **安全**
		- **IP过滤：** 限制对IP地址/地址段的访问。
		- **静态HTTPS：** 静态证书文件。
		- **API签名：** 支持[HMAC]（https://en.wikipedia.org/wiki/HMAC）验证。
		- **JWT Verification:** 验证[JWT Token](https://jwt.io/)。
		- **OAuth2:** 验证[OAuth/2](https://datatracker.ietf.org/doc/html/rfc6749)请求。
		- **Let's Encrypt:** 自动管理证书文件。
	- **管道过滤机制**。
		- **责任链模式：** 协调过滤器链。
		- **过滤器管理：** 使开发新过滤器变得容易。
	- **服务网格**
		- ** 网格主控：** 是管理网格服务生命周期的控制平面。
		- **Mesh Sidecar:** 是数据平面，作为端点进行流量拦截和路由。
		- **网格入口控制器：** 是针对网格的入口控制器，将外部流量路由到网格服务。
		  > 注意，该功能被[EaseMesh](https://github.com/megaease/easemesh)所使用。
	- **第三方的集成**
		- **FaaS** 与无服务器平台Knative集成。
		- **服务发现** 与Eureka、Consul、Etcd和Zookeeper集成。
		- **入站控制器** 与Kubernetes集成，作为入站控制器。
- **扩展性**
    - **WebAssembly**执行用户开发的[WebAssembly](https://webassembly.org/)代码。
- **高性能和可用性**
	- **适应性**：在处理链中适应请求和响应。
	- **验证**：头文件验证、OAuth2、JWT和HMAC验证。
	- **负载平衡：** 轮询、随机、加权随机、IP哈希、HTTP头信息哈希。
	- **缓存：** 用于后端服务器。
	- **压缩：** 压缩响应的主体。
	- **热更新：** 更新Easegress的配置和二进制文件，不会丢失连接。
- **操作**
	- **易于集成：** 命令行(`egctl`)、MegaEase Portal、HTTP 客户端，如 curl、postman 等。
	- **分布式跟踪**
		- 内置[Open Zipkin](https://zipkin.io/)
		- [Open Tracing](https://opentracing.io/)为厂商中立的API。
	- **可观察性**
		- **节点：** 角色（领导者、写者、读者）、健康与否、最后一次心跳时间，等等
		- **流量：** 多维度的：服务器和后端。
			- **吞吐量:** 请求数、TPS/m1、m5、m15和错误百分比等的总和错误统计。
			- **延迟：** p25、p50、p75、p95、p98、p99、p999。
			- **数据大小：** 请求和响应大小。
			- **状态代码:** HTTP状态代码。
			- **TopN:** 按聚合的API排序（仅在服务器维度）。


## 用户案例

下面的例子展示了如何在不同场景下使用Easegress。

- [API Aggregator](./doc/cookbook/api_aggregator.md) - 将业务关联 API  聚合为单个 API。
- [FaaS](./doc/cookbook/faas.md) - 支持Knative FaaS集成。
- [Flash Sale](./doc/cookbook/flash_sale.md) - 如何使用Easegress进行高并发的秒杀活动。
- [LoadBalancer](./doc/cookbook/load_balancer.md) - 负载均衡的若干策略 
- [Distributed Tracing](./doc/cookbook/distributed_tracing.md) - 如何进行APM追踪 - Zipkin。
- Kubernetes入口控制器](./doc/cookbook/k8s_ingress_controller.md) - 如何与Kubernetes集成作为入口控制器。
- 性能](./doc/cookbook/performance.md) - 性能优化 - 压缩、缓存等。
- [Pipeline](./doc/cookbook/pipeline.md) - 如何为请求/响应处理协调HTTP过滤器。
- [Resilience and Fault Tolerance](./doc/cookbook/resilience.md) - 断路器、速率限制器、回流器、时间限制器等（移植自[Java resilience4j](https://github.com/resilience4j/resilience4j)
- [Security](./doc/cookbook/security.md) - 如何通过Header、JWT、HMAC、OAuth2等进行认证。
- [Service Proxy](./doc/cookbook/service_proxy.md) - 支持微服务注册表 - Zookeeper、Eureka、Consul、Nacos等。
- [WebAssembly](./doc/cookbook/wasm.md) - 使用AssemblyScript来扩展Easegress
- [Workflow](./doc/cookbook/workflow.md) - 将若干 API 进行组合，定制为工作流。

完整的列表请参见 [Cookbook](./doc/cookbook/README.md)。


## 入门

Easegress的基本通用用法是为后端服务器快速设置代理。我们把它分成多个简单的步骤来说明基本概念和操作。

### 设置 Easegress

我们可以从[发布页](https://github.com/megaease/easegress/releases)下载二进制文件。  
例如，我们使用linux版本。

``` bash
$ mkdir easegress
$ wget https://github.com/megaease/easegress/releases/download/v1.1.0/easegress-v1.1.0-linux-amd64.tar.gz
$ tar zxvf easegress-v1.1.0-linux-amd64.tar.gz -C easegress && cd easegress
```

或使用源代码。

```bash
$ git clone https://github.com/megaease/easegress && cd easegress
$ make
```

然后我们可以把二进制目录添加到`PATH`中，并执行服务器。

``` bash
$ export PATH=${PATH}:$(pwd)/bin/
$ easegress-server
2021-05-17T16:45:38.185+08:00	INFO	cluster/config.go:84	etcd config: init-cluster:eg-default-name=http://localhost:2380 cluster-state:new force-new-cluster:false
2021-05-17T16:45:38.185+08:00	INFO	cluster/cluster.go:379	client is ready
2021-05-17T16:45:39.189+08:00	INFO	cluster/cluster.go:590	server is ready
2021-05-17T16:45:39.21+08:00	INFO	cluster/cluster.go:451	lease is ready
2021-05-17T16:45:39.231+08:00	INFO	cluster/cluster.go:187	cluster is ready
2021-05-17T16:45:39.253+08:00	INFO	supervisor/supervisor.go:180	create system controller StatusSyncController
2021-05-17T16:45:39.253+08:00	INFO	cluster/cluster.go:496	session is ready
2021-05-17T16:45:39.253+08:00	INFO	api/api.go:96	api server running in localhost:2381
2021-05-17T16:45:44.235+08:00	INFO	cluster/member.go:210	self ID changed from 0 to 689e371e88f78b6a
2021-05-17T16:45:44.236+08:00	INFO	cluster/member.go:137	store clusterMembers: eg-default-name(689e371e88f78b6a)=http://localhost:2380
2021-05-17T16:45:44.236+08:00	INFO	cluster/member.go:138	store knownMembers  : eg-default-name(689e371e88f78b6a)=http://localhost:2380
```

Makefile的默认目标是将两个二进制文件编译到`bin/`目录中。`bin/easegress-server`是服务器端的二进制文件，`bin/egctl`是客户端的二进制文件。我们可以把它添加到`$PATH`中，以简化下面的命令。

我们可以运行`easegress-server`而不指定任何参数（服务默认依赖于2379、2380、2381端口）。  
> 我们可以在配置文件中更改默认端口，或者在命令行启动时指定相关参数（参数具体释义可通过执行`easegress-server --help`命令获取）。

``` bash
$ egctl member list
- options:
    name: eg-default-name
    labels: {}
    cluster-name: eg-cluster-default-name
    cluster-role: writer
    cluster-request-timeout: 10s
    cluster-listen-client-urls:
    - http://127.0.0.1:2379
    cluster-listen-peer-urls:
    - http://127.0.0.1:2380
    cluster-advertise-client-urls:
    - http://127.0.0.1:2379
    cluster-initial-advertise-peer-urls:
    - http://127.0.0.1:2380
    cluster-join-urls: []
    api-addr: localhost:2381
    debug: false
    home-dir: ./
    data-dir: data
    wal-dir: ""
    log-dir: log
    member-dir: member
    cpu-profile-file: ""
    memory-profile-file: ""
  lastHeartbeatTime: "2021-05-05T15:43:27+08:00"
  etcd:
    id: a30c34bf7ec77546
    startTime: "2021-05-05T15:42:37+08:00"
    state: Leader
```

成功启动后，我们可以检查单节点集群的状态。它显示了静态选项以及心跳和etcd的动态状态。

###创建一个HTTPServer和管道

现在让我们创建一个HTTPServer，在10080端口监听，以处理HTTP流量。

``` bash
$ echo '
name: pipeline-demo
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin' | egctl object create
```

上面的路由器规则意味着它将把前缀为`/pipeline`的流量引向管道`pipeline-demo`，它将在下面被创建。如果我们在创建之前`curl`它，它将返回503。


### 测试

现在你可以使用一些HTTP客户端，如`curl`来测试这个功能。

``` bash
$ curl -v http://127.0.0.1:10080/pipeline
```

如果你没有设置一些应用程序来处理本地主机中的9095、9096和9097，它也会返回503。我们准备了一个简单的服务，让我们方便地测试，例子显示。

```bash
$ go run example/backend-service/mirror/mirror.go & # Running in background
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
Your Request
===============
Method: POST
URL   : /pipeline
Header: map[Accept:[*/*] Accept-Encoding:[gzip] Content-Type:[application/x-www-form-urlencoded] User-Agent:[curl/7.64.1]]
Body  : Hello, Easegress
```

### 更多过滤器

现在我们想给管道添加更多的功能，那么我们可以给管道添加各种过滤器。例如，我们希望对`pipeline-demo`进行验证和请求适应。

<p align="center">
  <img src="./doc/pipeline-demo.png" width=240>
</p>


```bash
$ cat pipeline-demo.yaml
name: pipeline-demo
kind: HTTPPipeline
flow:
  - filter: validator
    jumpIf: { invalid: END }
  - filter: requestAdaptor
  - filter: proxy
filters:
  - name: validator
    kind: Validator
    headers:
      Content-Type:
        values:
        - application/json
  - name: requestAdaptor
    kind: RequestAdaptor
    header:
      set:
        X-Adapt-Key: goodplan
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin

$ egctl object update -f pipeline-demo.yaml
```

更新管道后，原来的`curl -v http://127.0.0.1:10080/pipeline`会因为验证而得到400。所以我们改变了它以满足限制。

```bash
$ curl http://127.0.0.1:10080/pipeline -H 'Content-Type: application/json' -d '{"message": "Hello, Easegress"}'
Your Request
===============
Method: POST
URL   : /pipeline
Header: map[Accept:[*/*] Accept-Encoding:[gzip] Content-Type:[application/json] User-Agent:[curl/7.64.1] X-Adapt-Key:[goodplan]]
Body  : {"message": "Hello, Easegress"}
```

我们还可以看到Easegress又向镜像服务发送了一个头`X-Adapt-Key: goodplan`。


## 文档

更多信息请参见[reference](./doc/reference.md)和[developer guide](./doc/developer-guide.md)。

## 路线图

参见[Easegress 路线图](./doc/Roadmap.md)了解详情。

## 社区

- [加入Slack工作区](https://join.slack.com/t/openmegaease/shared_invite/zt-upo7v306-lYPHvVwKnvwlqR0Zl2vveA)了解需求、问题讨论和解决。
- [MegaEase on Twitter](https://twitter.com/megaease)


## 许可证

Easegress采用Apache 2.0许可证。详情请参见 [LICENSE](./LICENSE) 文件。
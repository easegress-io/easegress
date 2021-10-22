# Easegress

<a href="https://megaease.com/easegress">
    <img src="./doc/easegress.svg"
        alt="Easegress logo" title="Easegress" height="175" width="175" align="right"/>
</a>

- [Easegress](#easegress)
  - [Easegress 简介](#easegress-简介)
  - [功能](#功能)
  - [用户案例](#用户案例)
  - [入门](#入门)
    - [安装 Easegress](#安装-easegress)
    - [创建 HTTPServer 和 Pipeline](#创建-httpserver-和-pipeline)
    - [测试](#测试)
    - [使用更多过滤器](#使用更多过滤器)
  - [文档](#文档)
  - [路线图](#路线图)
  - [社区](#社区)
  - [许可证](#许可证)

## Easegress 简介
	
`Easegress`是一个云原生流量协调系统，具有以下特性：
	
- **高可用性：** 内置 Raft 共识和选举算法，提供 99.99% 的可用性。
- **流量编排：** 支持多种流量过滤器，轻松编排流量处理流程（Pipeline）。
- **高性能：** 基础功能采用轻量级方法实现，性能优异。
- **可观察性：** 周期性报告多种统计数据，系统状态尽在掌握。
- **可扩展性：** 良好的 API 设计，不必知道底层细节，也能自己开发过滤器和控制器。
- **集成性：** 接口简单，易于与其他系统集成，如 Kubernetes 入口控制器、[EaseMesh](https://github.com/megaease/easemesh) 边车、工作流等。

下面是其架构图：

![架构](./doc/architecture.png)

## 功能

- **服务管理**
	- **支持多种协议**
		- HTTP/1.1
		- HTTP/2
		- HTTP/3(QUIC)
		- MQTT
	- **路由规则**：精确路径、路径前缀、路径的正则表达式、方法、标头。
	- **弹性和容错**。
		- **断路器**： 暂时阻止可能的故障。
		- **速率限制**： 限制请求的速率。
		- **重试**：重试失败的请求。
		- **时间限制**：限制请求的执行时间。
	- **部署管理**
		- **蓝绿部署**：一次性切换流量。
		- **金丝雀部署**：按着色编排流量。
	- **API管理**
		- **API聚合**：聚合多个API的结果。
		- **API编排**：编排API的处理流程。
	- **安全**
		- **IP过滤**：限制对IP地址/地址段的访问。
		- **静态HTTPS**：静态证书文件。
		- **API签名**：支持 [HMAC](https://en.wikipedia.org/wiki/HMAC) 验证。
		- **JWT验证**：验证 [JWT Token](https://jwt.io/)。
		- **OAuth2**：验证 [OAuth/2](https://datatracker.ietf.org/doc/html/rfc6749) 请求。
		- **Let's Encrypt:** 自动管理证书文件。
	- **管道过滤机制**。
		- **责任链模式**：编排过滤器链。
		- **过滤器管理**：轻松开发新过滤器。
	- **服务网格**
		- **网格主控**：是管理网格服务生命周期的控制平面。
		- **边车**：是数据平面，作为端点进行流量拦截和路由。
		- **网格入口控制器**：是针对网格的入口控制器，将外部流量路由到网格服务。
		  > 注意，[EaseMesh](https://github.com/megaease/easemesh)使用了此功能。
	- **第三方的集成**
		- **FaaS**：与 ServerLess 平台 Knative 集成。
		- **服务发现**：与 Eureka、Consul、Etcd 和 Zookeeper 集成。
		- **入口控制器**：与 Kubernetes 集成，作为入口控制器。
- **扩展性**
    - **WebAssembly**：执行用户开发的 [WebAssembly](https://webassembly.org/) 代码。
- **高性能和可用性**
	- **改编**：使用过滤器改编请求和应答。
	- **验证**：标头验证、OAuth2、JWT 和 HMAC 验证。
	- **负载平衡**：轮询、随机、加权随机、IP哈希、标头哈希。
	- **缓存**：缓存后端服务的应答，减少对后端服务的请求量。
	- **压缩**：减少应答数据的体积。
	- **热更新**：线上更新 Easegress 的配置和二进制文件，服务不中断。
- **操作**
	- **易于集成**：命令行(`egctl`)、MegaEase Portal，以及 HTTP 客户端，如 curl、postman 等。
	- **分布式跟踪**
		- 内置 [Open Zipkin](https://zipkin.io/)
		- [Open Tracing](https://opentracing.io/)，提供厂商中立的 API。
	- **可观察性**
		- **节点**：角色（Leader、Writer、Reader）、健康状态、最后一次心跳时间，等等。
		- **多维度的服务器和后端流量数据**
			- **吞吐量**：请求数、TPS/m1、m5、m15 和错误百分比等。
			- **延迟**：p25、p50、p75、p95、p98、p99、p999。
			- **数据大小**：请求和响应大小。
			- **状态代码**：HTTP状态代码。
			- **TopN**：按 API 聚合并排序（仅服务器维度）。


## 用户案例

下面的例子展示了如何在不同场景下使用 Easegress。

- [API 聚合](./doc/cookbook/api_aggregator.md) - 将多个 API 聚合为一个。
- [分布式调用链](./doc/cookbook/distributed_tracing.md) - 如何使用 Zipkin 进行 APM 追踪。
- [函数即服务 FaaS](./doc/cookbook/faas.md) - 支持 Knative FaaS 集成。
- [高并发秒杀](./doc/cookbook/flash_sale.md) - 如何使用 Easegress 进行高并发的秒杀活动。
- [Kubernetes入口控制器](./doc/cookbook/k8s_ingress_controller.md) - 如何作为入口控制器与 Kubernetes 集成。
- [负载均衡](./doc/cookbook/load_balancer.md) - 各种负载均衡策略。
- [MQTT代理](./doc/cookbook/mqtt_proxy.md) - 支持 Kafka 作为后端的 MQTT 代理
- [高性能](./doc/cookbook/performance.md) - 性能优化，压缩、缓存等。
- [管道编排](./doc/cookbook/pipeline.md) - 如何编排 HTTP 过滤器来处理请求和应答。
- [弹力和容错设计](./doc/cookbook/resilience.md) - 断路器、速率限制、重试、时间限制等（移植自[Java resilience4j](https://github.com/resilience4j/resilience4j)
- [安全](./doc/cookbook/security.md) - 如何通过标头、JWT、HMAC、OAuth2 等进行认证。
- [服务网关](./doc/cookbook/service_proxy.md) - 使用 Zookeeper、Eureka、Consul、Nacos 等进行服务注册。
- [WebAssembly](./doc/cookbook/wasm.md) - 使用 AssemblyScript 来扩展 Easegress。
- [工作流](./doc/cookbook/workflow.md) - 将若干 API 进行组合，定制为工作流。

完整的列表请参见 [Cookbook](./doc/cookbook/README.md)。


## 入门

Easegress 的基本用法是做为后端服务器的代理。下面分步说明相关基本概念和操作方法。

### 安装 Easegress

我们可以从[发布页](https://github.com/megaease/easegress/releases)下载 Easegress 的最新或历史版本。例如，可以使用下面的命令安装 v1.0.0 的 Linux：

```bash
$ mkdir easegress
$ wget https://github.com/megaease/easegress/releases/download/v1.0.0/easegress-v1.0.0-linux-amd64.tar.gz
$ tar zxvf easegress-v1.0.0-linux-amd64.tar.gz -C easegress && cd easegress
```

或者，也可以通过源码安装：

```bash
$ git clone https://github.com/megaease/easegress && cd easegress
$ make
```

然后把二进制所在目录添加到 `PATH` 中，并启动服务：

```bash
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

Makefile 默认会将两个二进制文件编译到 `bin/` 目录中。`bin/easegress-server` 是服务器端的二进制文件，`bin/egctl` 是客户端的二进制文件。我们可以把它添加到 `$PATH` 中，以便于执行后续命令。

如果启动时不指定任何参数，`easegress-server` 会默认使用端口 2379、2380 和 2381。我们可以在配置文件中更改默认端口，或者在命令行启动时指定相关参数（参数具体释义可通过执行 `easegress-server --help` 命令获取）。

```bash
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

成功启动后，我们可以用上述命令检查单节点集群的状态，它展示示了系统的静态选项，以及心跳和etcd的动态状态。

### 创建 HTTPServer 和 Pipeline

现在我们可以创建一个监听 10080 端口的 HTTPServer 来接收 HTTP 流量。

```bash
$ echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: pipeline-demo' | egctl object create
```

上面的路由规则将把路径前缀为 `/pipeline` 的请求分发到名为 `pipeline-demo` 的 Pipeline，目前还没有这条 Pipeline，如果 `curl` 这个地址，将返回 503。

```bash
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

这条 Pipeline 的定义是将请求按轮询的方式分发到三个后端服务实例上。

### 测试

现在可以使用一个 HTTP 客户端，如 `curl` 进行测试：

```bash
$ curl -v http://127.0.0.1:10080/pipeline
```

在没有后端程序处理本机端口 9095、9096 和 9097 的流量时，它也会返回503。为便于测试，我们提供了一个简单的服务程序，使用方法如下：

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

### 使用更多过滤器

现在我们可以给 Pipeline 添加其它过滤器来实现更多的功能，例如，如果希望对 `pipeline-demo` 可以验证和改写请求，可以这样做：

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

更新 Pipeline 后，再次执行的 `curl -v http://127.0.0.1:10080/pipeline` 会因为验证失败而得到 400。下面我们修改请求来通过验证：

```bash
$ curl http://127.0.0.1:10080/pipeline -H 'Content-Type: application/json' -d '{"message": "Hello, Easegress"}'
Your Request
===============
Method: POST
URL   : /pipeline
Header: map[Accept:[*/*] Accept-Encoding:[gzip] Content-Type:[application/json] User-Agent:[curl/7.64.1] X-Adapt-Key:[goodplan]]
Body  : {"message": "Hello, Easegress"}
```

我们可以看到，除了原有的标头，Easegress 还向后台服务发送了标头 `X-Adapt-Key: goodplan`。


## 文档

更详细的文档请移步 [reference](./doc/reference.md) 和 [developer guide](./doc/developer-guide.md)。

## 路线图

请参考 [Easegress 路线图](./doc/Roadmap.md) 来了解详情。

## 社区

- [加入Slack工作区](https://join.slack.com/t/openmegaease/shared_invite/zt-upo7v306-lYPHvVwKnvwlqR0Zl2vveA)，提出需求、讨论问题、解决问题。
- [推特上的 MegaEase](https://twitter.com/megaease)


## 许可证

Easegress 采用 Apache 2.0 许可证。详见 [LICENSE](./LICENSE) 文件。
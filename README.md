# Easegress

- [Easegress](#easegress)
  - [What is Easegress](#what-is-easegress)
  - [Features](#features)
  - [Get Started](#get-started)
    - [Setting up Easegress](#setting-up-easegress)
    - [Create an HTTPServer and Pipeline](#create-an-httpserver-and-pipeline)
    - [Test](#test)
    - [More Filters](#more-filters)
  - [Documentation](#documentation)
  - [Roadmap](#roadmap)
  - [License](#license)

## What is Easegress

`Easegress` (formerly known as `EaseGateway`)is an all-rounder traffic orchestration system,  which is designed for:

- **High Availability:** Built-in Raft consensus & leader election makes 99.99% availability.
- **Traffic Orchestration:** Dynamically orchestrating various filters to a traffic pipeline.
- **High Performance:** Lightweight and essential features speed up the performance.
- **Observability:**  There are many meaningful statistics periodically in a readable way.
- **Extensibility:** It's easy to develop your own filter or controller with high-level programming language.
- **Integration:** The simple interfaces make it easy to integrate with other systems, such as Kubernetes Ingress, [EaseMesh](https://github.com/megaease/easemesh) sidecar, Workflow, etc.

The architecture of Easegress:

![architecture](./doc/architecture.png)

## Features

- **Service Management**
	- **Multiple protocols:**
		- HTTP/1.1
		- HTTP/2
		- HTTP/3(QUIC)
		- MQTT(coming soon)
	- **Rich Routing Rules:** exact path, path prefix, regular expression of the path, method, headers.
	- **Resilience&Fault Tolerance**
		- **Circuit breaker:** temporarily blocks possible failures.
		- **Rate limiter:** limits the rate of incoming requests.
		- **Retryer:** repeats failed executions.
		- **Time limiter:** limits the duration of execution.
	- **Deployment Management**
		- **Blue-green Strategy:** switches traffic at one time.
		- **Canary Strategy:** schedules traffic slightly.
	- **API Management**
		- **API Aggregation:** aggregates results of multiple APIs.
		- **API Orchestration:** orchestrates the flow of APIs.
	- **Security**
		- **IP Filter:** Limits access to IP addresses.
		- **Static HTTPS:** static certificate files.
		- **API Signature:** supports [HMAC](https://en.wikipedia.org/wiki/HMAC) verification.
		- **JWT Verification:** verifies [JWT Token](https://jwt.io/).
		- **OAuth2:** validates [OAuth/2](https://datatracker.ietf.org/doc/html/rfc6749) requests.
		- **Let's Encrypt:** automatically manage certificate files.
	- **Pipeline-Filter Mechanism**
		- **Chain of Responsibility Pattern:** orchestrates filters chain.
		- **Filter Management:** makes it easy to develop new filters.
	- **Service Mesh**
		- **Mesh Master:** is the control plane to manage the lifecycle of mesh services.
		- **Mesh Sidecar:** is the data plane as the endpoint to do traffic interception and routing.
		- **Mesh Ingress Controller:** is the mesh-specific ingress controller to route external traffic to mesh services.
	- **Third-Part Integration**
		- **FaaS** integrates with the serverless platform Knative.
		- **Service Discovery** integrates with Eureka, Consul, Etcd, and Zookeeper.
- **High Performance and Availability**
	- **Adaption**: adapts request, response in the handling chain.
	- **Validation**: headers validation, OAuth2, JWT, and HMAC verification.
	- **Load Balance:** round-robin, random, weighted random, ip hash, header hash.
	- **Cache:** for the backend servers.
	- **Compression:** compresses body for the response.
	- **Hot-Update:** updates both config and binary of Easegress in place without losing connections.
- **Operation**
	- **Easy to Integrate:** command line(`egctl`), MegaEase Portal, HTTP clients such as curl, postman, etc.
	- **Distributed Tracing**
		- Built-in  [Open Zipkin](https://zipkin.io/)
		- [Open Tracing](https://opentracing.io/) for vendor-neutral APIs
	- **Observability**
		- **Node:** role(leader, writer, reader), health or not, last heartbeat time, and so on
		- **Traffic:** in multi-dimension: server and backend.
			- **Throughput:** total and error statistics of request count, TPS/m1, m5, m15, and error percent, etc.
			- **Latency:** p25, p50, p75, p95, 98, p99, p999.
			- **Data Size:** request and response size.
			- **Status Codes:** HTTP status codes.
			- **TopN:** sorted by aggregated APIs(only in server dimension).

## Get Started

The basic common usage of Easegress is to quickly set up proxy for the backend servers. We split it into multiple simple steps to illustrate the essential concepts and operations.

### Setting up Easegress

We can download the binary from [release page](https://github.com/megaease/easegress/releases). For example we use linux version:

```bash
$ wget https://github.com/megaease/easegress/releases/download/v1.0.0/easegress_Linux_x86_64.tar.gz
$ tar zxvf easegress_Linux_x86_64.tar.gz -C easegress && cd easegress
```

or use source code:

```bash
$ git clone https://github.com/megaease/easegress && cd easegress
$ make
```

Then we can add the binary directory to the `PATH` and execute the server:

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

The default target of Makefile is to compile two binary into the directory `bin/`. `bin/easegress-server` is the server-side binary, `bin/egctl` is the client-side binary. We could add it to the `$PATH` for simplifying the following commands.

We could run `easegress-server` without specifying any arguments, which launch itself by opening default ports 2379, 2380, 2381. Of course, we can change them in the config file or command arguments that are explained well in `easegress-server --help`.

```bash
$ egctl member list
- options:
    name: eg-default-name
    labels: {}
    cluster-name: eg-cluster-default-name
    cluster-role: writer
    cluster-request-timeout: 10s
    cluster-client-url: http://localhost:2379
    cluster-peer-url: http://localhost:2380
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

After launched successfully, we could check the status of the one-node cluster. It shows the static options and dynamic status of heartbeat and etcd.

### Create an HTTPServer and Pipeline

Now let's create an HTTPServer listening on port 10080 to handle the HTTP traffic.

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

The rules of routers above mean that it will lead the traffic with the prefix `/pipeline` to the pipeline `pipeline-demo`, which will be created below. If we `curl` it before created, it will return 503.

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

The pipeline means it will do proxy for 3 backend endpoints in load balance policy `roundRobin`.

### Test

Now you can use some HTTP clients such as `curl` to test the feature:

```bash
$ curl -v http://127.0.0.1:10080/pipeline
```

If you are not set up some applications to handle the 9095, 9096, and 9097 in the localhost, it will return 503 too. We prepare a simple service to let us test handily, the example shows:

```bash
$ go run test/backend-service/mirror.go & # Running in background
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
Your Request
===============
Method: POST
URL   : /pipeline
Header: map[Accept:[*/*] Accept-Encoding:[gzip] Content-Type:[application/x-www-form-urlencoded] User-Agent:[curl/7.64.1]]
Body  : Hello, Easegress
```

### More Filters

Now we want to add more features to the pipeline, then we could add kinds of filters to the pipeline. For example, we want validation and request adaptation for the `pipeline-demo`.


<p align="center">
<img src="./doc/pipeline-demo.png" width=360 height=500>
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

After updating the pipeline, the original `curl -v http://127.0.0.1:10080/pipeline ` will get 400 because of the validating. So we changed it to satisfy the limitation:

```bash
$ curl http://127.0.0.1:10080/pipeline -H 'Content-Type: application/json' -d '{"message": "Hello, Easegress"}'
Your Request
===============
Method: POST
URL   : /pipeline
Header: map[Accept:[*/*] Accept-Encoding:[gzip] Content-Type:[application/json] User-Agent:[curl/7.64.1] X-Adapt-Key:[goodplan]]
Body  : {"message": "Hello, Easegress"}
```

We can also see Easegress send one more header `X-Adapt-Key: goodplan` to the mirror service.

## Documentation

See [reference](./doc/reference.md) and [developer guide](./doc/developer-guide.md) for more information.

## Roadmap 

See [Easegress Roadmap](./doc/Roadmap.md) for details.

## License

Easegress is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.

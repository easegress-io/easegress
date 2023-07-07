# Easegress

[![Go Report Card](https://goreportcard.com/badge/github.com/megaease/easegress)](https://goreportcard.com/report/github.com/megaease/easegress)
[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/megaease/easegress/test.yml?branch=main)](https://github.com/megaease/easegress/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/megaease/easegress/branch/main/graph/badge.svg?token=5Q80B98LPI)](https://codecov.io/gh/megaease/easegress)
[![Docker pulls](https://img.shields.io/docker/pulls/megaease/easegress.svg)](https://hub.docker.com/r/megaease/easegress)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/megaease/easegress)](https://github.com/megaease/easegress/blob/main/go.mod)
[![Join MegaEase Slack](https://img.shields.io/badge/slack-megaease-brightgreen?logo=slack)](https://join.slack.com/t/openmegaease/shared_invite/zt-upo7v306-lYPHvVwKnvwlqR0Zl2vveA)

<a href="https://megaease.com/easegress">
    <img src="./doc/imgs/easegress.svg"
        alt="Easegress logo" title="Easegress" height="175" width="175" align="right"/>
</a>

- [Easegress](#easegress)
  - [What is Easegress](#what-is-easegress)
  - [Features](#features)
  - [Use Cases](#use-cases)
  - [Getting Started](#getting-started)
    - [Setting up Easegress](#setting-up-easegress)
    - [Create an HTTPServer and Pipeline](#create-an-httpserver-and-pipeline)
    - [Test](#test)
    - [Add Another Pipeline](#add-another-pipeline)
    - [Update the HTTPServer](#update-the-httpserver)
    - [Test the RSS Pipeline](#test-the-rss-pipeline)
  - [Documentation](#documentation)
  - [Roadmap](#roadmap)
  - [Community](#community)
  - [Contributing](#contributing)
  - [License](#license)

## What is Easegress

`Easegress` is a Cloud Native traffic orchestration system designed for:

- **High Availability:** Built-in Raft consensus & leader election provides 99.99% availability.
- **Traffic Orchestration:** Simple orchestration of various filters for each traffic pipeline.
- **High Performance:** Lightweight and essential features speed up the performance.
- **Observability:** There are many meaningful statistics periodically in a readable way.
- **Extensibility:** It's easy to develop your own filter or controller with high-level programming language.
- **Integration:** The simple interfaces make it easy to integrate with other systems, such as Kubernetes Ingress, [EaseMesh](https://github.com/megaease/easemesh) sidecar, Workflow, etc.

The architecture of Easegress:

![architecture](./doc/imgs/architecture.png)

## Features

- **Service Management**
  - **Multiple protocols:**
    - HTTP/1.1
    - HTTP/2
    - HTTP/3(QUIC)
    - MQTT
  - **Rich Routing Rules:** exact path, path prefix, regular expression of the path, method, headers, clientIPs.
  - **Resilience&Fault Tolerance**
    - **CircuitBreaker:** temporarily blocks possible failures.
    - **RateLimiter:** limits the rate of incoming requests.
    - **Retry:** repeats failed executions.
    - **TimeLimiter:** limits the duration of execution.
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
    - **Filter Management:** makes it easy to develop new filters.
  - **Service Mesh**
    - **Mesh Master:** is the control plane to manage the lifecycle of mesh services.
    - **Mesh Sidecar:** is the data plane as the endpoint to do traffic interception and routing.
    - **Mesh Ingress Controller:** is the mesh-specific ingress controller to route external traffic to mesh services.
        > Notes: This feature is leveraged by [EaseMesh](https://github.com/megaease/easemesh)
  - **Third-Part Integration**
    - **FaaS** integrates with the serverless platform Knative.
    - **Service Discovery** integrates with Eureka, Consul, Etcd, and Zookeeper.
    - **Ingress Controller** integrates with Kubernetes as an ingress controller.
- **Extensibility**
  - **WebAssembly** executes user developed [WebAssembly](https://webassembly.org/) code.
- **High Performance and Availability**
  - **Adaption**: adapts request, response in the handling chain.
  - **Validation**: headers validation, OAuth2, JWT, and HMAC verification.
  - **Load Balance:** round-robin, random, weighted random, IP hash, header hash and support sticky sessions.
  - **Cache:** for the backend servers.
  - **Compression:** compresses body for the response.
  - **Hot-Update:** updates both config and binary of Easegress in place without losing connections.
- **Operation**
  - **Easy to Integrate:** command line([egctl](./doc/egctl-cheat-sheet.md)), MegaEase Portal, HTTP clients such as curl, postman, etc.
  - **Distributed Tracing**
    - Built-in [OpenTelemetry](https://opentelemetry.io/), which provides a vendor-neutral API.
  - **Observability**
    - **Node:** role(primary, secondary), raft leader status, healthy or not, last heartbeat time, and so on
    - **Traffic:** in multi-dimension: server and backend.
      - **Throughput:** total and error statistics of request count, TPS/m1, m5, m15, and error percent, etc.
      - **Latency:** p25, p50, p75, p95, p98, p99, p999.
      - **Data Size:** request and response size.
      - **Status Codes:** HTTP status codes.
      - **TopN:** sorted by aggregated APIs(only in server dimension).

## Use Cases

The following examples show how to use Easegress for different scenarios.

- [API Aggregation](./doc/cookbook/api-aggregation.md) - Aggregating many APIs into a single API.
- [Cluster Deployment](./doc/cookbook/multi-node-cluster.md) - How to deploy multiple Easegress cluster nodes.
- [Canary Release](./doc/cookbook/canary-release.md) - How to do canary release with Easegress.
- [Distributed Tracing](./doc/cookbook/distributed-tracing.md) - How to do APM tracing  - Zipkin.
- [FaaS](./doc/cookbook/faas.md) - Supporting Knative FaaS integration
- [Flash Sale](./doc/cookbook/flash-sale.md) - How to do high concurrent promotion sales with Easegress
- [Kubernetes Ingress Controller](./doc/cookbook/k8s-ingress-controller.md) - How to integrate with Kubernetes as ingress controller
- [LoadBalancer](./doc/cookbook/load-balancer.md) - A number of the strategies of load balancing
- [MQTTProxy](./doc/cookbook/mqtt-proxy.md) - An Example to MQTT proxy with Kafka backend.
- [Multiple API Orchestration](./doc/cookbook/translation-bot.md) - An Telegram translation bot.
- [Performance](./doc/cookbook/performance.md) - Performance optimization - compression, caching etc.
- [Pipeline](./doc/cookbook/pipeline.md) - How to orchestrate HTTP filters for requests/responses handling
- [Resilience and Fault Tolerance](./doc/cookbook/resilience.md) - CircuitBreaker, RateLimiter, Retry, TimeLimiter, etc. (Porting from [Java resilience4j](https://github.com/resilience4j/resilience4j))
- [Security](./doc/cookbook/security.md) - How to do authentication by Header, JWT, HMAC, OAuth2, etc.
- [Service Proxy](./doc/cookbook/service-proxy.md) - Supporting the Microservice registries - Zookeeper, Eureka, Consul, Nacos, etc.
- [WebAssembly](./doc/cookbook/wasm.md) - Using AssemblyScript to extend the Easegress
- [WebSocket](./doc/cookbook/websocket.md) - WebSocket proxy for Easegress
- [Workflow](./doc/cookbook/workflow.md) - An Example to make a workflow for a number of APIs.

For full list, see [Cookbook](./doc/README.md#1-cookbook--how-to-guide).

## Getting Started

The basic usage of Easegress is to quickly set up a proxy for the backend
servers. In this section, we will first set up a reverse proxy, and then
demonstrate the API orchestration feature by including more components in the
configuration, we will also show the essential concepts and operations of
Easegress.

### Setting up Easegress

We can download the latest or history binaries from the
[release page](https://github.com/megaease/easegress/releases). The following
shell script will:

- Download and extract the latest binaries to `./easegress` folder
- Install the Easegress Systemd service.

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/megaease/easegress/main/scripts/install.sh)"
```

or we can install Easegress from source code:

```bash
git clone https://github.com/megaease/easegress && cd easegress
make
```

> **Note**:
>
> - This repo requires Go 1.19+ compiler for the build.
> - If you need the WebAssembly feature, please run `make wasm`.

Then we can add the binary directory to the `PATH` and execute the server:

```bash
$ export PATH=${PATH}:$(pwd)/bin/
$ easegress-server
2022-07-04T13:47:36.579+08:00   INFO    cluster/config.go:106   etcd config: advertise-client-urls: [{Scheme:http Opaque: User: Host:localhost:2379 Path: RawPath: ForceQuery:false RawQuery: Fragment: RawFragment:}] advertise-peer-urls: [{Scheme:http Opaque: User: Host:localhost:2380 Path: RawPath: ForceQuery:false RawQuery: Fragment: RawFragment:}] init-cluster: eg-default-name=http://localhost:2380 cluster-state: new force-new-cluster: false
2022-07-04T13:47:37.516+08:00   INFO    cluster/cluster.go:332  client connect with endpoints: [http://localhost:2380]
2022-07-04T13:47:37.521+08:00   INFO    cluster/cluster.go:346  client is ready
2022-07-04T13:47:37.529+08:00   INFO    cluster/cluster.go:638  server is ready
2022-07-04T13:47:37.534+08:00   INFO    cluster/cluster.go:498  lease is ready (grant new one: b6a81c7bffb1a07)
2022-07-04T13:47:37.534+08:00   INFO    cluster/cluster.go:218  cluster is ready
2022-07-04T13:47:37.541+08:00   INFO    supervisor/supervisor.go:137    create TrafficController
2022-07-04T13:47:37.542+08:00   INFO    supervisor/supervisor.go:137    create RawConfigTrafficController
2022-07-04T13:47:37.544+08:00   INFO    supervisor/supervisor.go:137    create ServiceRegistry
2022-07-04T13:47:37.544+08:00   INFO    supervisor/supervisor.go:137    create StatusSyncController
2022-07-04T13:47:37.544+08:00   INFO    statussynccontroller/statussynccontroller.go:139        StatusUpdateMaxBatchSize is 20
2022-07-04T13:47:37.544+08:00   INFO    cluster/cluster.go:538  session is ready
2022-07-04T13:47:37.545+08:00   INFO    api/api.go:73   register api group admin
2022-07-04T13:47:37.545+08:00   INFO    api/server.go:86        api server running in localhost:2381
```

The default target of Makefile is to compile two binary into the `bin`
directory. `bin/easegress-server` is the server-side binary, `bin/egctl` is the
client-side binary. We could add it to the `$PATH` to simplify the following
commands.

We could run `easegress-server` without specifying any arguments, which launch
itself by opening default ports 2379, 2380, and 2381. We can change them in the
configuration file or command-line arguments that are explained well in
`easegress-server --help`.

```bash
$ egctl get member
NAME                    ROLE            AGE     STATE   API-ADDR        HEARTBEAT
eg-default-name         primary         9s      Leader  localhost:2381  3s ago

$ egctl describe member
Name: eg-default-name
LastHeartbeatTime: "2023-07-03T17:39:30+08:00"

Etcd:
=====
  id: 689e371e88f78b6a
  startTime: "2023-07-03T17:39:14+08:00"
  state: Leader
...
```

After launching successfully, we could check the status of the one-node
cluster. It shows the static options and dynamic status of heartbeat and etcd.

### Create an HTTPServer and Pipeline

Now let's create an HTTPServer listening on port 10080 to handle the HTTP
traffic.

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
      backend: pipeline-demo' | egctl create -f -
```

The rules above mean it will forward the traffic with the prefix `/pipeline` to
the `pipeline-demo` pipeline because the pipeline hasn't been created yet,
we will get 503 if we `curl` it now. Next, let's create the pipeline.

```bash
$ echo '
name: pipeline-demo
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin' | egctl create -f -
```

The pipeline means it will forward traffic to 3 backend endpoints, using the
`roundRobin` load balance policy.

Additionally, we provide a [dashboard](https://cloud.megaease.com) that
streamlines the aforementioned steps, this intuitive tool can help you create,
manage HTTPServers, Pipelines and other Easegress configuration.

![HTTP Server](doc/imgs/readme-httpserver.png)
![Pipeline](doc/imgs/readme-pipeline.png)

### Test

Now you can use an HTTP clients, such as `curl`, to test the feature:

```bash
curl -v http://127.0.0.1:10080/pipeline
```

If you haven't set up backend services on ports 9095, 9096, and 9097 of the
localhost, it returns 503 too. We provide a simple service for this:

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

### Add Another Pipeline

Now let's add another pipeline, it will get the address of an RSS feed from the
request, read the RSS feed, build the article list into a Slack message, and
then send it to Slack. But before creating the pipeline, please follow [this
document](https://api.slack.com/messaging/webhooks) to create your own Slack
webhook URL and replace the one in the below command with it.

<p align="center">
  <img src="./doc/imgs/rss-pipeline.png" width=480>
</p>

```bash
$ echo '
name: rss-pipeline
kind: Pipeline

flow:
- filter: validator
- filter: buildRssRequest
  namespace: rss
- filter: sendRssRequest
  namespace: rss
- filter: decompressResponse
  namespace: rss
- filter: buildSlackRequest
  namespace: slack
- filter: sendSlackRequest
  namespace: slack
- filter: buildResponse

filters:
- name: validator
  kind: Validator
  headers:
    "X-Rss-Url":
       regexp: ^https?://.+$

- name: buildRssRequest
  kind: RequestBuilder
  template: |
    url: /developers/feed2json/convert?url={{index (index .requests.DEFAULT.Header "X-Rss-Url") 0 | urlquery}}

- name: sendRssRequest
  kind: Proxy
  pools:
  - loadBalance:
      policy: roundRobin
    servers:
    - url: https://www.toptal.com
  compression:
    minLength: 4096

- name: buildSlackRequest
  kind: RequestBuilder
  template: |
    method: POST
    url: /services/T0XXXXXXXXX/B0YYYYYYY/ZZZZZZZZZZZZZZZZZZZZ   # This the Slack webhook address, please change it to your own.
    body: |
      {
         "text": "Recent posts - {{.responses.rss.JSONBody.title}}",
         "blocks": [{
            "type": "section",
            "text": {
              "type": "plain_text",
              "text": "Recent posts - {{.responses.rss.JSONBody.title}}"
            }
         }, {
            "type": "section",
            "text": {
              "type": "mrkdwn",
              "text": "{{range $index, $item := .responses.rss.JSONBody.items}}• <{{$item.url}}|{{$item.title}}>\n{{end}}"
         }}]
      }

- name: sendSlackRequest
  kind: Proxy
  pools:
  - loadBalance:
      policy: roundRobin
    servers:
    - url: https://hooks.slack.com
  compression:
    minLength: 4096

- name: decompressResponse
  kind: ResponseAdaptor
  decompress: gzip

- name: buildResponse
  kind: ResponseBuilder
  template: |
    statusCode: 200
    body: RSS feed has been sent to Slack successfully.' | egctl create -f -
```

### Update the HTTPServer

Now let's update the HTTPServer to forward the traffic with prefix `/rss` to
the new pipeline.

```bash
$ echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /rss          # +
      backend: rss-pipeline     # +
    - pathPrefix: /pipeline
      backend: pipeline-demo' | egctl apply -f -
```

### Test the RSS Pipeline

Execute the below command, your slack will receive the article list of the RSS
feed.

```bash
curl -H X-Rss-Url:https://hnrss.org/newest?count=5 http://127.0.0.1:10080/rss
```

Please note the maximum message size Slack allowed is about 3K, so you will
need to limit the number of articles returned by the RSS feed of some
sites(e.g. Hack News).

## Documentation

See [Easegress Documentation](./doc/README.md) for all documents.

## Roadmap

See [Easegress Roadmap](./doc/Roadmap.md) for details.

## Community

- [Join Slack Workspace](https://join.slack.com/t/openmegaease/shared_invite/zt-upo7v306-lYPHvVwKnvwlqR0Zl2vveA) for requirement, issue and development.
- [MegaEase on Twitter](https://twitter.com/megaease)

## Contributing

See [Contributing guide](./CONTRIBUTING.md#contributing).

## License

Easegress is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.

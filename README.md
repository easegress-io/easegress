
# EaseGateway [![Build Status](https://travis-ci.com/megaease/easegateway.svg?token=bgrenfvQpzZ8JoKbi6uX&branch=master)](https://travis-ci.com/megaease/easegateway) [![codecov](https://codecov.io/gh/megaease/easegateway/branch/master/graph/badge.svg?token=HAR3ZmYoQG)](https://codecov.io/gh/megaease/easegateway)

## What is EaseGateway

`EaseGateway` is an all-rounder gateway system,  which is designed:

- **Built-in Distributed Storage:** It uses [etcd](https://github.com/etcd-io/etcd)  as a built-in storage system without external storage, which guarantees the sync of config, statistics, and all kinds of data.
- **Traffic Scheduling:** The pipeline and filter mechanism can simply schedule the traffic.
- **High Performance:** Lightweight and essential features speed up the performance.
- **Observability:**  There are many meaningful statistics periodically in a readable way.
- **Easy to Develop:** The simple interfaces make it easy to develop new stuff, the  [EaseMesh](https://github.com/megaease/easemesh) is our real example.

We also prepare more detail in other documentations: [quick start](./doc/quick-start.md) and [reference](./doc/reference.md) and [develop guide](./doc/develop-guide.md).

The architecture of EaseGateway:

![architecture](./doc/architecture.png)

## Features

- **Service Management**
	- **Multiple protocol:**
		- HTTP/1.1
		- HTTP/2
		- HTTP/3(QUIC)
		- MQTT(coming soon)
	- **Rich Routing Rules:** exact path, path prefix, regular expression of path, method, headers.
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
		- **API Signature:** supports **HMAC** verification.
		- **JWT Verification:** verifies [JWT Token](https://jwt.io/).
		- **OAuth2:** validates OAuth2 requests.
		- **Let's Encrypt:** automatically manage certificate files.
	- **Pipeline-Filter Mechanism**
		- **Chain of Responsibility Pattern:** orchestrates filters chain.
		- **Filter Management:** makes it easy to develop new filters.
	- **Service Mesh**
		- **Mesh Aaster:** is the control plane to manage the lifecycle of mesh services.
		- **Mesh Sidecar:** is the data plane as the endpoint to do traffic interception and routing.
		- **Mesh Ingress Controller:** is the mesh-specific ingress controller to route external traffic to mesh services.
	- **Third-Part Integration**
		- **FaaS** integrates with the serverless platform Knative.
		- **Service Discovery** integrates with Eureka, Consul, Etcd, and Zookeeper.
- **High Performance and Availability**
	- **Adaption**: adapts request, response in the handling chain.
	- **Validation**: headers validation, Oauth2, JWT, and HMAC verifying.
	- **Load Balance:** round-robin, random, weighted random, ip hash, header hash.
	- **Cache:** for the backend servers.
	- **Compression:** compresses body for the response.
	- **Hot-Update:** updates both config and binary of EaseGateway in place without losing connections.
- **Operation**
	- **Easy to Integrate:** command line(egctl), MegaEase Portal, HTTP clients such as curl, postman, etc.
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

## Build and Quick Run

Requirements: Go version >= 1.13.

```shell

# Makefile generates binaray `easegateway-server` and `egctl` at `bin/`.
$ make
$ ./bin/easegateway-server
# Check member status
$ ./bin/egctl member list
```

Next step you could check out [quick start](./doc/quick-start.md) to give it an easy shot.

## License

EaseGateway is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.
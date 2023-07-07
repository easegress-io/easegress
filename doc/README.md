# Easegress Documentation

- [Easegress Documentation](#easegress-documentation)
	- [1. Cookbook / How-To Guide](#1-cookbook--how-to-guide)
	- [2. Development Guide](#2-development-guide)
	- [3. Performance](#3-performance)
	- [4. Reference Guide](#4-reference-guide)
		- [4.1 Controllers](#41-controllers)
			- [4.1.1 System Controllers](#411-system-controllers)
			- [4.1.2 Business Controllers](#412-business-controllers)
		- [4.2 Filters](#42-filters)
		- [4.3 Custom Data](#43-custom-data)
	- [5. egctl Cheat Sheet](#5-egctl-cheat-sheet)

## 1. Cookbook / How-To Guide

This is a cookbook that lists a number of useful and practical examples on how to use Easegress for different scenarios.

- [API Aggregator](./cookbook/api-aggregation.md) - Aggregating many APIs into a single API.
- [Cluster Deployment](./cookbook/multi-node-cluster.md) - How to deploy multiple Easegress cluster nodes.
- [Canary Release](./cookbook/canary-release.md) - How to do canary release with Easegress.
- [Distributed Tracing](./cookbook/distributed-tracing.md) - How to do APM tracing  - Zipkin.
- [FaaS](./cookbook/faas.md) - Supporting Knative FaaS integration
- [Flash Sale](./cookbook/flash-sale.md) - How to do high concurrent promotion sales with Easegress
- [Kubernetes Ingress Controller](./cookbook/k8s-ingress-controller.md) - How to integrated with Kubernetes as ingress controller, and [K8s Ingress Controller](./reference/ingresscontroller.md) for full manual.
- [LoadBalancer](./cookbook/load-balancer.md) - A number of strategy of load balancing
- [Migrate v1.x Filter To v2.x](./cookbook/migrate-v1-filter-to-v2.md) - How to migrate a v1.x filter to v2.x.
- [MQTTProxy](./cookbook/mqtt-proxy.md) - An Example to MQTT proxy with Kafka backend.
- [Multiple API Orchestration](./cookbook/translation-bot.md) - An Telegram translation bot.
- [Performance](./cookbook/performance.md) - Performance optimization - compression, caching etc.
- [Pipeline](./cookbook/pipeline.md) - How to orchestrate HTTP filters for requests/responses handling
- [Resilience and Fault Tolerance](./cookbook/resilience.md) - CircuitBreaker, RateLimiter, Retry, TimeLimiter, etc. (Porting from [Java resilience4j](https://github.com/resilience4j/resilience4j))
- [Security](./cookbook/security.md) - How to do authentication by Header, JWT, HMAC, OAuth2, etc.
- [Service Proxy](./cookbook/service-proxy.md) - Supporting the Microservice  registries - Zookeeper, Eureka, Consul, Nacos, etc.
- [WebAssembly](./cookbook/wasm.md) - Using AssemblyScript to extend the Easegress
- [WebSocket](./cookbook/websocket.md) - WebSocket proxy for Easegress
- [Workflow](./cookbook/workflow.md) - An Example to make a workflow for a number of APIs.

## 2. Development Guide

- [Easegress Roadmap](./Roadmap.md) - The development roadmap of Easegress.
- [Developer Guide](./developer-guide.md) - A guide help to develop the Easegress.
- [Source Code Introduction](./slides/Easegress.code.pptx) - A slide introduces source code (Dec. 2021 - Easegress v1.4.0)

## 3. Performance

- [Benchmark](./reference/benchmark.md) - Performance Test Report.
- [Linux Kernel Tuning](./reference/kernel-tuning.md) - Tuning the Linux Kernel to make the Easegress run faster.

## 4. Reference Guide

### 4.1 Controllers

The Easegress controller is the core entity to control kinds of working. There are two kinds of controllers - system and business. 

For the full document, please check - [Controller Reference](./reference/controllers.md)

#### 4.1.1 System Controllers

The following controllers are system level controllers.  One and only one instance of them are created in every Easegress node and they can't be deleted. 

- [ServiceRegistry](./reference/controllers.md#serviceregistry) - The service hub for all service registries - Consul, Etcd, Eureka, Zookeeper, Nacos...
- [TrafficController](./reference/controllers.md#trafficcontroller) - TrafficController handles the lifecycle of TrafficGates(HTTPServer and etc.) and Pipeline and their relationship. 
- [RawConfigTrafficController](./reference/controllers.md#rawconfigtrafficcontroller) - RawConfigTrafficController maps all traffic static configurations to TrafficController in the namespace `default`.

#### 4.1.2 Business Controllers

It could be created, updated, deleted by admin operation. They control various resources such as mesh traffic, service discovery, faas, and so on.

- [EaseMonitorMetrics](./reference/controllers.md#) - Monitor metrics of Easegress and send them to Kafka.
- [FaaSController](./reference/controllers.md#faascontroller) - For Easegress and FaaS products integration purpose.
- [IngressController](./reference/controllers.md#ingresscontroller) - an implementation of Kubernetes ingress controller, it watches Kubernetes Ingress, Service, Endpoints, and Secrets then translates them to Easegress HTTP server and pipelines. The [K8s Ingress Controller](./reference/ingresscontroller.md) for full manual.
- [MeshController](./reference/controllers.md#meshcontroller) - This is for [EaseMesh](https://github.com/megaease/easemesh) project.
- [ConsulServiceRegistry](./reference/controllers.md#consulserviceregistry) - supports service discovery for Consul as backend. 
- [EtcdServiceRegistry](./reference/controllers.md#etcdserviceregistry) - support service discovery for Etcd as backend. 
- [EurekaServiceRegistry](./reference/controllers.md#eurekaserviceregistry) - supports service discovery for Eureka as backend. 
- [ZookeeperServiceRegistry](./reference/controllers.md#zookeeperserviceregistry) -  supports service discovery for Zookeeper as backend. 
- [NacosServiceRegistry](./reference/controllers.md#nacosserviceregistry) - supports service discovery for Nacos as backend.
- [AutoCertManager](./reference/controllers.md#autocertmanager) - automatically manage HTTPS certificates. 

### 4.2 Filters

- [Proxy](./reference/filters.md#Proxy) - The Proxy filter is a proxy of backend service. 
- [CORSAdaptor](./reference/filters.md#CORSAdaptor) - The CORSAdaptor handles the CORS preflight request for backend service.
- [Fallback](./reference/filters.md#Fallback) - The Fallback filter mocks a response as fallback action of other filters. 
- [Mock](./reference/filters.md#Mock) - The Mock filter mocks responses according to configured rules, mainly for testing purposes.
- [RemoteFilter](./reference/filters.md#RemoteFilter) - The RemoteFilter is a filter making remote service acting as an internal filter. 
- [RateLimiter](./reference/filters.md#RateLimiter) - The RateLimiter protects backend service for high availability and reliability by limiting the number of requests sent to the service in a configured duration.
- [RequestBuilder](./reference/filters.md#RequestBuilder) - The RequestBuilder build a new request from existing requests/responses.
- [ResponseBuilder](./reference/filters.md#ResponseBuilder) - The ResponseBuilder build a new response from existing requests/responses.
- [RequestAdaptor](./reference/filters.md#RequestAdaptor) - The RequestAdaptor modifies the original request according to configuration.
- [ResponseAdaptor](./reference/filters.md#ResponseAdaptor) - The ResponseAdaptor modifies the original response according to the configuration before passing it back.
- [Validator](./reference/filters.md#Validator) - The Validator filter validates requests, forwards valid ones, and rejects invalid ones. 
- [WasmHost](./reference/filters.md#WasmHost) - The WasmHost filter implements a host environment for user-developed WebAssembly code. 

### 4.3 Custom Data

- [Custom Data Management](./reference/customdata.md) - Create/Read/Update/Delete custom data kinds and custom data items.

# 5. egctl Cheat Sheet
[egctl Cheat Sheet](./egctl-cheat-sheet.md) contains common used `egctl` commands and details about `.egctlrc`.

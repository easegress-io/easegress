# Controllers

- [Controllers](#controllers)
  - [System Controllers](#system-controllers)
    - [ServiceRegistry](#serviceregistry)
    - [TrafficController](#trafficcontroller)
    - [RawConfigTrafficController](#rawconfigtrafficcontroller)
      - [HTTPServer](#httpserver)
      - [HTTPPipeline](#httppipeline)
    - [StatusSyncController](#statussynccontroller)
  - [Business Controllers](#business-controllers)
    - [EaseMonitorMetrics](#easemonitormetrics)
    - [Function](#function)
    - [IngressController](#ingresscontroller)
    - [MeshController](#meshcontroller)
    - [ConsulServiceRegistry](#consulserviceregistry)
    - [EtcdServiceRegistry](#etcdserviceregistry)
    - [EurekaServiceRegistry](#eurekaserviceregistry)
    - [ZookeeperServiceRegistry](#zookeeperserviceregistry)
    - [NacosServiceRegistry](#nacosserviceregistry)
  - [Common Types](#common-types)
    - [tracing.Spec](#tracingspec)
    - [zipkin.Spec](#zipkinspec)
    - [ipfilter.Spec](#ipfilterspec)
    - [httpserver.Rule](#httpserverrule)
    - [httpserver.Path](#httpserverpath)
    - [httpserver.Header](#httpserverheader)
    - [httppipeline.Flow](#httppipelineflow)
    - [httppipeline.Filter](#httppipelinefilter)
    - [easemonitormetrics.Kafka](#easemonitormetricskafka)
    - [nacos.ServerSpec](#nacosserverspec)

As the [architecture diagram](./architecture.png) shows, the controller is the core entity to control kinds of working. There are two kinds of controllers overall:

- System Controller: It is created one and only one instance in every Easegress node, which can't be deleted. They mainly aim to control essential system-level stuff.
- Business Controller: It could be created, updated, deleted by admin operation. They control various resources such as mesh traffic, service discovery, faas, and so on.

In another view, Easegress as a traffic orchestration system, we could classify them into traffic controller and non-traffic controller:

- Traffic Controller: It invokes TrafficController to handle its specific traffic, such as MeshController.
- Non-Traffic Controller: It doesn't handle business traffic, such as EurekaServiceRegistry, even though it has admin traffic with Eureka.

The two categories are conceptual, which means they are not strict distinctions. We just use them as terms to clarify controllers technically.

## System Controllers

For now, all system controllers can not be configured. It may gain this capability if necessary in the future.

### ServiceRegistry

We use the system controller `ServiceRegistry` as the service hub for all service registries. Current drivers are

- [ConsulServiceRegistry](#consulserviceregistry)
- [EtcdServiceRegistry](#etcdserviceregistry)
- [EurekaServiceRegistry](#eurekaserviceregistry)
- [ZookeeperServiceRegistry](#zookeeperserviceregistry)
- [NacosServiceRegistry](#nacosserviceregistry)

The drivers need to offer notifying change periodically, and operations to the external service registry.

### TrafficController

TrafficController handles the lifecycle of HTTPServer and HTTPPipeline and their relationship. It manages the resource in a namespaced way. HTTPServer accepts incoming traffic and routes it to HTTPPipelines in the same namespace. Most other controllers could handle traffic by leverage the ability of TrafficController..

### RawConfigTrafficController

RawConfigTrafficController maps all traffic static configurations to TrafficController in the namespace `default`. We could use `egctl` to manage the configuration of servers and pipelines in the default namespace.

#### HTTPServer

HTTPServer is a server that listens on one port to route all traffic to available pipelines. Its simplest config looks like:

```yaml
kind: HTTPServer
name: http-server-example
port: 80
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: http-pipeline-example
```

| Name             | Type                               | Description                                                                              | Required             |
| ---------------- | ---------------------------------- | ---------------------------------------------------------------------------------------- | -------------------- |
| http3            | bool                               | Whether to support HTTP3(QUIC)                                                           | No                   |
| port             | uint16                             | The HTTP port listening on                                                               | Yes                  |
| keepAlive        | bool                               | Whether to support keepalive                                                             | Yes (default: false) |
| keepAliveTimeout | string                             | The timeout of keepalive                                                                 | Yes (default: 60s)   |
| maxConnections   | uint32                             | The max connections with clients                                                         | Yes (default: 10240) |
| https            | bool                               | Whether to use HTTPS                                                                     | Yes (default: false) |
| cacheSize        | uint32                             | The size of cache, 0 means no cache                                                      | No                   |
| xForwardedFor    | bool                               | Whether to set X-Forwarded-For header by own ip                                          | No                   |
| tracing          | [tracing.Spec](#tracingSpec)       | Distributed tracing settings                                                             | No                   |
| certBaset64      | string                             | Public key of PEM encoded data in base64 encoded format                                  | No                   |
| keyBase64        | string                             | Private key of PEM encoded data in base64 encoded format                                 | No                   |
| certs            | map[string]string                  | Public keys of PEM encoded data, the key is the logic pair name, which must match keys   | No                   |
| keys             | map[string]string                  | Private keys of PEM encoded data, the key is the logic pair name, which must match certs | No                   |
| ipFilter         | [ipfilter.Spec](#ipfilterSpec)     | IP Filter for all traffic under the server                                               | No                   |
| rules            | [httpserver.Rule](#httpserverRule) | Router rules                                                                             | No                   |

#### HTTPPipeline

HTTPPipeline uses the Chain of Responsibility pattern to orchestrate filters. Its simplest config looks like:

```yaml
name: http-pipeline-example
kind: HTTPPipeline

# Built-in labels are `END` which can't be used by filters.
flow:
  - filter: proxy
    jumpIf: { fallback: END }
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin
```

| Name    | Type                                         | Description                          | Required |
| ------- | -------------------------------------------- | ------------------------------------ | -------- |
| flow    | [httppipeline.Flow](#httppipelineFlow)       | Flow of http pipeline                | No       |
| Filters | [][httppipeline.Filter](#httppipelineFilter) | Filters definitions of http pipeline | Yes      |

### StatusSyncController

No config.

## Business Controllers

### EaseMonitorMetrics

EaseMonitorMetrics is adapted to monitor metrics of Easegress and send them to Kafka. The config looks like:

```yaml
kind: EaseMonitorMetrics
name: easemonitor-metrics-example
kafka:
  brokers: ["127.0.0.1:9092"]
  topic: metrics
```

| Name  | Type                                                 | Description          | Required |
| ----- | ---------------------------------------------------- | -------------------- | -------- |
| kafka | [easemonitormetrics.Kafka](#easemonitormetricsKafka) | Kafka related config | Yes      |

### Function

TODO (@ben)

### IngressController

The IngressController is an implementation of [Kubernetes ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/), it watches Kubernetes Ingress, Service, Endpoints, and Secrets then translates them to Easegress HTTP server and pipelines. The config looks like:

```yaml
kind: IngressController
name: ingress-controller-example
kubeConfig:
masterURL:
namespaces: ["default"]
ingressClass: easegress
httpServer:
  port: 8080
  https: false
  keepAlive: true
  keepAliveTimeout: 60s
  maxConnections: 10240
```

| Name         | Type                           | Description                                                                                                                                                               | Required                |
| ------------ | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| kubeConfig   | string                         | Path of the Kubernetes configuration file.                                                                                                                                | No                      |
| masterURL    | string                         | The address of the Kubernetes API server.                                                                                                                                 | No                      |
| namespaces   | []string                       | An array of Kubernetes namespaces which the IngressController needs to watch, all namespaces are watched if left empty.                                                   | No                      |
| ingressClass | string                         | The IngressController only handles `Ingresses` with `ingressClassName` set to the value of this option.                                                                   | No (default: easegress) |
| httpServer   | [httpserver.Spec](#httpserver) | Basic configuration for the shared HTTP traffic gate. The routing rules will be generated dynamically according to Kubernetes ingresses and should not be specified here. | Yes                     |

**Note**: IngressController uses `kubeConfig` and `masterURL` to connect to Kubernetes, at least one of them must be specified when deployed outside of a Kubernetes cluster, and both are optional when deployed inside a cluster.

### MeshController

MeshController contains the ingress controller (note this ingress controller is for mesh deployment, not the [Kubernetes ingress controller](#ingresscontroller) described above), master(control plane), worker/sidecar(data plane). The config looks like:

```yaml
name: mesh-controller-example
kind: MeshController
specUpdateInterval: 10s
heartbeatInterval: 5s
registryType: consul
serviceName: service-001
externalServiceRegistry: consul-service-registry-example
```

| Name                    | Type   | Description                                                               | Required              |
| ----------------------- | ------ | ------------------------------------------------------------------------- | --------------------- |
| heartbeatInterval       | string | Interval for one service instance reporting its heartbeat                 | Yes (default: 5s)     |
| registryType            | string | Protocol the registry center accepts, support `eureka`, `consul`, `nacos` | Yes (default: eureka) |
| apiPort                 | int    | Port listening on for worker's API server                                 | Yes (default: 13009)  |
| ingressPort             | int    | Port listening on for for ingress traffic                                 | Yes (default: 13010)  |
| externalServiceRegistry | string | External service registry name                                            | No                    |

### ConsulServiceRegistry

ConsulServiceRegistry supports service discovery for Consul as backend. The config looks like:

```yaml
kind: ConsulServiceRegistry
name: consul-service-registry-example
address: '127.0.0.1:8500'
scheme: http
syncInterval: 10s
```

| Name         | Type     | Description                  | Required                      |
| ------------ | -------- | ---------------------------- | ----------------------------- |
| address      | string   | Consul server address        | Yes (default: 127.0.0.1:8500) |
| scheme       | string   | Communication scheme         | Yes (default: http)           |
| datacenter   | string   | Datacenter name              | No                            |
| token        | string   | ACL token for communication  | No                            |
| Namespace    | string   | Namespace to use             | No                            |
| syncInterval | string   | Interval to synchronize data | Yes (default: 10s)            |
| serviceTags  | []string | Service tags to query        | No                            |

### EtcdServiceRegistry

EtcdServiceRegistry support service discovery for Etcd as backend. The config looks like:

```yaml
kind: EtcdServiceRegistry
name: etcd-service-registry-example
endpoints: ['127.0.0.1:12379']
prefix: "/services/"
cacheTimeout: 10s
```

| Name         | Type     | Description                    | Required                  |
| ------------ | -------- | ------------------------------ | ------------------------- |
| endpoints    | []string | Endpoints of Etcd servers      | Yes                       |
| prefix       | string   | Prefix of the keys of services | Yes (default: /services/) |
| cacheTimeout | string   | Timeout of cache               | Yes (default: 60s)        |

### EurekaServiceRegistry

EurekaServiceRegistry supports service discovery for Eureka as backend. The config looks like:

```yaml
kind: EurekaServiceRegistry
name: eureka-service-registry-example
endpoints: ['http://127.0.0.1:8761/eureka']
syncInterval: 10s
```

| Name         | Type     | Description                  | Required                                    |
| ------------ | -------- | ---------------------------- | ------------------------------------------- |
| endpoints    | []string | Endpoints of Eureka servers  | Yes (default: http://127.0.0.1:8761/eureka) |
| syncInterval | string   | Interval to synchronize data | Yes (default: 10s)                          |

### ZookeeperServiceRegistry

ZookeeperServiceRegistry supports service discovery for Zookeeper as backend. The config looks like:

```yaml
kind: ZookeeperServiceRegistry
name: zookeeper-service-registry-example
zkservices: [127.0.0.1:2181]
prefix: /services
conntimeout: 6s
syncInterval: 10s
```

| Name         | Type     | Description                  | Required                      |
| ------------ | -------- | ---------------------------- | ----------------------------- |
| zkservices   | []string | Zookeeper service addresses  | Yes (default: 127.0.0.1:2181) |
| connTimeout  | string   | Timeout of connection        | Yes (default: 6s)             |
| Prefix       | string   | Prefix of services           | Yes (default: /)              |
| syncInterval | string   | Interval to synchronize data | Yes (default: 10s)            |

### NacosServiceRegistry

NacosServiceRegistry supports service discovery for Nacos as backend. The config looks like:

```yaml
kind: NacosServiceRegistry
name: nacos-service-registry-example
syncInterval: 10s
servers:
  - scheme: http
    port: 8848
    contextPath: /nacos
    ipAddr: 127.0.0.1
```

| Name         | Type                                  | Description                  | Required           |
| ------------ | ------------------------------------- | ---------------------------- | ------------------ |
| servers      | [][nacosServerSpec](#nacosserverspec) | Servers of Nacos             | Yes                |
| syncInterval | string                                | Interval to synchronize data | Yes (default: 10s) |
| namespace    | string                                | The namespace of Nacos       | No                 |
| username     | string                                | The username of client       | No                 |
| password     | string                                | The password of client       | No                 |

## Common Types

### tracing.Spec

| Name        | Type                       | Description                   | Required |
| ----------- | -------------------------- | ----------------------------- | -------- |
| serviceName | string                     | The service name of top level | Yes      |
| Zipkin      | [zipkin.Spec](#zipkinSpec) | The tracing spec of zipkin    | No       |

### zipkin.Spec

| Name       | Type    | Description                                                                                        | Required |
| ---------- | ------- | -------------------------------------------------------------------------------------------------- | -------- |
| hostPort   | string  | The host:port of the service                                                                       | No       |
| serverURL  | string  | The zipkin server URL                                                                              | Yes      |
| sampleRate | float64 | The sample rate for collecting metrics, the range is [0, 1]                                        | Yes      |
| sameSpan   | bool    | Whether to allow to place client-side and server-side annotations for an RPC call in the same span | No       |
| id128Bit   | bool    | Whether to start traces with 128-bit trace id                                                      | No       |

### ipfilter.Spec

| Name           | Type     | Description                                          | Required             |
| -------------- | -------- | ---------------------------------------------------- | -------------------- |
| blockByDefault | bool     | Set block is the default action if not matching      | Yes (default: false) |
| allowIPs       | []string | IPs to be allowed to pass (support IPv4, IPv6, CIDR) | No                   |
| blockIPs       | []string | IPs to be blocked to pass (support IPv4, IPv6, CIDR) | No                   |

### httpserver.Rule

| Name       | Type                               | Description                                                   | Required |
| ---------- | ---------------------------------- | ------------------------------------------------------------- | -------- |
| ipFilter   | [ipfilter.Spec](#ipfilterSpec)     | IP Filter for all traffic under the rule                      | No       |
| host       | string                             | Exact host to match, empty means to match all                 | No       |
| hostRegexp | string                             | Host in regular expression to match, empty means to match all | No       |
| paths      | [httpserver.Path](#httpserverPath) | Path matching rules, empty means to match nothing             | No       |

### httpserver.Path

| Name          | Type                                     | Description                                                                                                                            | Required |
| ------------- | ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| ipFilter      | [ipfilter.Spec](#ipfilterSpec)           | IP Filter for all traffic under the path                                                                                               | No       |
| path          | string                                   | Exact path to match                                                                                                                    | No       |
| pathPrefix    | string                                   | Prefix of the path to match                                                                                                            | No       |
| pathRegexp    | string                                   | Path in regular expression to match                                                                                                    | No       |
| rewriteTarget | string                                   | Use pathRegexp.[ReplaceAllString](https://golang.org/pkg/regexp/#Regexp.ReplaceAllString)(path, rewriteTarget) to rewrite request path | No       |
| methods       | []string                                 | Methods to match, empty means to allow all methods                                                                                     | No       |
| headers       | [][httpserver.Header](#httpserverHeader) | Headers to match (the requests matching headers won't be put into cache)                                                               | No       |
| backend       | string                                   | backend name (pipeline name in static config, service name in mesh)                                                                    | Yes      |

### httpserver.Header

There must be at least one of `values` and `regexp`.

| Name    | Type     | Description                                                         | Required |
| ------- | -------- | ------------------------------------------------------------------- | -------- |
| key     | string   | Header key to match                                                 | Yes      |
| values  | []string | Header values to match                                              | No       |
| regexp  | string   | Header value in regular expression to match                         | No       |
| backend | string   | backend name (pipeline name in static config, service name in mesh) | Yes      |

### httppipeline.Flow

| Name   | Type              | Description                                                                                                                                                                         | Required |
| ------ | ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| filter | string            | The filter name                                                                                                                                                                     | Yes      |
| jumpIf | map[string]string | Jump to another filter conditionally, the key is the result of the current filter, the value is the jumping filter name. `END` is the built-in value for the ending of the pipeline | No       |

### httppipeline.Filter

The self-defining specification of each filter references to [filters](./filters.md).

| Name                                 | Type   | Description    | Required |
| ------------------------------------ | ------ | -------------- | -------- |
| name                                 | string | Name of filter | Yes      |
| kind                                 | string | Kind of filter | Yes      |
| [self-defining fields](./filters.md) | -      | -              | -        |

### easemonitormetrics.Kafka

| Name    | Type     | Description      | Required                      |
| ------- | -------- | ---------------- | ----------------------------- |
| brokers | []string | Broker addresses | Yes (default: localhost:9092) |
| topic   | string   | Produce topic    | Yes                           |

### nacos.ServerSpec

| Name        | Type   | Description                                  | Required |
| ----------- | ------ | -------------------------------------------- | -------- |
| ipAddr      | string | The ip address                               | Yes      |
| port        | uint16 | The port                                     | Yes      |
| scheme      | string | The scheme of protocol (support http, https) | No       |
| contextPath | string | The context path                             | No       |

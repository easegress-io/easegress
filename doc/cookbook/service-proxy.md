# Service Proxy

- [Service Proxy](#service-proxy)
  - [Basic: Load Balance](#basic-load-balance)
    - [Dynamic: Integration with Service Registry](#dynamic-integration-with-service-registry)
    - [Zookeeper](#zookeeper)
    - [Consul](#consul)
    - [Eureka](#eureka)

Easegress servers as a reverse proxy. It can easily integrate with mainstream Service Registries.  


## Basic: Load Balance

```yaml
name: pipeline-reverse-proxy
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
        policy: roundRobin
```


### Dynamic: Integration with Service Registry

* We integrate `Proxy` with service registries such as [Consul](https://github.com/megaease/easegress/blob/main/doc/controllers.md#consulserviceregistry), [Etcd](https://github.com/megaease/easegress/blob/main/doc/controllers.md#etcdserviceregistry), [Zookeeper](https://github.com/megaease/easegress/blob/main/doc/controllers.md#zookeeperserviceregistry), [Eureka](https://github.com/megaease/easegress/blob/main/doc/controllers.md#eurekaserviceregistry). You need to create one of them to connect the external service registry. The service registry config takes higher priority than static servers. If the dynamic servers pulling failed, it will use static servers if there are.


### Zookeeper

1. First we need to create a ZookeeperServiceRegistry in Easegress 

``` yaml
kind: ZookeeperServiceRegistry
name: zookeeper-001
zkservices: [127.0.0.1:2181]
prefix: /services
conntimeout: 6s
syncInterval: 10s
```

2. Create a pipeline and set its `serviceRegistry` field into `zookeeper-001` and it will look up the zookeeper configuration for the service named `springboot-application-order` as in field `serviceName`.    

```yaml
name: pipeline-reverse-proxy
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
      serviceRegistry: zookeeper-001              # +
      serviceName: springboot-application-order   # +
      loadBalance:
        policy: roundRobin
```


### Consul 

1. First we need to create a ConsulServiceRegistry in Easegress 

```yaml
kind: ConsulServiceRegistry
name: consul-001
address: '127.0.0.1:8500'
scheme: http
syncInterval: 10s
```

2. Create a pipeline and set its `serviceRegistry` field into `consul-001` and it will look up the consul configuration for the service named `springboot-application-order` as in field `serviceName`.    

```yaml
name: pipeline-reverse-proxy
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
      serviceRegistry: consul-001              # +
      serviceName: springboot-application-order   # +
      loadBalance:
        policy: roundRobin
```


### Eureka
1. First we need to create a EurekaServiceRegistry in Easegress 

```yaml
kind: EurekaServiceRegistry
name: eureka-001
endpoints: ['http://127.0.0.1:8761/eureka']
syncInterval: 10s
```


2. Create a pipeline and set its `serviceRegistry` field into `eureka-001` and it will look up the eureka configuration for the service named `springboot-application-order` as in field `serviceName`.    

```yaml
name: pipeline-reverse-proxy
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
      serviceRegistry: eureka-001              # +
      serviceName: springboot-application-order   # +
      loadBalance:
        policy: roundRobin
```


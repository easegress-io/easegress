# Load Balancer

- [Load Balancer](#load-balancer)
  - [Why Use Easegress as Reverse Proxy](#why-use-easegress-as-reverse-proxy)
  - [Basic: Load Balance](#basic-load-balance)
  - [Traffic Adaptor: Change Something of Two-Way Traffic](#traffic-adaptor-change-something-of-two-way-traffic)
  - [References](#references)
    - [Traffic Adaptor: Change Something of Two-Way Traffic](#traffic-adaptor-change-something-of-two-way-traffic-1)

The reverse proxy is the common middleware that is accessed by clients, forwards them to backend servers. The reverse proxy is a very core role played by Easegress.

## Why Use Easegress as Reverse Proxy

Easegress integrates many features as a reverse proxy with easy configuration.

- Offload SSL/TLS layer
- Hot-updated routing without losing requests
- Intercept invalid requests (according to IP, metadata of requests, credential verification, etc)
- Adapting requests to satisfy requirements of backend servers
- Bring more resilience for backend servers (rate limiting, time limiting, circuit breaker, retrying)
- Load balance for backend servers (static addresses, or integration with service registry)
- Compression and Caching for response

## Basic: Load Balance

The filter `Proxy` is the filter to fire requests to backend servers. It contains servers group under load balance, whose policy support `roundRobin`, `random`, `weightedRandom`, `ipHash`, `headerHash`.

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

## Traffic Adaptor: Change Something of Two-Way Traffic

Sometimes backend applications can't adapt to quick changes of requirements of traffic. Easegress could be an adaptor between new traffic and old applications. There are 2 phases of adaption in reverse proxy: request adaption, response adaption. `RequestAdaptor` supports the adaption of a method, path, header, and body. `ResponseAdaptor` supports the adaption of header and body. As you can see, the flow in spec plays a critical role.

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: requestAdaptor
  - filter: proxy
  - filter: responseAdaptor
filters:
  - name: requestAdaptor
    kind: RequestAdaptor
    host: easegress.megaease.com
    method: POST
    path:
      addPrefix: /apis/v2
    header:
      set:
        X-Api-Version: v2

  - name: responseAdaptor
    kind: ResponseAdaptor
    header:
      set:
        Server: Easegress v1.0.0
      add:
        X-Easegress-Pipeline: pipeline-reverse-proxy

  - name: proxy
    kind: Proxy
    # ...
```

For the full YAML, see [here](#traffic-adaptor-change-something-of-two-way-traffic-1)

## References

### Traffic Adaptor: Change Something of Two-Way Traffic

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: requestAdaptor
  - filter: proxy
  - filter: responseAdaptor
filters:
  - name: requestAdaptor
    kind: RequestAdaptor
    host: easegress.megaease.com
    method: POST
    path:
      trimPrefix: /apis/v2
    header:
      set:
        X-Api-Version: v2

  - name: responseAdaptor
    kind: ResponseAdaptor
    header:
      set:
        Server: Easegress v1.0.0
      add:
        X-Easegress-Pipeline: pipeline-reverse-proxy

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

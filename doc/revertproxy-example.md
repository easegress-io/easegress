# Reverse Proxy
Reverse proxy is the common middleware which is  accessed by clients, forwards them to backend servers. Easegres Reverse proxy is a very core role played by Easegress.
## Why Use Easegress as Reverse Proxy
Easegress integrates much features as a reverse proxy with easy configuration.
- Offload SSL/TLS layer
- Hot-udpated routing without losing requests
- Intercept invalid requests (according to IP, metadata of requests, credential verification, etc)
- Adapting requests to satify requirements of backend servers
- Bring more resilence for backend servers (rate limiting, time limiting, circuit breaker, retrying)
- Load balance for backend servers (static addresses, or integration with service registry)
- Compression and Caching for response
## Cookbook as Reverse Proxy
### Basic: Load Balance
```yaml
name: pipeline-reverse-proxy
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
        policy: roundRobin
```
### Dynamic: Integration with Service Registry
```yaml
mainPool:
  serviceRegisrty: zookeeper-001				# +
  serviceName: springboot-application-order		# +
```
### Conditional Door: Intercept Invalid Requests
(Validator)
### Traffic Adaptor: Change Something of Two-Way Traffic
(RequestAdaptor, ResponseAdaptor)
### More Security: Verify Credential
(JWT, HMAC, etc)
### More Livingness: Resilience of Service
(circuitbreker, ratelimiter, retryer, timelimiter)
### Resouce Saving: Compression and Caching
> NOTE: When there are multiple instances of Easegress, the configuration applys for every instance equally. For example, TPS of RateLimiter is configured with 100 in 3-instances cluster, so the total TPS will be 300.
(compression in proxy filter, cache in http server)
## Reference
Complete example of configuration.
### Dynamic: Integration with Service Registry
```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      serviceRegisty: zookeeper-001
      serviceName: springboot-application-order
      loadBalance:
        policy: roundRobin
```
### Conditional Door: Intercept Invalid Requests
### Traffic Adaptor: Change Something of Two-Way Traffic
### More Security: Verify Credential
### More Livingness: Resilience of Service
### Resouce Saving: Compression and Caching

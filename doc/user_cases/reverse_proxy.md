- [Reverse Proxy](#reverse-proxy)
  - [Why Use Easegress as Reverse Proxy](#why-use-easegress-as-reverse-proxy)
  - [Cookbook as Reverse Proxy](#cookbook-as-reverse-proxy)
    - [Basic: Load Balance](#basic-load-balance)
    - [Dynamic: Integration with Service Registry](#dynamic-integration-with-service-registry)
    - [Conditional Door: Intercept Invalid Requests](#conditional-door-intercept-invalid-requests)
    - [Traffic Adaptor: Change Something of Two-Way Traffic](#traffic-adaptor-change-something-of-two-way-traffic)
    - [More Security: Verify Credential](#more-security-verify-credential)
    - [More Livingness: Resilience of Service](#more-livingness-resilience-of-service)
      - [CircuitBreaker](#circuitbreaker)
      - [RateLimiter](#ratelimiter)
      - [Retryer](#retryer)
      - [TimeLimiter](#timelimiter)
    - [Resouce Saving: Compression and Caching](#resouce-saving-compression-and-caching)
  - [Reference](#reference)
    - [Dynamic: Integration with Service Registry](#dynamic-integration-with-service-registry-1)
    - [Conditional Door: Intercept Invalid Requests](#conditional-door-intercept-invalid-requests-1)
    - [Traffic Adaptor: Change Something of Two-Way Traffic](#traffic-adaptor-change-something-of-two-way-traffic-1)
    - [More Security: Verify Credential](#more-security-verify-credential-1)
    - [More Livingness: Resilience of Service](#more-livingness-resilience-of-service-1)
      - [CircuitBreaker](#circuitbreaker-1)
      - [RateLimiter](#ratelimiter-1)
      - [Retryer](#retryer-1)
      - [TimeLimiter](#timelimiter-1)
    - [Resource Saving: Compression and Caching](#resource-saving-compression-and-caching)
    - [Concpets](#concpets)

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

1. Using Headers validation in Easegress. This is the simplest way for validating requests. It checks the HTTP request headers by using regular expression matching.

``` yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: header-validator
  - filter: proxy

#...
  - kind: Validator
    name: header-validator
    headers:
      Is-Valid:
        values: ["abc", "goodplan"]
        regexp: "^ok-.+$"
#...

```

In order to enable Header type validator correctly in pipeline, we should add it before proxy filter.
And the example above will check the `Is-Valid` header field by trying to match `abc` or `goodplan`. Also, it will use `^ok-.+$` regular expression for checking if it can't match the `values` filed.


2. Using JWT validation in Easegress. JWT is wildly used in modern web environment. JSON Web Token (JWT, pronounced /dʒɒt/, same as the word "jot") is a proposed Internet standard for creating data with optional signature and/or optional encryption whose payload holds JSON that asserts some number of claims.[1]
> Easegress support three types of JWT, HS256, HS384, and HS512.


``` yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: jwt-validator
  - filter: proxy

#...
  - kind: Validator
    name: jwt-validator
    jwt:
      cookieName: auth
      algorithm: HS256
      secret: 6d79736563726574
#...

```
The example above will check the value named `auth` in cookie with HS256 with secret,6d79736563726574.


3. Using Signature validation in Easegress. Signature validation implements an Amazon Signature V4[2] compatible signature validation validator. Once you enable this kind of validation, please make sure your HTTP client has followed the signature generation process in AWS V4 doc and bring it to request Easegress.

``` yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: signature-validator
  - filter: proxy

#...
  - kind: Validator
    name: signature-validator
    signature:
      accessKeys:
        AKID: SECRET
#...

```

The example here only use an accessKeys for processing Amazon Signature V4 validation. It also has other complicated and customized filed for more security purpose. Check it out in Easegress filter doc if needed.[3]

4. Using Oauth2 validation in Easegress
OAuth 2.0 is the industry-standard protocol for authorization. OAuth 2.0 focuses on client developer simplicity while providing specific authorization flows for web applications, desktop applications, mobile phones, and living room devices. This specification and its extensions are being developed within the IETF OAuth Working Group.[4]

``` yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: oauth-validator
  - filter: proxy

#...
  - kind: Validator
    name: oauth-validator
    oauth2:
      tokenIntrospect:
      endPoint: https://127.0.0.1:8443/auth/realms/test/protocol/openid-connect/token/introspect
      clientId: easegress
      clientSecret: 42620d18-871d-465f-912a-ebcef17ecb82
      insecureTls: false
#...

```

The example above uses a token introspection server, which is provide by `endpoint` filed for validation. It also support `Self-Encoded Access Tokens mode` which will require a JWT related configuration related. Check it out in Easegress filter doc if needed. [5]


### More Livingness: Resilience of Service

#### CircuitBreaker

Below is an example configuration with a `COUNT_BASED` policy. `GET` request to paths begin with `/books/` uses this policy, which short-circuits requests if more than half of the last 100 requests failed with status code 500, 503, or 504.

```yaml
flow:
  - filter: circuit-breaker                           # +
  - filter: proxy
filters:
  - name: circuit-breaker                             # +
    kind: CircuitBreaker                              # +
    policies:                                         # +
    - name: count-based-policy                        # +
      slidingWindowType: COUNT_BASED                  # +
      failureRateThreshold: 50                        # +
      slidingWindowSize: 100                          # +
      failureStatusCodes: [500, 503, 504]             # +
    urls:                                             # +
    - methods: [GET]                                  # +
      url:                                            # +
        prefix: /books/                               # +
      policyRef: count-based-policy                   # +
  - name: proxy
```

And we can add a `TIME_BASED` policy, `GET` & `POST` requests to paths that match regular express `^/users/\d+$` uses this policy, which short-circuits requests if more than 60% of the requests within the last 200 seconds failed.

```yaml
    policies:
    - name: time-based-policy                         # +
      slidingWindowType: TIME_BASED                   # +
      failureRateThreshold: 60                        # +
      slidingWindowSize: 200                          # +
      failureStatusCodes: [500, 503, 504]             # +
    urls:
    - methods: [GET, POST]                            # +
      url:                                            # +
        regex: ^/users/\d+$                           # +
      policyRef: time-based-policy                    # +
```

In addition to failures, the circuit breaker can also short-circuits requests on slow requests. Below configuration regards requests which costs more than 30 seconds as slow requests, and short-circuits requests if 60% of recent requests are slow.

```yaml
    policies:
    - name: count-based-policy
      slowCallRateThreshold: 60                       # +
      slowCallDurationThreshold: 30s                  # +
```

For a policy, if the first request fails, the failure rate could be 100% because there's only one request. This is not a desired behavior in most cases, we can avoid it by specify `minimumNumberOfCalls`.

```yaml
    policies:
    - name: count-based-policy
      minimumNumberOfCalls: 10                        # +
```

We can also configure the wait duration in open state, and the max wait duration in half-open state:

```yaml
    policies:
    - name: count-based-policy
      waitDurationInOpenState: 2m                     # +
      maxWaitDurationInHalfOpenState: 1m              # +
```

In half-open state, we can limit the number of permitted requests:

```yaml
    policies:
    - name: count-based-policy
      permittedNumberOfCallsInHalfOpenState: 10       # +
```

#### RateLimiter

The below configuration limits the request rate for requests to `/admin` and requests that match regular expression `^/pets/\d+$`.

```yaml
flow:
  - filter: rate-limiter                              # +
  - filter: proxy
filters:
  - name: rate-limiter                                # +
    kind: RateLimiter                                 # +
    policies:                                         # +
    - name: policy-example                            # +
      timeoutDuration: 100ms                          # +
      limitRefreshPeriod: 10ms                        # +
      limitForPeriod: 50                              # +
    defaultPolicyRef: policy-example                  # +
    urls:                                             # +
    - methods: [GET, POST, PUT, DELETE]               # +
      url:                                            # +
        exact: /admin                                 # +
        regex: ^/pets/\d+$                            # +
      policyRef: policy-example                       # +
  - name: proxy
```

#### Retryer

If we want to retry on HTTP status code 500, 503, and 504, we can create a `Retryer` with below configuration, it make at most 3 attempts on failure.

```yaml
flow:
  - filter: retryer                                   # +
  - filter: proxy
filters:
  - name: retryer                                     # +
    kind: Retryer                                     # +
    policies:                                         # +
    - name: policy-example                            # +
      maxAttempts: 3                                  # +
      waitDuration: 500ms                             # +
      failureStatusCodes: [500, 503, 504]             # +
    defaultPolicyRef: policy-example                  # +
    urls:                                             # +
    - methods: [GET, POST, PUT, DELETE]               # +
      url:                                            # +
        prefix: /books/                               # +
      policyRef: policy-example                       # +
  - name: proxy
```

By default, the wait duration between two attempts is `waitDuration`, but this can be changed by specify `backOffPolicy` and `randomizationFactor`.

```yaml
    - name: policy-example
      backOffPolicy: Exponential                      # +
      randomizationFactor: 0.5                        # +
```

#### TimeLimiter

TimeLimiter limits the time of requests, a request is canceled if it cannot get a response in configured duration.

```yaml
flow:
  - filter: time-limiter                              # +
  - filter: proxy
filters:
  - name: time-limiter                                # +
    kind: TimeLimiter                                 # +
    defaultTimeoutDuration: 500ms                     # +
    urls:                                             # +
    - methods: [POST]                                 # +
      url:                                            # +
        exact: /users/1                               # +
      timeoutDuration: 400ms                          # +
  - name: proxy
```

### Resouce Saving: Compression and Caching

> NOTE: When there are multiple instances of Easegress, the configuration applys for every instance equally. For example, TPS of RateLimiter is configured with 100 in 3-instances cluster, so the total TPS will be 300.

1. Compression in proxy filter
Easegress proxy filter supports `gzip` type compression.

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
#...
    compression:
      minLength: 1024
#...
```

As the example above, we only need to value the `minLength` field to tell Easegress' proxy filter avoiding gzip the response body if the response body doesn't lagert the `minLength`. Also it will add the `gzip` header automatically if it's truly invoked.

2. Caching in proxy filter
Easegress proxy filter has `pool` section for describing forwarding backends. And it also supports cacheing the response according to the HTTP Methods and the HTTP response code.

``` yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
 #...
      memoryCache:
        expiration: 10s
        maxEntryBytes: 4096
        codes:
        - 200
        - 201
        methods:
        - GET
        - HEAD
 #...
```

The example above will cache the response which size is smaller than 4096, and response code is 200 or 201, with HTTP method Get and Head.

3. Caching in HTTPServer
As a traffic gate of Easegress, HTTPServer also supports caching. It can be used in serving static resource scenario. HTTPServer will use request's Host, Method, and Path to form a key for build-in LRU cache. Recommend to enable this feature only in the HTTPServer for routing static resources.(Easegress supports multiple HTTPServers for different usage purpose)

```yaml
kind: HTTPServer
name: http-server-example
port: 10080
https: false
#...
cacheSize: 10240
#...
```

As the example above, all we need is valuing the `cacheSize` to indicated the lru cache's size. It will disuse the least used cache value firstly.

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

1. Header

``` yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: header-validator
  - filter: proxy
filters:
  - kind: Validator
    name: header-validator
    headers:
      Is-Valid:
        values: ["abc", "goodplan"]
        regexp: "^ok-.+$"
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

2. JWT

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: jwt-validator
  - filter: proxy
filters:
  - kind: Validator
    name: jwt-validator
    jwt:
      cookieName: auth
      algorithm: HS256
      secret: 6d7973656372657
       - name: proxy
  - kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

3. Signature

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: signature-validator
  - filter: proxy
filters:
  - kind: Validator
    name: signature-validator
    signature:
      accessKeys:
        AKID: SECRET
  - kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```
4. OAuth

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: oauth-validator
  - filter: proxy
filters:
  - kind: Validator
    name: oauth-validator
    oauth2:
      tokenIntrospect:
      endPoint: https://127.0.0.1:8443/auth/realms/test/protocol/openid-connect/token/introspect
      clientId: easegress
      clientSecret: 42620d18-871d-465f-912a-ebcef17ecb82
      insecureTls: false
  - kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

### More Livingness: Resilience of Service

#### CircuitBreaker

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: circuit-breaker
  - filter: proxy
filters:
  - name: circuit-breaker
    kind: CircuitBreaker
    policies:
    - name: count-based-policy
      slidingWindowType: COUNT_BASED
      failureRateThreshold: 50
      slidingWindowSize: 100
      failureStatusCodes: [500, 503, 504]
      slowCallRateThreshold: 60
      slowCallDurationThreshold: 30s
      minimumNumberOfCalls: 10
      waitDurationInOpenState: 2m
      maxWaitDurationInHalfOpenState: 1m
      permittedNumberOfCallsInHalfOpenState: 10
    - name: time-based-policy
      slidingWindowType: TIME_BASED
      failureRateThreshold: 60
      slidingWindowSize: 200
      failureStatusCodes: [500, 503, 504]
    urls:
    - methods: [GET]
      url:
        prefix: /books/
      policyRef: count-based-policy
    - methods: [GET, POST]
      url:
        regex: ^/users/\d+$
      policyRef: time-based-policy
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

#### RateLimiter

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: rate-limiter
  - filter: proxy
filters:
  - name: rate-limiter
    kind: RateLimiter
    policies:
    - name: policy-example
      timeoutDuration: 100ms
      limitRefreshPeriod: 10ms
      limitForPeriod: 50
    defaultPolicyRef: policy-example
    urls:
    - methods: [GET, POST, PUT, DELETE]
      url:
        exact: /admin
        regex: ^/pets/\d+$
      policyRef: policy-example
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

#### Retryer

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: retryer
  - filter: proxy
filters:
  - name: retryer
    kind: Retryer
    policies:
    - name: policy-example
      backOffPolicy: Exponential
      randomizationFactor: 0.5
      maxAttempts: 3
      waitDuration: 500ms
      failureStatusCodes: [500, 503, 504]
    defaultPolicyRef: policy-example
    urls:
    - methods: [GET, POST, PUT, DELETE]
      url:
        prefix: /books/
      policyRef: policy-example
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

#### TimeLimiter

```yaml
name: pipeline-reverse-proxy
kind: HTTPPipeline
flow:
  - filter: time-limiter
  - filter: proxy
filters:
  - name: time-limiter
    kind: TimeLimiter
    defaultTimeoutDuration: 500ms
    urls:
    - methods: [POST]
      url:
        exact: /users/1
      timeoutDuration: 400ms
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

### Resource Saving: Compression and Caching

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
     compression:
       minLength: 1024
```

2. Caching in proxy filter's pool section

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
      memoryCache:
        expiration: 10s
        maxEntryBytes: 4096
        codes:
        - 200
        - 201
        methods:
        - GET
        - HEAD
```

### Concepts
1. https://en.wikipedia.org/wiki/JSON_Web_Token
2. https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
3. https://github.com/megaease/easegress/blob/main/doc/filters.md#signerliteral
4. https://oauth.net/2/
5. https://github.com/megaease/easegress/blob/main/doc/filters.md#validatorOAuth2JWT


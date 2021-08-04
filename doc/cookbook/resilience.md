# Resilience

- [Resilience](#resilience)
  - [Basic: Load Balance](#basic-load-balance)
  - [More Livingness: Resilience of Service](#more-livingness-resilience-of-service)
    - [CircuitBreaker](#circuitbreaker)
    - [RateLimiter](#ratelimiter)
    - [Retryer](#retryer)
    - [TimeLimiter](#timelimiter)
  - [References](#references)
    - [CircuitBreaker](#circuitbreaker-1)
    - [RateLimiter](#ratelimiter-1)
    - [Retryer](#retryer-1)
    - [TimeLimiter](#timelimiter-1)
    - [Concepts](#concepts)

The as a Cloud Native traffic orchestrator, Easegress supports build-in resilience features. It is the ability of your system to react to failure and still remain functional. It's not about avoiding failure, but accepting failure and constructing your cloud-native services to respond to it. You want to return to a fully functioning state quickly as possible.[1]

## Basic: Load Balance

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

## More Livingness: Resilience of Service

### CircuitBreaker

Below is an example configuration with a `COUNT_BASED` policy. `GET` request to paths begin with `/books/` uses this policy, which short-circuits requests if more than half of the last 100 requests failed with status code 500, 503, or 504.

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
    urls:
    - methods: [GET]
      url:
        prefix: /books/
      policyRef: count-based-policy
  - name: proxy
    kind: Proxy
```

And we can add a `TIME_BASED` policy, `GET` & `POST` requests to paths that match regular express `^/users/\d+$` uses this policy, which short-circuits requests if more than 60% of the requests within the last 200 seconds failed.

```yaml
    policies:
    - name: time-based-policy
      slidingWindowType: TIME_BASED
      failureRateThreshold: 60
      slidingWindowSize: 200
      failureStatusCodes: [500, 503, 504]
    urls:
    - methods: [GET, POST]
      url:
        regex: ^/users/\d+$
      policyRef: time-based-policy
```

In addition to failures, the circuit breaker can also short-circuit requests on slow requests. Below configuration regards requests which cost more than 30 seconds as slow requests and short-circuits requests if 60% of recent requests are slow.

```yaml
    policies:
    - name: count-based-policy
      slowCallRateThreshold: 60
      slowCallDurationThreshold: 30s
```

For a policy, if the first request fails, the failure rate could be 100% because there's only one request. This is not the desired behavior in most cases, we can avoid it by specifying `minimumNumberOfCalls`.

```yaml
    policies:
    - name: count-based-policy
      minimumNumberOfCalls: 10
```

We can also configure the wait duration in the `open` state and the max wait duration in the `half-open` state:

```yaml
    policies:
    - name: count-based-policy
      waitDurationInOpenState: 2m
      maxWaitDurationInHalfOpenState: 1m
```

In the `half-open` state, we can limit the number of permitted requests:

```yaml
    policies:
    - name: count-based-policy
      permittedNumberOfCallsInHalfOpenState: 10
```

For the full YAML, see [here](#circuitbreaker-1)

### RateLimiter

> NOTE: When there are multiple instances of Easegress, the configuration will be applied for every instance equally. For example, TPS of RateLimiter is configured with 100 in 3-instances cluster, so the total TPS will be 300.

The below configuration limits the request rate for requests to `/admin` and requests that match regular expression `^/pets/\d+$`.

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
```

For the full YAML, see [here](#ratelimiter-1)

### Retryer

If we want to retry on HTTP status code 500, 503, and 504, we can create a `Retryer` with the below configuration, it makes at most 3 attempts on failure.

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
```

By default, the wait duration between two attempts is `waitDuration`, but this can be changed by specifying `backOffPolicy` and `randomizationFactor`.

```yaml
    - name: policy-example
      backOffPolicy: Exponential
      randomizationFactor: 0.5
```

For the full YAML, see [here](#retryer-1)

### TimeLimiter

TimeLimiter limits the time of requests, a request is canceled if it cannot get a response in configured duration.

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
```

For the full YAML, see [here](#timelimiter-1)

## References

### CircuitBreaker

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

### RateLimiter

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

### Retryer

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

### TimeLimiter

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

### Concepts
1. https://docs.microsoft.com/en-us/dotnet/architecture/cloud-native/resiliency

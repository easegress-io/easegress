# Resilience

- [Resilience](#resilience)
  - [Basic: Load Balance](#basic-load-balance)
  - [More Livingness: Resilience of Service](#more-livingness-resilience-of-service)
    - [CircuitBreaker](#circuitbreaker)
    - [RateLimiter](#ratelimiter)
    - [Retry](#retry)
    - [TimeLimiter](#timelimiter)
  - [References](#references)
    - [CircuitBreaker](#circuitbreaker-1)
    - [RateLimiter](#ratelimiter-1)
    - [Retry](#retry-1)
    - [Concepts](#concepts)

As a Cloud Native traffic orchestrator, Easegress supports build-in resilience
features. It is the ability of your system to react to failure and still
remain functional. It's not about avoiding failure, but accepting failure
and constructing your cloud-native services to respond to it. You want to
return to a fully functioning state quickly as possible.[1]

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

## More Livingness: Resilience of Service

### CircuitBreaker

CircuitBreaker leverges a finite state machine to implement the processing
logic, the state machine has three states: `CLOSED`, `OPEN`, and `HALF_OPEN`.
When the state is `CLOSED`, requests pass through normally, state transits
to `OPEN` if request failure rate or slow request rate reach a configured
threshold and requests will be shor-circuited in this state. After a
configured duration, state transits from `OPEN` to `HALF_OPEN`, in which a
limited number of requests are permitted to pass through while other
requests are still short-circuited, and state transit to `CLOSED` or `OPEN`
based on the results of the permitted requests.

When `CLOSED`, it uses a sliding window to store and aggregate the result
of recent requests, the window can either be `COUNT_BASED` or `TIME_BASED`.
The `COUNT_BASED` window aggregates the last N requests and the `TIME_BASED`
window aggregates requests in the last N seconds, where N is the window size.

Below is an example configuration with a `COUNT_BASED` policy. `GET` request
to paths begin with `/books/` uses this policy, which short-circuits requests
if more than half of the last 100 requests failed with status code 500, 503,
or 504.

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
    circuitBreakerPolicy: countBased
    failureCodes: [500, 503, 504]

resilience:
- name: countBased
  kind: CircuitBreaker
  slidingWindowType: COUNT_BASED
  failureRateThreshold: 50
  slidingWindowSize: 100
```

And we can also use a `TIME_BASED` policy, which short-circuits requests
if more than 60% of the requests within the last 200 seconds failed.

```yaml
resilience:
- name: time-based-policy
  kind: CircuitBreaker
  slidingWindowType: TIME_BASED
  failureRateThreshold: 60
  slidingWindowSize: 200
```

In addition to failures, we can also short-circuit slow requests. Below
configuration regards requests which cost more than 30 seconds as slow
requests and short-circuits requests if 60% of recent requests are slow.

```yaml
resilience:
- name: countBased
  kind: CircuitBreaker
  slowCallRateThreshold: 60
  slowCallDurationThreshold: 30s
```

For a policy, if the first request fails, the failure rate could be 100%
because there's only one request. This is not the desired behavior in most
cases, we can avoid it by specifying `minimumNumberOfCalls`.

```yaml
resilience:
- name: countBased
  kind: CircuitBreaker
  minimumNumberOfCalls: 10
```

We can also configure the wait duration in the `open` state and the max
wait duration in the `half-open` state:

```yaml
resilience:
- name: countBased
  kind: CircuitBreaker
  waitDurationInOpenState: 2m
  maxWaitDurationInHalfOpenState: 1m
```

In the `half-open` state, we can limit the number of permitted requests:

```yaml
resilience:
- name: countBased
  kind: CircuitBreaker
  permittedNumberOfCallsInHalfOpenState: 10
```

For the full YAML, see [here](#circuitbreaker-1), and please refer
[CircuitBreaker Policy](../reference/controllers.md#circuitbreaker-policy]
for more information.

### RateLimiter

> NOTE: When there are multiple instances of Easegress, the configuration
> will be applied for every instance equally. For example, TPS of RateLimiter
> is configured with 100 in 3-instances cluster, so the total TPS will be 300.

The below configuration limits the request rate for requests to `/admin`
and requests that match regular expression `^/pets/\d+$`.

```yaml
name: pipeline-reverse-proxy
kind: Pipeline

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

For the full YAML, see [here](#ratelimiter-1).

### Retry

If we want to retry a failed request, for example, retry on HTTP status
codes 500, 503, and 504, we can create a `RetryerPolicy` with the below
configuration, it makes at most 3 attempts on failure.

```yaml
name: pipeline-reverse-proxy
kind: Pipeline

flow:
- filter: retryer
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
    retryPolicy: retry3Times
    failureCodes: [500, 503, 504]
    
resilience:
- name: retry3Times
  kind: Retry
  maxAttempts: 3
  waitDuration: 500ms
```

By default, the wait duration between two attempts is `waitDuration`, but
this can be changed by specifying `backOffPolicy` and `randomizationFactor`.

```yaml
resilience:
- name: retry3Times
  kind: Retry
  backOffPolicy: Exponential
  randomizationFactor: 0.5
```

For the full YAML, see [here](#retry-1), and please refer
[Retry Policy](../reference/controllers.md#retry-policy] for more information.

### TimeLimiter

TimeLimiter limits the time of requests, a request is canceled if it cannot
get a response in configured duration. As this resilience type only requires
config a timeout duration, it is implemented directly on filters like `Proxy`.

```yaml
name: pipeline-reverse-proxy
kind: Pipeline

flow:
- filter: retryer
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
    timeout: 500ms
```

## References

### CircuitBreaker

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
    circuitBreakerPolicy: countBasedPolicy
    failureCodes: [500, 503, 504]

resilience:
- name: countBasedPolicy
  kind: CircuitBreaker
  slidingWindowType: COUNT_BASED
  failureRateThreshold: 50
  slidingWindowSize: 100
  slowCallRateThreshold: 60
  slowCallDurationThreshold: 30s
  minimumNumberOfCalls: 10
  waitDurationInOpenState: 2m
  maxWaitDurationInHalfOpenState: 1m
  permittedNumberOfCallsInHalfOpenState: 10
- name: timeBasedPolicy
  kind: CircuitBreaker
  slidingWindowType: TIME_BASED
  failureRateThreshold: 60
  slidingWindowSize: 200
```

### RateLimiter

```yaml
name: pipeline-reverse-proxy
kind: Pipeline

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
  pools:
  - servers:
    - url: http://127.0.0.1:9095
    - url: http://127.0.0.1:9096
    - url: http://127.0.0.1:9097
    loadBalance:
      policy: roundRobin
```

### Retry

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
    retryPolicy: retry3Times
    failureCodes: [500, 503, 504]

resilience:
- name: retry3Times
  kind: Retry
  backOffPolicy: Exponential
  randomizationFactor: 0.5
  maxAttempts: 3
  waitDuration: 500ms
```

### Concepts
1. https://docs.microsoft.com/en-us/dotnet/architecture/cloud-native/resiliency

# Filters

- [Filters](#filters)
  - [APIAggregator](#apiaggregator)
    - [Configuration](#configuration)
    - [Results](#results)
  - [Proxy](#proxy)
    - [Configuration](#configuration-1)
    - [Results](#results-1)
  - [Bridge](#bridge)
    - [Configuration](#configuration-2)
    - [Results](#results-2)
  - [CORSAdaptor](#corsadaptor)
    - [Configuration](#configuration-3)
    - [Results](#results-3)
  - [Fallback](#fallback)
    - [Configuration](#configuration-4)
    - [Results](#results-4)
  - [Mock](#mock)
    - [Configuration](#configuration-5)
    - [Results](#results-5)
  - [RemoteFilter](#remotefilter)
    - [Configuration](#configuration-6)
    - [Results](#results-6)
  - [RequestAdaptor](#requestadaptor)
    - [Configuration](#configuration-7)
    - [Results](#results-7)
  - [CircuitBreaker](#circuitbreaker)
    - [Configuration](#configuration-8)
    - [Results](#results-8)
  - [RateLimiter](#ratelimiter)
    - [Configuration](#configuration-9)
    - [Results](#results-9)
  - [TimeLimiter](#timelimiter)
    - [Configuration](#configuration-10)
    - [Results](#results-10)
  - [Retryer](#retryer)
    - [Configuration](#configuration-11)
    - [Results](#results-11)
  - [ResponseAdaptor](#responseadaptor)
    - [Configuration](#configuration-12)
    - [Results](#results-12)
  - [Validator](#validator)
    - [Configuration](#configuration-13)
    - [Results](#results-13)
  - [WasmHost](#wasmhost)
    - [Configuration](#configuration-14)
    - [Results](#results-14)
  - [Common Types](#common-types)
    - [apiaggregator.Pipeline](#apiaggregatorpipeline)
    - [pathadaptor.Spec](#pathadaptorspec)
    - [pathadaptor.RegexpReplace](#pathadaptorregexpreplace)
    - [httpheader.AdaptSpec](#httpheaderadaptspec)
    - [proxy.FallbackSpec](#proxyfallbackspec)
    - [proxy.PoolSpec](#proxypoolspec)
    - [proxy.Server](#proxyserver)
    - [proxy.LoadBalance](#proxyloadbalance)
    - [memorycache.Spec](#memorycachespec)
    - [httpfilter.Spec](#httpfilterspec)
    - [urlrule.StringMatch](#urlrulestringmatch)
    - [urlrule.URLRule](#urlruleurlrule)
    - [resilience.URLRule](#resilienceurlrule)
    - [httpfilter.Probability](#httpfilterprobability)
    - [proxy.Compression](#proxycompression)
    - [mock.Rule](#mockrule)
    - [circuitbreaker.Policy](#circuitbreakerpolicy)
    - [ratelimiter.Policy](#ratelimiterpolicy)
    - [timelimiter.URLRule](#timelimiterurlrule)
    - [retryer.Policy](#retryerpolicy)
    - [httpheader.ValueValidator](#httpheadervaluevalidator)
    - [validator.JWTValidatorSpec](#validatorjwtvalidatorspec)
    - [signer.Spec](#signerspec)
    - [signer.Literal](#signerliteral)
    - [validator.OAuth2ValidatorSpec](#validatoroauth2validatorspec)
    - [validator.OAuth2TokenIntrospect](#validatoroauth2tokenintrospect)
    - [validator.OAuth2JWT](#validatoroauth2jwt)

A Filter is a request/response processor. Multiple filters can be orchestrated together to form a pipeline, each filter returns a string result after it finishes processing the input request/response. An empty result means the input was successfully processed by the current filter and can go forward to the next filter in the pipeline, while a non-empty result means the pipeline or preceding filter need to take extra action.

## APIAggregator

The API Aggregator forwards one request to multiple API HTTP Pipelines in the same namespace and aggregates responses.

Below is an example configuration that forwards one request to two pipelines, `http-pipeline-1` and `http-pipeline-2`. When forwarding a request to `http-pipeline-2`, the request method is changed to `GET` and a new header `Original-Method` is added with the original request method. The two responses are merged into one before return to the client.

```yaml
kind: APIAggregator
name: api-aggregator-example
mergeResponse: true
pipelines:
- name: http-pipeline-1
- name: http-pipeline-2
  method: GET
  header:
    add:
      Original-Method: "[[filter.api-aggregator-example.req.method]]"
  disableBody: false
```

### Configuration

| Name           | Type                                               | Description                                                                                                                                     | Required |
| -------------- | -------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| maxBodyBytes   | int64                                              | The upper limit of request body size, default is 0 which means no limit                                                                         | No       |
| partialSucceed | bool                                               | Whether regards the result of the original request as successful or not when a request to some of the API pipelines fails, default is false     | No       |
| timeout        | string                                             | Timeout duration for requests to API proxies                                                                                                    | No       |
| mergeResponse  | bool                                               | Whether merging the multiple response objects into one, default is false means the final response is an array of the responses from API proxies | No       |
| pipelines      | [][apiaggregator.Pipeline](#apiaggregatorpipeline) | Configuration of API proxies                                                                                                                    | Yes      |

### Results

| Value  | Description                                         |
| ------ | --------------------------------------------------- |
| failed | The APIAggregator has failed to process the request |

## Proxy

The Proxy filter is a proxy of backend service.

Below is one of the simplest Proxy configurations, it forward requests to `http://127.0.0.1:9095`.

```yaml
kind: Proxy
name: proxy-example-1
mainPool:
  servers:
  - url: http://127.0.0.1:9095
```

Besides `mainPool`, `candidatePools` can also be configured, if so, Proxy first checks if one of the candidate pools can process a request. For example, the candidate pool in the below configuration randomly selects and processes 30‰ of requests, and the main pool processes the other 970‰ of requests.

```yaml
kind: Proxy
name: proxy-example-2
mainPool:
  servers:
  - url: http://127.0.0.1:9095
candidatePools:
  - servers:
    - url: http://127.0.0.2:9095
    probability:
      perMill: 30
      policy: random
```

Servers of a pool can also be dynamically configured via service discovery, the below configuration gets a list of servers by `serviceRegistry` & `serviceName`, and only servers that have tag `v2` are selected.

```yaml
kind: Proxy
name: proxy-example-3
mainPool:
  serverTags: ["v2"]
  serviceName: service-001
  serviceRegistry: eureka-service-registry-example
```

When there are multiple servers in a pool, the Proxy can do a load balance between them:

```yaml
kind: Proxy
name: proxy-example-4
mainPool:
  serverTags: ["v2"]
  serviceName: service-001
  serviceRegistry: eureka-service-registry-example
  loadBalance:
    policy: roundRobin
    headerHashKey: X-User-Id
```

### Configuration

| Name           | Type                                           | Description                                                                                                                                                                                                                                                                                                         | Required |
| -------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| fallback       | [proxy.FallbackSpec](#proxyFallbackSpec)       | Fallback steps when failed to send a request or receives a failure response                                                                                                                                                                                                                                         | No       |
| mainPool       | [proxy.PoolSpec](#proxyPoolSpec)               | Main pool of backend servers                                                                                                                                                                                                                                                                                        | Yes      |
| candidatePools | [][proxy.PoolSpec](#proxyPoolSpec)             | One or more pool configuration similar with `mainPool` but with `filter` options configured. When `Proxy` get a request, it first goes through the pools in `candidatePools`, and if one of the pools filter in the request, servers of this pool handles the request, otherwise, the request is pass to `mainPool` | No       |
| mirrorPool     | [proxy.PoolSpec](#proxyPoolSpec)               | Definition a mirror pool, requests are sent to this pool simultaneously when they are sent to candidate pools or main pool                                                                                                                                                                                          | No       |
| failureCodes   | []int                                          | HTTP status codes need to be handled as failure                                                                                                                                                                                                                                                                     | No       |
| compression    | [proxy.CompressionSpec](#proxyCompressionSpec) | Response compression options                                                                                                                                                                                                                                                                                        | No       |

### Results

| Value         | Description                          |
| ------------- | ------------------------------------ |
| fallback      | Fallback steps have been executed    |
| internalError | Encounters an internal error         |
| clientError   | Client-side(Easegress) network error |
| serverError   | Server-side network error            |

## Bridge

The Bridge filter route requests from one pipeline to other pipelines or HTTP proxies under an HTTP server.

The upstream filter set the target pipeline/proxy in request header `X-Easegress-Bridge-Dest`. Bridge extracts the header value and tries to match it in the configuration. It sends the request if a destination matched and aborts the process if no match. It selects the first destination from the filter configuration if there's no header named `X-Easegress-Bridge-Dest`.

Below is an example configuration with two destinations.

```yaml
kind: Bridge
name: bridge-example
destinations: ["pipeline1", "pipeline2"]
```

### Configuration

| Name         | Type     | Description                      | Required |
| ------------ | -------- | -------------------------------- | -------- |
| destinations | []string | Destination pipeline/proxy names | Yes      |

### Results

| Value                   | Description                          |
| ----------------------- | ------------------------------------ |
| destinationNotFound     | The desired destination is not found |
| invokeDestinationFailed | Failed to invoke the destination     |

## CORSAdaptor

The CORSAdaptor handles the [CORS](https://en.wikipedia.org/wiki/Cross-origin_resource_sharing) preflight request for backend service.

The below example configuration handles the preflight `GET` request from `*.megaease.com`.

```yaml
kind: CORSAdaptor
name: cors-adaptor-example
allowedOrigins: ["http://*.megaease.com"]
allowedMethods: [GET]
```

### Configuration

| Name             | Type     | Description                                                                                                                                                                                                                                                                                                                                                             | Required |
| ---------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| allowedOrigins   | []string | An array of origins a cross-domain request can be executed from. If the special `*` value is present in the list, all origins will be allowed. An origin may contain a wildcard (*) to replace 0 or more characters (i.e.: http://*.domain.com). Usage of wildcards implies a small performance penalty. Only one wildcard can be used per origin. Default value is `*` | No       |
| allowedMethods   | []string | An array of methods the client is allowed to use with cross-domain requests. The default value is simple methods (HEAD, GET, and POST)                                                                                                                                                                                                                                  | No       |
| allowedHeaders   | []string | An array of non-simple headers the client is allowed to use with cross-domain requests. If the special `*` value is present in the list, all headers will be allowed. The default value is [] but "Origin" is always appended to the list                                                                                                                               | No       |
| allowCredentials | bool     | Indicates whether the request can include user credentials like cookies, HTTP authentication, or client-side SSL certificates                                                                                                                                                                                                                                           | No       |
| exposedHeaders   | []string | Indicates which headers are safe to expose to the API of a CORS API specification                                                                                                                                                                                                                                                                                       | No       |

### Results

| Value       | Description                                                        |
| ----------- | ------------------------------------------------------------------ |
| preflighted | The request is a preflight one and has been processed successfully |

## Fallback

The Fallback filter mocks a response as fallback action of other filters. The below example configuration mocks the response with a specified status code, headers, and body.

```yaml
kind: Fallback
name: fallback-example
mockCode: 200
mockHeaders:
  Content-Type: application/json
mockBody: '{"message": "The feature turned off, please try it later."}'
```

### Configuration

| Name        | Type              | Description                                                                          | Required |
| ----------- | ----------------- | ------------------------------------------------------------------------------------ | -------- |
| mockCode    | int               | This code overwrites the status code of the original response                        | Yes      |
| mockHeaders | map[string]string | Headers to be added/set to the original response                                     | No       |
| mockBody    | string            | Default is an empty string, overwrite the body of the original response if specified | No       |

### Results

| Value    | Description                                                                  |
| -------- | ---------------------------------------------------------------------------- |
| fallback | The fallback steps have been executed, this filter always return this result |

## Mock

The Mock filter mocks responses according to configured rules, mainly for testing purposes.

Below is an example configuration to mock response for requests to path `/users/1` with specified status code, headers, and body, also with a 100ms delay to mock the time for request processing.

```yaml
kind: Mock
name: mock-example
rules:
- path: /users/1
  code: 200
  headers:
    Content-Type: application/json
  body: '{"name": "alice", "age": 30}'
  delay: 100ms
```

### Configuration

| Name  | Type                     | Description   | Required |
| ----- | ------------------------ | ------------- | -------- |
| rules | [][mock.Rule](#mockRule) | Mocking rules | Yes      |

### Results

| Value  | Description                                                       |
| ------ | ----------------------------------------------------------------- |
| mocked | The request matches one of the rules and response has been mocked |

## RemoteFilter

The RemoteFilter is a filter making remote service acting as an internal filter. It forwards original request & response information to the remote service and returns a result according to the response of the remote service.

The below example configuration forwards request & response information to `http://127.0.0.1:9096/verify`.

```yaml
kind: RemoteFilter
name: remote-filter-example
url: http://127.0.0.1:9096/verify
timeout: 500ms
```

### Configuration

| Name    | Type   | Description                            | Required |
| ------- | ------ | -------------------------------------- | -------- |
| url     | string | Address of remote service              | Yes      |
| timeout | string | Timeout duration of the remote service | No       |

### Results

| Value           | Description                                                                                   |
| --------------- | --------------------------------------------------------------------------------------------- |
| failed          | Failed to send the request to remote service, or remote service returns a non-2xx status code |
| responseAlready | The remote service returns status code 205                                                    |

## RequestAdaptor

The RequestAdaptor modifies the original request according to configuration.

The below example configuration adds prefix `/v3` to the request path.

```yaml
kind: RequestAdaptor
name: request-adaptor-example
path:
  addPrefix: /v3
```


The below example configuration removes header `X-Version` from all `GET` requests.

```yaml
kind: RequestAdaptor
name: request-adaptor-example
method: GET
header:
  del: ["X-Version"]
```

### Configuration

| Name   | Type                                         | Description                                                                                                                                                                                                         | Required |
| ------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| method | string                                       | If provided, the method of the original request is replaced by the value of this option                                                                                                                             | No       |
| path   | [pathadaptor.Spec](#pathadaptorSpec)         | Rules to revise request path                                                                                                                                                                                        | No       |
| header | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header                                                                                                                                                                                      | No       |
| body   | string                                       | If provided the body of the original request is replaced by the value of this option. Note: the body can be a template, which means runtime variables (enclosed by `[[` & `]]`) are replaced by their actual values | No       |
| host   | string                                       | If provided the host of the original request is replaced by the value of this option. Note: the host can be a template, which means runtime variables (enclosed by `[[` & `]]`) are replaced by their actual values | No       |

### Results

The RequestAdaptor always returns an empty result.

## CircuitBreaker

The CircuitBreaker is a finite state machine with three states: `CLOSED`, `OPEN`, and `HALF_OPEN`. When the state is `CLOSED`, requests pass through the CircuitBreaker normally, state transits to `OPEN` if request failure rate or slow request rate reach a configured threshold and the CircuitBreaker short-circuiting all requests in this state. After a configured duration, state transits from `OPEN` to `HALF_OPEN`, in which a limited number of requests are permitted to pass through the CircuitBreaker while other requests are still short-circuited, and state transit to `CLOSED` or `OPEN` based on the results of the permitted requests.

When `CLOSED`, the CircuitBreaker uses a sliding window to store and aggregate the result of recent requests, the window can either be `COUNT_BASED` or `TIME_BASED`. The `COUNT_BASED` window aggregates the last N requests and the `TIME_BASED` window aggregates requests in the last N seconds, where N is the window size.

Below is an example configuration with both `COUNT_BASED` and `TIME_BASED` policies. `GET` request to paths begin with `/books/` uses policy `count-based-example`, which short-circuits requests if more than half of recent requests failed with status code 500, 503, or 504. `GET` & `POST` requests to paths begin with `/users/` uses policy `time-based-example`, which short-circuits requests if more than 60% of recent requests failed.

```yaml
kind: CircuitBreaker
name: circuit-breaker-example
policies:
- name: count-based-example
  slidingWindowType: COUNT_BASED
  failureRateThreshold: 50
  slidingWindowSize: 100
  failureStatusCodes: [500, 503, 504]
- name: time-based-example
  slidingWindowType: TIME_BASED
  failureRateThreshold: 60
  slidingWindowSize: 100
  failureStatusCodes: [500, 503, 504]
urls:
- methods: [GET]
  url:
    prefix: /books/
  policyRef: count-based-example
- methods: [GET, POST]
  url:
    prefix: /users/
  policyRef: time-based-example
```

### Configuration

| Name             | Type                                             | Description                                                                                                                                                                                                           | Required |
| ---------------- | ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| policies         | [][circuitbreaker.Policy](#circuitbreakerPolicy) | Policy definitions                                                                                                                                                                                                    | Yes      |
| defaultPolicyRef | string                                           | The default policy, if no `policyRef` is configured in one of the `urls`, it uses this policy                                                                                                                         | No       |
| urls             | []resilience.URLRule                             | An array of request match criteria and policy to apply on matched requests. Note that a standalone CircuitBreaker instance is created for each item of the array, even two or more items can refer to the same policy | Yes      |

### Results

| Value          | Description                          |
| -------------- | ------------------------------------ |
| shortCircuited | The request has been short-circuited |

## RateLimiter

RateLimiter protects backend service for high availability and reliability by limiting the number of requests sent to the service in a configured duration.

Below example configuration limits `GET`, `POST`, `PUT`, `DELETE` requests to path which matches regular expression `^/pets/\d+$` to 50 per 10ms, and a request fails if it cannot be permitted in 100ms due to high concurrency requests count.

```yaml
kind: RateLimiter
name: rate-limiter-example
policies:
- name: policy-example
  timeoutDuration: 100ms
  limitRefreshPeriod: 10ms
  limitForPeriod: 50
defaultPolicyRef: policy-example
urls:
- methods: [GET, POST, PUT, DELETE]
  url:
    regex: ^/pets/\d+$
  policyRef: policy-example
```

### Configuration

| Name             | Type                                       | Description                                                                                                                                                                                                        | Required |
| ---------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------- |
| policies         | [][ratelimiter.Policy](#ratelimiterPolicy) | Policy definitions                                                                                                                                                                                                 | Yes      |
| defaultPolicyRef | string                                     | The default policy, if no `policyRef` is configured in one of the `urls`, it uses this policy                                                                                                                      | No       |
| urls             | [][resilience.URLRule](#resilienceURLRule) | An array of request match criteria and policy to apply on matched requests. Note that a standalone RateLimiter instance is created for each item of the array, even two or more items can refer to the same policy | Yes      |

### Results

| Value       | Description                                                |
| ----------- | ---------------------------------------------------------- |
| rateLimited | The request has been rejected as a result of rate limiting |

## TimeLimiter

TimeLimiter limits the time of requests, a request is canceled if it cannot get a response in configured duration.

The below example configuration marks a `POST` request to path `/users/1` as timed out if it cannot get a response in 500ms.

```yaml
kind: TimeLimiter
name: time-limiter-example
urls:
- methods: [POST]
  url:
    exact: /users/1
  timeoutDuration: 500ms
```

### Configuration

| Name                   | Type                                         | Description                                                                                                                        | Required |
| ---------------------- | -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | -------- |
| defaultTimeoutDuration | string                                       | The default timeout duration, if `timeoutDuration` is not configured in one of the `urls`, this duration is used. Default is 500ms | No       |
| urls                   | [][timelimiter.URLRule](#timelimiterURLRule) | An array of request match criteria and policy to apply on matched requests                                                         | Yes      |

### Results

| Value   | Description              |
| ------- | ------------------------ |
| timeout | The request is timed out |

## Retryer

Retryer retries failed requests according to configured policy.

Below example configuration retries `GET`, `POST`, `PUT`, `DELETE` requests to paths begin with `/books/` when response status code is 500, 503 or 504, max retry attempts is 3 and base wait duration between attempts is 500ms.

```yaml
kind: Retryer
name: retryer-example
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
```

### Configuration

| Name             | Type                               | Description                                                                                   | Required |
| ---------------- | ---------------------------------- | --------------------------------------------------------------------------------------------- | -------- |
| policies         | [][retryer.Policy](#retryerPolicy) | Policy definitions                                                                            | Yes      |
| defaultPolicyRef | string                             | The default policy, if no `policyRef` is configured in one of the `urls`, it uses this policy | No       |
| urls             | []resilience.URLRule               | An array of request match criteria and policy to apply on matched requests                    | Yes      |

### Results

The filter always returns the result of its succeeding filter, and the result of the last attempt is returned when there are two or more attempts.

## ResponseAdaptor

The ResponseAdaptor modifies the original response according to the configuration before passing it back.

Below is an example configuration that adds a header named `X-Response-Adaptor` with value `response-adaptor-example` to responses.

```yaml
kind: ResponseAdaptor
name: response-adaptor-example
header:
  add:
    X-Response-Adaptor: response-adaptor-example
```

### Configuration

| Name   | Type                                         | Description                                                                                                                                                                                                         | Required |
| ------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| header | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header                                                                                                                                                                                      | No       |
| body   | string                                       | If provided the body of the original request is replaced by the value of this option. Note: the body can be a template, which means runtime variables (enclosed by `[[` & `]]`) are replaced by their actual values | No       |

### Results

The filter always returns an empty result.

## Validator

The Validator filter validates requests, forwards valid ones, and rejects invalid ones. Four validation methods (`headers`, `jwt`, `signature`, and `oauth2`) are supported up to now, and these methods can either be used together or alone. When two or more methods are used together, a request needs to pass all of them to be forwarded.

Below is an example configuration for the `headers` validation method. Requests which has a header named `Is-Valid` with value `abc` or `goodplan` or matches regular expression `^ok-.+$` are considered to be valid.

```yaml
kind: Validator
name: header-validator-example
headers:
  Is-Valid:
    values: ["abc", "goodplan"]
    regexp: "^ok-.+$"
```

Below is an example configuration for the `jwt` validation method.

```yaml
kind: Validator
name: jwt-validator-example
jwt:
  cookieName: auth
  algorithm: HS256
  secret: 6d79736563726574
```

Below is an example configuration for the `signature` validation method, note multiple access key id/secret pairs can be listed in `accessKeys`, but there's only one pair here as an example.

```yaml
kind: Validator
name: signature-validator-example
signature:
  accessKeys:
    AKID: SECRET
```

Below is an example configuration for the `oauth2` validation method which uses a token introspection server for validation.

```yaml
kind: Validator
name: oauth2-validator-example
oauth2:
  tokenIntrospect:
    endPoint: https://127.0.0.1:8443/auth/realms/test/protocol/openid-connect/token/introspect
    clientId: easegress
    clientSecret: 42620d18-871d-465f-912a-ebcef17ecb82
    insecureTls: false
```

### Configuration

| Name      | Type                                                              | Description                                                                                                                                                                                                   | Required |
| --------- | ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| headers   | map[string][httpheader.ValueValidator](#httpheaderValueValidator) | Header validation rules, the key is the header name and the value is validation rule for corresponding header value, a request needs to pass all of the validation rules to pass the `headers` validation     | No       |
| jwt       | [validator.JWTValidatorSpec](#validatorJWTValidatorSpec)          | JWT validation rule, validates JWT token string from the `Authorization` header or cookies                                                                                                                    | No       |
| signature | [signer.Spec](#signerSpec)                                        | Signature validation rule, implements an [Amazon Signature V4](https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html) compatible signature validation validator, with customizable literal strings | No       |
| oauth2    | [validator.OAuth2ValidatorSpec](#validatorOAuth2ValidatorSpec)    | The `OAuth/2` method support `Token Introspection` mode and `Self-Encoded Access Tokens` mode, only one mode can be configured at a time                                                                      | No       |

### Results

| Value   | Description                         |
| ------- | ----------------------------------- |
| invalid | The request doesn't pass validation |

## WasmHost

The WasmHost filter implements a host environment for user-developed [WebAssembly](https://webassembly.org/) code. Below is an example configuration that loads wasm code from a file, and more details could be found at [this document](./wasmhost.md).

```yaml
name: wasm-host-example
kind: WasmHost
maxConcurrency: 2
code: /home/megaease/wasm/hello.wasm
timeout: 200ms
```

Note: this filter is disabled in the default build of `Easegress`, it can be enabled by:

```bash
$ make GOTAGS=wasmhost
```

or

```bash
$ go build -tags=wasmhost
```

### Configuration

| Name           | Type              | Description                                                                                     | Required |
| -------------- | ----------------- | ----------------------------------------------------------------------------------------------- | -------- |
| maxConcurrency | int32             | The maximum requests the filter can process concurrently. Default is 10 and minimum value is 1. | Yes      |
| code           | string            | The wasm code, can be the base64 encoded code, or path/url of the file which contains the code. | Yes      |
| timeout        | string            | Timeout for wasm execution, default is 100ms.                                                   | Yes      |
| parameters     | map[string]string | Parameters to initialize the wasm code.                                                         | No       |


### Results

| Value                                                                       | Description                                        |
| --------------------------------------------------------------------------- | -------------------------------------------------- |
| outOfVM                                                                     | Can not found an available wasm VM.                |
| wasmError                                                                   | An error occurs during the execution of wasm code. |
| wasmResult1 <td rowspan="3">Results defined and returned by wasm code.</td> |
| ...                                                                         |
| wasmResult9                                                                 |

## Common Types

### apiaggregator.Pipeline

| Name        | Type                                         | Description                                                                | Required |
| ----------- | -------------------------------------------- | -------------------------------------------------------------------------- | -------- |
| name        | string                                       | The name of target HTTP pipeline                                           | Yes      |
| method      | string                                       | Replaces request method with the value of this option when specified       | No       |
| path        | [pathadaptor.Spec](#pathadaptorSpec)         | Rules to revise request path                                               | No       |
| header      | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header                                             | No       |
| disableBody | bool                                         | Whether forwards the body of the original request or not, default is false | No       |

### pathadaptor.Spec

| Name         | Type                                                   | Description                                                                 | Required |
| ------------ | ------------------------------------------------------ | --------------------------------------------------------------------------- | -------- |
| replace      | string                                                 | Replaces request path with the value of this option when specified          | No       |
| addPrefix    | string                                                 | Prepend the value of this option to request path when specified             | No       |
| trimPrefix   | string                                                 | Trims the value of this option if request path start with it when specified | No       |
| regexReplace | [pathadaptor.RegexpReplace](#pathadaptorRegexpReplace) | Revise request path with regular expression                                 | No       |

### pathadaptor.RegexpReplace

| Name    | Type   | Description                                                                                                             | Required |
| ------- | ------ | ----------------------------------------------------------------------------------------------------------------------- | -------- |
| regexp  | string | Regular expression to match request path. The syntax of the regular expression is [RE2](https://golang.org/s/re2syntax) | Yes      |
| replace | string | Replacement when the match succeeds. Placeholders like `$1`, `$2` can be used to represent the sub-matches in `regexp`  | Yes      |

### httpheader.AdaptSpec

Rules to revise request header. Note that both header name and value can be a template, which means runtime variables (enclosed by `[[` & `]]`) are replaced by their actual values.

| Name | Type              | Description                         | Required |
| ---- | ----------------- | ----------------------------------- | -------- |
| del  | []string          | Name of the headers to be removed   | No       |
| set  | map[string]string | Name & value of headers to be set   | No       |
| add  | map[string]string | Name & value of headers to be added | No       |

### proxy.FallbackSpec

| Name        | Type              | Description                                                                             | Required |
| ----------- | ----------------- | --------------------------------------------------------------------------------------- | -------- |
| forCodes    | bool              | When true, fallback handles HTTP status code listed in `failureCodes`, default is false | No       |
| mockCode    | int               | Please refer the [Fallback](filters.md#Fallback) filter                                 | Yes      |
| mockHeaders | map[string]string | Please refer the [Fallback](filters.md#Fallback) filter                                 | No       |
| mockBody    | string            | Please refer the [Fallback](filters.md#Fallback) filter                                 | No       |

### proxy.PoolSpec

| Name            | Type                                   | Description                                                                                                  | Required |
| --------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| spanName        | string                                 | Span name for tracing, if not specified, the `url` of the target server is used                              | No       |
| serverTags      | []string                               | Server selector tags, only servers have tags in this array are included in this pool                         | No       |
| servers         | [][proxy.Server](#proxyServer)         | An array of static servers. If omitted, `serviceName` and `serviceRegistry` must be provided, and vice versa | No       |
| serviceName     | string                                 | This option and `serviceRegistry` are for dynamic server discovery                                           | No       |
| serviceRegistry | string                                 | This option and `serviceName` are for dynamic server discovery                                               | No       |
| loadBalance     | [proxy.LoadBalance](#proxyLoadBalance) | Load balance options                                                                                         | Yes      |
| memoryCache     | [memorycache.Spec](#memorycacheSpec)   | Options for response caching                                                                                 | No       |
| filter          | [httpfilter.Spec](#httpfilterSpec)     | Filter options for candidate pools                                                                           | No       |

### proxy.Server

| Name   | Type     | Description                                                                                                  | Required |
| ------ | -------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| url    | string   | Address of the server                                                                                        | Yes      |
| tags   | []string | Tags of this server, refer `serverTags` in [proxy.PoolSpec](#proxyPoolSpec)                                  | No       |
| weight | int      | When load balance policy is `weightedRandom`, this value is used to calculate the possibility of this server | No       |

### proxy.LoadBalance

| Name          | Type   | Description                                                                                                 | Required |
| ------------- | ------ | ----------------------------------------------------------------------------------------------------------- | -------- |
| policy        | string | Load balance policy, valid values are `roundRobin`, `random`, `weightedRandom`, `ipHash` ,and `headerHash`  | Yes      |
| headerHashKey | string | When `policy` is `headerHash`, this option is the name of a header whose value is used for hash calculation | No       |

### memorycache.Spec

| Name          | Type     | Description                                                                    | Required |
| ------------- | -------- | ------------------------------------------------------------------------------ | -------- |
| codes         | []int    | HTTP status codes to be cached                                                 | Yes      |
| expiration    | string   | Expiration duration of cache entries                                           | Yes      |
| maxEntryBytes | uint32   | Maximum size of the response body, response with a larger body is never cached | Yes      |
| methods       | []string | HTTP request methods to be cached                                              | Yes      |

### httpfilter.Spec

If `headers` criteria are configured, a request is filtered in if it matches both `headers` and `urls`.
If `headers` criteria are NOT configured, the `probability` options are used.

| Name        | Type                                                  | Description                                                                                                                 | Required |
| ----------- | ----------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | -------- |
| headers     | map[string][urlrule.StringMatch](#urlruleStringMatch) | Request header filter options. The key of this map is header name, and the value of this map is header value match criteria | No       |
| urls        | [][urlrule.URLRule](#urlruleURLRule)                  | Request URL match criteria                                                                                                  | No       |
| probability | [httpfilter.Probability](#httpfilterProbability)      | Options for filter in requests by probability                                                                               | No       |

### urlrule.StringMatch

The relationship between `exact`, `prefix`, and `regex` is `OR`.

| Name   | Type   | Description                                                                 | Required |
| ------ | ------ | --------------------------------------------------------------------------- | -------- |
| exact  | string | The string must be identical to the value of this field.                    | No       |
| prefix | string | The string must begin with the value of this field                          | No       |
| regex  | string | The string must the regular expression specified by the value of this field | No       |

### urlrule.URLRule

The relationship between `methods` and `url` is `AND`.

| Name    | Type                                       | Description                                                      | Required |
| ------- | ------------------------------------------ | ---------------------------------------------------------------- | -------- |
| methods | []string                                   | HTTP method criteria, Default is an empty list means all methods | No       |
| url     | [urlrule.StringMatch](#urlruleStringMatch) | Criteria to match a URL                                          | Yes      |

### resilience.URLRule

The relationship between `methods` and `url` is `AND`.

| Name      | Type                                       | Description                                                      | Required |
| --------- | ------------------------------------------ | ---------------------------------------------------------------- | -------- |
| methods   | []string                                   | HTTP method criteria, Default is an empty list means all methods | No       |
| url       | [urlrule.StringMatch](#urlruleStringMatch) | Criteria to match a URL                                          | Yes      |
| policyRef | string                                     | Name of resilience policy for matched requests                   | No       |

### httpfilter.Probability

| Name          | Type   | Description                                                                                                 | Required |
| ------------- | ------ | ----------------------------------------------------------------------------------------------------------- | -------- |
| perMill       | uint32 | Target filter in ratio, in per millage                                                                      | Yes      |
| policy        | string | Randomization policy, valid values are `ipHash`, `headerHash`, and `random`                                 | Yes      |
| headerHashKey | string | When `policy` is `headerHash`, this option is the name of a header whose value is used for hash calculation | No       |

### proxy.Compression

| Name      | Type | Description                                                                                   | Required |
| --------- | ---- | --------------------------------------------------------------------------------------------- | -------- |
| minLength | int  | Minimum response body size to be compressed, response with a smaller body is never compressed | Yes      |

### mock.Rule

| Name       | Type              | Description                                                                                                                                         | Required |
| ---------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| code       | int               | HTTP status code of the mocked response                                                                                                             | Yes      |
| path       | string            | Path match criteria, if request path is the value of this option, then the response of the request is mocked according to this rule                 | No       |
| pathPrefix | string            | Path prefix match criteria, if request path begins with the value of this option, then the response of the request is mocked according to this rule | No       |
| delay      | string            | Delay duration, for the request processing time mocking                                                                                             | No       |
| headers    | map[string]string | Headers of the mocked response                                                                                                                      | No       |
| body       | string            | Body of the mocked response, default is an empty string                                                                                             | No       |

### circuitbreaker.Policy

| Name                                  | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                                                              | Required |
| ------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| name                                  | string | Name of the policy. Must be unique in one CircuitBreaker configuration                                                                                                                                                                                                                                                                                                                                                                   | Yes      |
| slidingWindowType                     | string | Type of the sliding window which is used to record the outcome of requests when the CircuitBreaker is `CLOSED`. Sliding window can either be `COUNT_BASED` or `TIME_BASED`. If the sliding window is `COUNT_BASED`, the last `slidingWindowSize` requests are recorded and aggregated. If the sliding window is `TIME_BASED`, the requests of the last `slidingWindowSize` seconds are recorded and aggregated. Default is `COUNT_BASED` | No       |
| failureRateThreshold                  | int8   | Failure rate threshold in percentage. When the failure rate is equal to or greater than the threshold the CircuitBreaker transitions to `OPEN` and starts short-circuiting requests. Default is 50                                                                                                                                                                                                                                       | No       |
| slowCallRateThreshold                 | int8   | Slow rate threshold in percentage. The CircuitBreaker considers a request as slow when its duration is greater than `slowCallDurationThreshold`. When the percentage of slow requests is equal to or greater than the threshold, the CircuitBreaker transitions to `OPEN` and starts short-circuiting requests. Default is 100                                                                                                           | No       |
| countingNetworkError                  | bool   | Counting network error as failure or not. Default is false                                                                                                                                                                                                                                                                                                                                                                               | No       |
| slidingWindowSize                     | uint32 | The size of the sliding window which is used to record the outcome of requests when the CircuitBreaker is `CLOSED`. Default is 100                                                                                                                                                                                                                                                                                                       | No       |
| permittedNumberOfCallsInHalfOpenState | uint32 | The number of permitted requests when the CircuitBreaker is `HALF_OPEN`. Default is 10                                                                                                                                                                                                                                                                                                                                                   | No       |
| minimumNumberOfCalls                  | uint32 | The minimum number of requests which are required (per sliding window period) before the CircuitBreaker can calculate the error rate or slow requests rate. For example, if `minimumNumberOfCalls` is 10, then at least 10 requests must be recorded before the failure rate can be calculated. If only 9 requests have been recorded the CircuitBreaker will not transition to `OPEN` even if all 9 requests have failed. Default is 10 | No       |
| maxWaitDurationInHalfOpenState        | string | The maximum wait duration which controls the longest amount of time a CircuitBreaker could stay in `HALF_OPEN` state before it switches to `OPEN`. Value 0 means Circuit Breaker would wait infinitely in `HALF_OPEN` State until all permitted requests have been completed. Default is 0                                                                                                                                               | No       |
| waitDurationInOpenState               | string | The time that the CircuitBreaker should wait before transitioning from `OPEN` to `HALF_OPEN`. Default is 60s                                                                                                                                                                                                                                                                                                                             | No       |
| failureStatusCodes                    | []int  | HTTP status codes which need to be counting as failures                                                                                                                                                                                                                                                                                                                                                                                  | No       |

### ratelimiter.Policy

| Name               | Type   | Description                                                                                                                                                       | Required |
| ------------------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| name               | string | Name of the policy. Must be unique in one RateLimiter configuration                                                                                               | Yes      |
| timeoutDuration    | string | Maximum duration a request waits for permission to pass through the RateLimiter. The request fails if it cannot get permission in this duration. Default is 100ms | No       |
| limitRefreshPeriod | string | The period of a limit refresh. After each period the RateLimiter sets its permissions count back to the `limitForPeriod` value. Default is 10ms                   | No       |
| limitForPeriod     | int    | The number of permissions available in one `limitRefreshPeriod`. Default is 50                                                                                    | No       |

### timelimiter.URLRule

| Name            | Type                                       | Description                                                      | Required |
| --------------- | ------------------------------------------ | ---------------------------------------------------------------- | -------- |
| methods         | []string                                   | HTTP method criteria, Default is an empty list means all methods | No       |
| url             | [urlrule.StringMatch](#urlruleStringMatch) | Criteria to match a URL                                          | Yes      |
| timeoutDuration | string                                     | Timeout duration for matched requests. Default is 500ms          | No       |

### retryer.Policy

| Name                 | Type    | Description                                                                                                                                                                                                                                                      | Required |
| -------------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| name                 | string  | Name of the policy. Must be unique in one Retryer configuration                                                                                                                                                                                           | Yes      |
| countingNetworkError | bool    | Counting network error as failure or not. Default is false                                                                                                                                                                                                       | No       |
| failureStatusCodes   | []int   | HTTP status codes which need to be counting as failures                                                                                                                                                                                                          | No       |
| maxAttempts          | int     | The maximum number of attempts (including the initial one). Default is 3                                                                                                                                                                                         | No       |
| waitDuration         | string  | The base wait duration between attempts. Default is 500ms                                                                                                                                                                                                        | No       |
| backOffPolicy        | string  | The back-off policy for wait duration, could be `EXPONENTIAL` or `RANDOM` and the default is `RANDOM`. If configured as `EXPONENTIAL`, the base wait duration becomes 1.5 times larger after each failed attempt                                                 | No       |
| randomizationFactor  | float64 | Randomization factor for actual wait duration, a number in interval `[0, 1]`, default is 0. The actual wait duration used is a random number in interval `[(base wait duration) * (1 - randomizationFactor),  (base wait duration) * (1 + randomizationFactor)]` | No       |

### httpheader.ValueValidator

| Name   | Type     | Description                                                                                                                                                                      | Required |
| ------ | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| values | []string | An array of strings, if one of the header values of any header of the request is found in the array, the request is considered to pass the validation of current rule            | No       |
| regexp | string   | A regular expression, if one of the header values of any header of the request matches this regular expression, the request is considered to pass the validation of current rule | No       |

### validator.JWTValidatorSpec

| Name       | Type   | Description                                                                                                                                             | Required |
| ---------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| cookieName | string | The name of a cookie, if this option is set and the cookie exists, its value is used as the token string, otherwise, the `Authorization` header is used | No       |
| algorithm  | string | The algorithm for validation, `HS256`, `HS384`, and `HS512` are supported                                                                               | Yes      |
| secret     | string | The secret for validation, in hex encoding                                                                                                              | Yes      |

### signer.Spec

| Name        | Type                             | Description                                                               | Required |
| ----------- | -------------------------------- | ------------------------------------------------------------------------- | -------- |
| literal     | [signer.Literal](#signerLiteral) | Literal strings for customization, default value is used if omitted       | No       |
| excludeBody | bool                             | Exclude request body from the signature calculation, default is `false`   | No       |
| ttl         | string                           | Time to live of a signature, default is 0 means a signature never expires | No       |
| accessKeys  | map[string]string                | A map of access key id to access key secret                               | Yes      |

### signer.Literal

| Name             | Type   | Description                                                                                                                                        | Required |
| ---------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| scopeSuffix      | string | The last part to build credential scope, default is `megaease_request`, in `Amazon Signature V4`, it is `aws4_request`                             | No       |
| algorithmName    | string | The query name of the signature algorithm in the request, default is `X-Me-Algorithm`,  in `Amazon Signature V4`, it is `X-Amz-Algorithm`          | No       |
| algorithmValue   | string | The header/query value of the signature algorithm for the request, default is "ME-HMAC-SHA256", in `Amazon Signature V4`, it is `AWS4-HMAC-SHA256` | No       |
| signedHeaders    | string | The header/query headers of the signed headers, default is `X-Me-SignedHeaders`, in `Amazon Signature V4`, it is `X-Amz-SignedHeaders`             | No       |
| signature        | string | The query name of the signature, default is `X-Me-Signature`, in `Amazon Signature V4`, it is `X-Amz-Signature`                                    | No       |
| date             | string | The header/query name of the request time, default is `X-Me-Date`, in `Amazon Signature V4`, it is `X-Amz-Date`                                    | No       |
| expires          | string | The query name of expire duration, default is `X-Me-Expires`, in `Amazon Signature V4`, it is `X-Amz-Date`                                         | No       |
| credential       | string | The query name of credential, default is `X-Me-Credential`, in `Amazon Signature V4`, it is `X-Amz-Credential`                                     | No       |
| contentSha256    | string | The header name of body/payload hash, default is "X-Me-Content-Sha256", in `Amazon Signature V4`, it is `X-Amz-Content-Sha256`                     | No       |
| signingKeyPrefix | string | The prefix is prepended to access key secret when deriving the signing key, default is `ME`, in `Amazon Signature V4`, it is `AWS4`                | No       |

### validator.OAuth2ValidatorSpec

| Name            | Type                                                               | Description                                       | Required |
| --------------- | ------------------------------------------------------------------ | ------------------------------------------------- | -------- |
| tokenIntrospect | [validator.OAuth2TokenIntrospect](#validatorOAuth2TokenIntrospect) | Configuration for Token Introspection mode        | No       |
| jwt             | [validator.OAuth2JWT](#validatorOAuth2JWT)                         | Configuration for Self-Encoded Access Tokens mode | No       |

### validator.OAuth2TokenIntrospect

| Name         | Type   | Description                                                                                                                                                           | Required |
| ------------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| endPoint     | string | The endpoint of the token introspection server                                                                                                                        | Yes      |
| clientId     | string | Client id of Easegress in the token introspection server                                                                                                              | No       |
| clientSecret | string | Client secret of Easegress                                                                                                                                            | No       |
| basicAuth    | string | If `clientId` not specified and this option is specified, its value is used for basic authorization with the token introspection server                               | No       |
| insecureTls  | bool   | Whether the connection between Easegress and the token introspection server need to be secure or not, default is `false` means the connection need to be a secure one | No       |

### validator.OAuth2JWT

| Name      | Type   | Description                                                              | Required |
| --------- | ------ | ------------------------------------------------------------------------ | -------- |
| algorithm | string | The algorithm for validation, `HS256`, `HS384` and `HS512` are supported | Yes      |
| secret    | string | The secret for validation, in hex encoding                               | Yes      |

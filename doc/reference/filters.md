# Filters

- [Filters](#filters)
  - [Proxy](#proxy)
    - [Configuration](#configuration)
    - [Results](#results)
  - [SimpleHTTPProxy](#simplehttpproxy)
    - [Configuration](#configuration-1)
    - [Results](#results-1)
  - [WebSocketProxy](#websocketproxy)
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
  - [RequestBuilder](#requestbuilder)
    - [Configuration](#configuration-8)
    - [Results](#results-8)
  - [RateLimiter](#ratelimiter)
    - [Configuration](#configuration-9)
    - [Results](#results-9)
  - [ResponseAdaptor](#responseadaptor)
    - [Configuration](#configuration-10)
    - [Results](#results-10)
  - [ResponseBuilder](#responsebuilder)
    - [Configuration](#configuration-11)
    - [Results](#results-11)
  - [Validator](#validator)
    - [Configuration](#configuration-12)
    - [Results](#results-12)
  - [WasmHost](#wasmhost)
    - [Configuration](#configuration-13)
    - [Results](#results-13)
  - [Kafka](#kafka)
    - [Configuration](#configuration-14)
    - [Results](#results-14)
  - [HeaderToJSON](#headertojson)
    - [Configuration](#configuration-15)
    - [Results](#results-15)
  - [CertExtractor](#certextractor)
    - [Configuration](#configuration-16)
    - [Results](#results-16)
  - [HeaderLookup](#headerlookup)
    - [Configuration](#configuration-17)
    - [Results](#results-17)
  - [ResultBuilder](#resultbuilder)
    - [Configuration](#configuration-18)
    - [Results](#results-18)
  - [DataBuilder](#databuilder)
    - [Configuration](#configuration-19)
    - [Results](#results-19)
  - [OIDCAdaptor](#oidcadaptor)
    - [Configuration](#configuration-20)
    - [Results](#results-20)
  - [OPAFilter](#opafilter)
    - [Configuration](#configuration-21)
    - [Results](#results-21)
  - [Redirector](#redirector)
    - [Configuration](#configuration-22)
    - [Results](#results-22)
  - [GRPCProxy](#grpcproxy)
    - [Configuration](#configuration-23)
    - [Results](#results-23)
  - [Common Types](#common-types)
    - [pathadaptor.Spec](#pathadaptorspec)
    - [pathadaptor.RegexpReplace](#pathadaptorregexpreplace)
    - [httpheader.AdaptSpec](#httpheaderadaptspec)
    - [proxy.ServerPoolSpec](#proxyserverpoolspec)
    - [proxy.Server](#proxyserver)
    - [proxy.LoadBalanceSpec](#proxyloadbalancespec)
    - [proxy.StickySessionSpec](#proxystickysessionspec)
    - [proxy.HealthCheckSpec](#proxyhealthcheckspec)
    - [proxy.MemoryCacheSpec](#proxymemorycachespec)
    - [proxy.RequestMatcherSpec](#proxyrequestmatcherspec)
    - [grpcproxy.ServerPoolSpec](#grpcproxyserverpoolspec)
    - [grpcproxy.RequestMatcherSpec](#grpcproxyrequestmatcherspec)
    - [StringMatcher](#stringmatcher)
    - [proxy.MethodAndURLMatcher](#proxymethodandurlmatcher)
    - [urlrule.URLRule](#urlruleurlrule)
    - [proxy.Compression](#proxycompression)
    - [proxy.MTLS](#proxymtls)
    - [websocketproxy.WebSocketServerPoolSpec](#websocketproxywebsocketserverpoolspec)
    - [mock.Rule](#mockrule)
    - [mock.MatchRule](#mockmatchrule)
    - [ratelimiter.Policy](#ratelimiterpolicy)
    - [httpheader.ValueValidator](#httpheadervaluevalidator)
    - [validator.JWTValidatorSpec](#validatorjwtvalidatorspec)
    - [validator.BasicAuthValidatorSpec](#validatorbasicauthvalidatorspec)
    - [basicAuth.LDAPSpec](#basicauthldapspec)
    - [signer.Spec](#signerspec)
    - [signer.HeaderHoisting](#signerheaderhoisting)
    - [signer.Literal](#signerliteral)
    - [validator.OAuth2ValidatorSpec](#validatoroauth2validatorspec)
    - [validator.OAuth2TokenIntrospect](#validatoroauth2tokenintrospect)
    - [validator.OAuth2JWT](#validatoroauth2jwt)
    - [kafka.Topic](#kafkatopic)
    - [headertojson.HeaderMap](#headertojsonheadermap)
    - [headerlookup.HeaderSetterSpec](#headerlookupheadersetterspec)
    - [requestadaptor.SignerSpec](#requestadaptorsignerspec)
    - [Template Of Builder Filters](#template-of-builder-filters)
      - [HTTP Specific](#http-specific)

A Filter is a request/response processor. Multiple filters can be orchestrated
together to form a pipeline, each filter returns a string result after it
finishes processing the input request/response. An empty result means the
input was successfully processed by the current filter and can go forward
to the next filter in the pipeline, while a non-empty result means the pipeline
or preceding filter needs to take extra action.

## Proxy

The Proxy filter is a proxy of the backend service.

Below is one of the simplest Proxy configurations, it forward requests
to `http://127.0.0.1:9095`.

```yaml
kind: Proxy
name: proxy-example-1
pools:
- servers:
  - url: http://127.0.0.1:9095
```

Pool without `filter` is considered the main pool, other pools with `filter`
are considered candidate pools. Proxy first checks if one of the candidate
pools can process a request. For example, the first candidate pool in the
below configuration selects and processes requests with the header
`X-Candidate:candidate`, the second candidate pool randomly selects and
processes 400‰ of requests, and the main pool processes the other 600‰
of requests.

```yaml
kind: Proxy
name: proxy-example-2
pools:
- servers:
  - url: http://127.0.0.1:9095
  filter:
    headers:
      X-Candidate:
        exact: candidate
- servers:
  - url: http://127.0.0.1:9096
  filter:
    permil: 400 # between 0 and 1000
    policy: random
- servers:
  - url: http://127.0.0.1:9097
```

Servers of a pool can also be dynamically configured via service discovery,
the below configuration gets a list of servers by `serviceRegistry` &
`serviceName`, and only servers that have tag `v2` are selected.

```yaml
kind: Proxy
name: proxy-example-3
pools:
- serverTags: ["v2"]
  serviceName: service-001
  serviceRegistry: eureka-service-registry-example
```

When there are multiple servers in a pool, the Proxy can do a load balance
between them:

```yaml
kind: Proxy
name: proxy-example-4
pools:
- serverTags: ["v2"]
  serviceName: service-001
  serviceRegistry: eureka-service-registry-example
  loadBalance:
    policy: roundRobin
```

### Configuration
| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| pools | [proxy.ServerPoolSpec](#proxyserverpoolspec) | The pool without `filter` is considered the main pool, other pools with `filter` are considered candidate pools, and a `Proxy` must contain exactly one main pool. When `Proxy` gets a request, it first goes through the candidate pools, and if one of the pool's filter matches the request, servers of this pool handle the request, otherwise, the request is passed to the main pool. | Yes |
| mirrorPool | [proxy.ServerPoolSpec](#proxyserverpoolspec) | Define a mirror pool, requests are sent to this pool simultaneously when they are sent to candidate pools or main pool | No |
| compression | [proxy.Compression](#proxyCompression) | Response compression options | No |
| mtls | [proxy.MTLS](#proxymtls) | mTLS configuration | No |
| maxIdleConns | int | Controls the maximum number of idle (keep-alive) connections across all hosts. Default is 10240 | No |
| maxIdleConnsPerHost | int | Controls the maximum idle (keep-alive) connections to keep per-host. Default is 1024 | No |
| serverMaxBodySize | int64 | Max size of response body. the default value is 4MB. Responses with a body larger than this option are discarded.  When this option is set to `-1`, Easegress takes the response body as a stream and the body can be any size, but some features are not possible in this case, please refer [Stream](./stream.md) for more information. | No |

### Results

| Value         | Description                                            |
| ------------- | -------------------------------------------------------|
| internalError | Encounters an internal error                           |
| clientError   | Client-side (Easegress) network error                  |
| serverError   | Server-side network error                              |
| failureCode   | Resp failure code matches failureCodes set in poolSpec |

## SimpleHTTPProxy

The SimpleHTTPProxy filter is a simplified version of the Proxy filter, designed to handle HTTP requests in a more straightforward manner while providing basic proxy functionality for backend services.

The following example demonstrates a basic configuration for `SimpleHTTPProxy`. Unlike the `Proxy` filter, the backend service's address is not specified in the `SimpleHTTPProxy` configuration. Instead, the request URL is used directly, allowing for the use of a single `SimpleHTTPProxy` instance for multiple backend services.

```yaml
name: simple-http-proxy
kind: Pipeline
flow:
  - filter: requestBuilder
  - filter: proxy

filters:
  - kind: RequestBuilder
    name: requestBuilder
    template: |
      url: http://127.0.0.1:9095
      method: GET

  - kind: SimpleHTTPProxy
    name: proxy
```

### Configuration
| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| retryPolicy | string | Retry policy name | No |
| timeout | string | Request calceled when timeout | No |
| compression | [proxy.Compression](#proxyCompression) | Response compression options | No |
| maxIdleConns | int | Controls the maximum number of idle (keep-alive) connections across all hosts. Default is 10240 | No |
| maxIdleConnsPerHost | int | Controls the maximum idle (keep-alive) connections to keep per-host. Default is 1024 | No |
| serverMaxBodySize | int64 | Max size of response body. the default value is 4MB. Responses with a body larger than this option are discarded.  When this option is set to `-1`, Easegress takes the response body as a stream and the body can be any size, but some features are not possible in this case, please refer [Stream](./stream.md) for more information. | No |

### Results

| Value         | Description                                            |
| ------------- | -------------------------------------------------------|
| internalError | Encounters an internal error                           |
| clientError   | Client-side (Easegress) network error                  |
| serverError   | Server-side network error                              |

## WebSocketProxy

The WebSocketProxy filter is a proxy of the websocket backend service.

Below is one of the simplest WebSocketProxy configurations, it forwards
the websocket connection to `ws://127.0.0.1:9095`.

```yaml
kind: WebSocketProxy
name: proxy-example-1
pools:
- servers:
  - url: ws://127.0.0.1:9095
```

Same as the `Proxy` filter:
* a `filter` can be configured on a pool.
* the servers of a pool can be dynamically configured via service discovery.
* When there are multiple servers in a pool, the pool can do a load balance
  between them.

Note, when routing traffic to a pipeline with a `WebSocketProxy`, the
`HTTPServer` must set the corresponding `clientMaxBodySize` to `-1`, as
below:

```yaml
name: demo-server
kind: HTTPServer
port: 8080
rules:
- paths:
  path: /ws
  clientMaxBodySize: -1          # REQUIRED!
  backend: websocket-pipeline
```

### Configuration
| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| pools | [websocketproxy.WebSocketServerPoolSpec](#websocketproxywebsocketserverpoolspec) | The pool without `filter` is considered the main pool, other pools with `filter` are considered candidate pools, and a `Proxy` must contain exactly one main pool. When `WebSocketProxy` gets a request, it first goes through the candidate pools, and if it matches one of the pool's filter, servers of this pool handle the connection, otherwise, it is passed to the main pool. | Yes |

### Results

| Value         | Description                                            |
| ------------- | -------------------------------------------------------|
| internalError | Encounters an internal error                           |
| clientError   | Client-side network error                              |

## CORSAdaptor

The CORSAdaptor handles the [CORS](https://en.wikipedia.org/wiki/Cross-origin_resource_sharing) preflight, simple and not so simple request for the backend service.

The below example configuration handles the CORS `GET` request from `*.megaease.com`.

```yaml
kind: CORSAdaptor
name: cors-adaptor-example
allowedOrigins: ["http://*.megaease.com"]
allowedMethods: [GET]
```

### Configuration
| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| allowedOrigins | []string | An array of origins a cross-domain request can be executed from. If the special `*` value is present in the list, all origins will be allowed. An origin may contain a wildcard (*) to replace 0 or more characters (i.e.: http://*.domain.com). Usage of wildcards implies a small performance penalty. Only one wildcard can be used per origin. Default value is `*` | No |
| allowedMethods | []string | An array of methods the client is allowed to use with cross-domain requests. The default value is simple methods (HEAD, GET, and POST) | No |
| allowedHeaders | []string | An array of non-simple headers the client is allowed to use with cross-domain requests. If the special `*` value is present in the list, all headers will be allowed. The default value is [] but "Origin" is always appended to the list | No |
| allowCredentials | bool | Indicates whether the request can include user credentials like cookies, HTTP authentication, or client-side SSL certificates | No |
| exposedHeaders | []string | Indicates which headers are safe to expose to the API of a CORS API specification | No |
| maxAge | int | Indicates how long (in seconds) the results of a preflight request can be cached. The default is 0 stands for no max age | No |

### Results

| Value       | Description                                                         |
| ----------- | ------------------------------------------------------------------- |
| preflighted | The request is a preflight one and has been processed successfully. |
| rejected    | The request was rejected by CORS checking. |

## Fallback

The Fallback filter mocks a response as the fallback action of other filters.
The below example configuration mocks the response with a specified status
code, headers, and body.

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
| responseNotFound | No response found |

## Mock

The Mock filter mocks responses according to configured rules, mainly for
testing purposes.

Below is an example configuration to mock response for requests to path
`/users/1` with specified status code, headers, and body, also with a 100ms
delay to mock the time for request processing.

```yaml
kind: Mock
name: mock-example
rules:
- match:
    path: /users/1
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

The RemoteFilter is a filter making remote service act as an internal filter.
It forwards original request & response information to the remote service and
returns a result according to the response of the remote service.

The below example configuration forwards request & response information to
`http://127.0.0.1:9096/verify`.

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

The example configuration below adds prefix `/v3` to the request path.

```yaml
kind: RequestAdaptor
name: request-adaptor-example
path:
  addPrefix: /v3
```

The example configuration below removes header `X-Version` from all `GET` requests.

```yaml
kind: RequestAdaptor
name: request-adaptor-example
method: GET
header:
  del: ["X-Version"]
```

The example configuration below modifies the request path using regular expressions.

```yaml
kind: RequestAdaptor
name: request-adaptor-example
path:
  regexpReplace:
    regexp: "^/([a-z]+)/([a-z]+)" # groups /$1/$2 for lowercase alphabet
    replace: "/$2/$1" # changes the order of groups
```

The example configuration below signs the request using the
[Amazon Signature V4](https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html)
signing process, with the default configuration of this signing process.

```yaml
kind: RequestAdaptor
name: request-adaptor-example
path:
  signer:
    for: "aws4"
```

### Configuration

| Name       | Type                                         | Description                                                                                                                                                            | Required |
| -----------| -------------------------------------------- |------------------------------------------------------------------------------------------------------------------------------------------------------------------------| -------- |
| method     | string                                       | If provided, the method of the original request is replaced by the value of this option                                                                                | No       |
| path       | [pathadaptor.Spec](#pathadaptorSpec)         | Rules to revise request path                                                                                                                                           | No       |
| header     | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header                                                                                                                                         | No       |
| body       | string                                       | If provided the body of the original request is replaced by the value of this option.                                                                                  | No       |
| host       | string                                       | If provided the host of the original request is replaced by the value of this option.                                                                                  | No       |
| decompress | string                                       | If provided, the request body is replaced by the value of decompressed body. Now support "gzip" decompress                                                             | No       |
| compress   | string                                       | If provided, the request body is replaced by the value of compressed body. Now support "gzip" compress                                                                 | No       |
| sign   | [requestadaptor.SignerSpec](#requestadaptorsignerspec) | If provided, sign the request using the [Amazon Signature V4](https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html) signing process with the configuration | No       |
| template        | string | template to create request adaptor, please refer the [template](#template-of-builder-filters) for more information                                                       | No       |
| leftDelim       | string | left action delimiter of the template, default is `{{`                                                                                                                 | No       |
| rightDelim      | string | right action delimiter of the template, default is `}}`                                                                                                                | No       |

**NOTE**: template field takes higher priority than the static field with the same name.

### Results

| Value          | Description                              |
| -------------- | ---------------------------------------- |
| decompressFail | the request body can not be decompressed |
| compressFail   | the request body can not be compressed   |
| signFail       | the request body can not be signed   |

## RequestBuilder

The RequestBuilder creates a new request from existing requests/responses
according to the configuration, and saves the new request into [the
namespace it is bound](controllers.md#pipeline).

The example configuration below creates a reference to the request of
namespace `DEFAULT`.

```yaml
name: requestbuilder-example-1
kind: RequestBuilder
protocol: http
sourceNamespace: DEFAULT
```

The example configuration below creates an HTTP request with method `GET`,
url `http://127.0.0.1:8080`, header `X-Mock-Header:mock-value`, and body
`this is the body`.

```yaml
name: requestbuilder-example-1
kind: RequestBuilder
protocol: http
template: |
  method: get
  url: http://127.0.0.1:8080
  headers:
    X-Mock-Header:
    - mock-value
  body: "this is the body"
```

### Configuration

| Name            | Type   | Description                                   | Required |
|-----------------|--------|-----------------------------------------------|----------|
| protocol        | string | protocol of the request to build, default is `http`.  | No       |
| sourceNamespace | string | add a reference to the request of the source namespace    | No       |
| template        | string | template to create request, the schema of this option must conform with `protocol`, please refer the [template](#template-of-builder-filters) for more information        | No       |
| leftDelim       | string | left action delimiter of the template, default is `{{`  | No       |
| rightDelim      | string | right action delimiter of the template, default is `}}` | No       |

**NOTE**: `sourceNamespace` and `template` are mutually exclusive, you must
set one and only one of them.

### Results

| Value          | Description                              |
| -------------- | ---------------------------------------- |
| buildErr       | error happens when build request         |

## RateLimiter

RateLimiter protects backend service for high availability and reliability
by limiting the number of requests sent to the service in a configured duration.

Below example configuration limits `GET`, `POST`, `PUT`, `DELETE` requests
to path which matches regular expression `^/pets/\d+$` to 50 per 10ms, and a
request fails if it cannot be permitted in 100ms due to high concurrency
requests count.

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
| policies         | [][urlrule.URLRule](#urlruleURLRule) | Policy definitions                                                                                                                                                                                                  | Yes      |
| defaultPolicyRef | string                                     | The default policy, if no `policyRef` is configured in one of the `urls`, it uses this policy                                                                                                                      | No       |
| urls             | [][resilience.URLRule](#resilienceURLRule) | An array of request match criteria and policy to apply on matched requests. Note that a standalone RateLimiter instance is created for each item of the array, even two or more items can refer to the same policy | Yes      |

### Results

| Value       | Description                                                |
| ----------- | ---------------------------------------------------------- |
| rateLimited | The request has been rejected as a result of rate limiting |


## ResponseAdaptor

The ResponseAdaptor modifies the input response according to the configuration.

Below is an example configuration that adds a header named `X-Response-Adaptor`
with the value `response-adaptor-example` to the input response.

```yaml
kind: ResponseAdaptor
name: response-adaptor-example
header:
  add:
    X-Response-Adaptor: response-adaptor-example
```

### Configuration

| Name   | Type     | Description                                                                                                         | Required |
| ------ | -------- |---------------------------------------------------------------------------------------------------------------------| -------- |
| header | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header                                                                                      | No       |
| body   | string   | If provided the body of the original request is replaced by the value of this option.                               | No       |
| compress | string | compress body, currently only support gzip                                                                          | No |
| decompress | string | decompress body, currently only support gzip                                                                        | No |
| template        | string | template to create response adaptor, please refer the [template](#template-of-builder-filters) for more information | No       |
| leftDelim       | string | left action delimiter of the template, default is `{{`                                                              | No       |
| rightDelim      | string | right action delimiter of the template, default is `}}`                                                             | No       |

**NOTE**: template field takes higher priority than the static field with the same name.

### Results

| Value            | Description                                                |
| ---------------- | ---------------------------------------------------------- |
| responseNotFound | responseNotFound response is not found                     |
| decompressFailed | error happens when decompress body                         |
| compressFailed   | error happens when compress body                           |

## ResponseBuilder

The ResponseBuilder creates a new response from existing requests/responses
according to the configuration, and saves the new response into [the
namespace it is bound](controllers.md#pipeline).

The example configuration below creates a reference to the response of
namespace `DEFAULT`.

```yaml
name: responsebuilder-example-1
kind: ResponseBuilder
protocol: http
sourceNamespace: DEFAULT
```

The example configuration below creates an HTTP response with status code
200, header `X-Mock-Header:mock-value`, and body `this is the body`.

```yaml
name: responsebuilder-example-1
kind: ResponseBuilder
protocol: http
template: |
  statusCode: 200
  headers:
    X-Mock-Header:
    - mock-value
  body: "this is the body"
```

### Configuration

| Name            | Type   | Description                                   | Required |
|-----------------|--------|-----------------------------------------------|----------|
| protocol        | string | protocol of the response to build, default is `http`.  | No       |
| sourceNamespace | string | add a reference to the response of the source namespace    | No       |
| template        | string | template to create response, the schema of this option must conform with `protocol`, please refer the [template](#template-of-builder-filters) for more information        | No       |
| leftDelim       | string | left action delimiter of the template, default is `{{`  | No       |
| rightDelim      | string | right action delimiter of the template, default is `}}` | No       |

**NOTE**: `sourceNamespace` and `template` are mutually exclusive, you must
set one and only one of them.

### Results
| Value          | Description                              |
| -------------- | ---------------------------------------- |
| buildErr       | error happens when build response.         |

## Validator

The Validator filter validates requests, forwards valid ones, and rejects
invalid ones. Four validation methods (`headers`, `jwt`, `signature`, `oauth2`
and `basicAuth`) are supported up to now, and these methods can either be
used together or alone. When two or more methods are used together, a request
needs to pass all of them to be forwarded.

Below is an example configuration for the `headers` validation method.
Requests which has a header named `Is-Valid` with value `abc` or `goodplan`
or matches regular expression `^ok-.+$` are considered to be valid.

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

Below is an example configuration for the `signature` validation method,
note multiple access keys id/secret pairs can be listed in `accessKeys`,
but there's only one pair here as an example.

```yaml
kind: Validator
name: signature-validator-example
signature:
  accessKeys:
    AKID: SECRET
```

Below is an example configuration for the `oauth2` validation method which
uses a token introspection server for validation.

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

Here's an example of `basicAuth` validation method which uses
[Apache2 htpasswd](https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html)
formatted encrypted password file for validation.

```yaml
kind: Validator
name: basicAuth-validator-example
basicAuth:
  mode: "FILE"
  userFile: /etc/apache2/.htpasswd
```

### Configuration

| Name      | Type                                                              | Description                                                                                                                                                                                                   | Required |
| --------- | ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| headers   | map[string][httpheader.ValueValidator](#httpheaderValueValidator) | Header validation rules, the key is the header name and the value is validation rule for corresponding header value, a request needs to pass all of the validation rules to pass the `headers` validation     | No       |
| jwt       | [validator.JWTValidatorSpec](#validatorJWTValidatorSpec)          | JWT validation rule, validates JWT token string from the `Authorization` header or cookies                                                                                                                    | No       |
| signature | [signer.Spec](#signerSpec)                                        | Signature validation rule, implements an [Amazon Signature V4](https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html) compatible signature validation validator, with customizable literal strings | No       |
| oauth2    | [validator.OAuth2ValidatorSpec](#validatorOAuth2ValidatorSpec)    | The `OAuth/2` method support `Token Introspection` mode and `Self-Encoded Access Tokens` mode, only one mode can be configured at a time                                                                      | No       |
| basicAuth    | [validator.BasicAuthValidatorSpec](#validatorBasicAuthValidatorSpec)    | The `BasicAuth` method support `FILE`, `ETCD` and `LDAP` mode, only one mode can be configured at a time.                                                                  | No       |

### Results

| Value   | Description                         |
| ------- | ----------------------------------- |
| invalid | The request doesn't pass validation |

## WasmHost

The WasmHost filter implements a host environment for user-developed
[WebAssembly](https://webassembly.org/) code. Below is an example
configuration that loads wasm code from a file, and more details could be
found in [this document](./wasmhost.md).

```yaml
name: wasm-host-example
kind: WasmHost
maxConcurrency: 2
code: /home/megaease/wasm/hello.wasm
timeout: 200ms
```

Note: this filter is disabled in the default build of `Easegress`, it can
be enabled by:

```bash
$ make GOTAGS=wasmhost
```

or

```bash
$ make wasm
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



## Kafka

The Kafka filter converts HTTP Requests to Kafka messages and sends them to
the Kafka backend. The topic of the Kafka message comes from the HTTP header,
if not found, then the default topic will be used. The payload of the Kafka
message comes from the body of the HTTP Request.

Below is an example configuration.

```yaml
kind: Kafka
name: kafka-example
backend: [":9093"]
topic:
  default: kafka-topic
  dynamic:
    header: X-Kafka-Topic
```

### Configuration

| Name         | Type     | Description                      | Required |
| ------------ | -------- | -------------------------------- | -------- |
| backend | []string | Addresses of Kafka backend | Yes      |
| topic | [Kafka.Topic](#kafkatopic) | the topic is Spec used to get Kafka topic used to send message to the backend | Yes      |


### Results

| Value                   | Description                          |
| ----------------------- | ------------------------------------ |
| parseErr     | Failed to get Kafka message from the HTTP request |

## HeaderToJSON

The HeaderToJSON converts HTTP headers to JSON and combines it with the HTTP
request body. To use this filter, make sure your HTTP Request body is empty
or JSON schema.

Below is an example configuration.

```yaml
kind: HeaderToJSON
name: headertojson-example
headerMap:
  - header: X-User-Name
    json: username
  - header: X-Type
    json: type
```

### Configuration

| Name         | Type     | Description                      | Required |
| ------------ | -------- | -------------------------------- | -------- |
| headerMap | [][HeaderToJSON.HeaderMap](#headertojsonheadermap) | headerMap defines a map between HTTP header name and corresponding JSON field name | Yes      |


### Results

| Value                   | Description                             |
| ----------------------- | --------------------------------------- |
| jsonEncodeDecodeErr     | Failed to convert HTTP headers to JSON. |
| bodyReadErr             | Request body is stream                  |

## CertExtractor

CertExtractor extracts a value from requests TLS certificates Subject or
Issuer metadata (https://pkg.go.dev/crypto/x509/pkix#Name) and adds the
value to headers. Request can contain zero or multiple certificates so
the position (first, second, last, etc) of the certificate in the chain
is required.

Here's an example configuration, that adds a new header `tls-cert-postalcode`,
based on the PostalCode of the last TLS certificate's Subject:

```yaml
kind: "CertExtractor"
name: "postalcode-extractor"
certIndex: -1 # take last certificate in chain
target: "subject"
field: "PostalCode"
headerKey: "tls-cert-postalcode"
```

### Configuration

| Name         | Type     | Description                      | Required |
| ------------ | -------- | -------------------------------- | -------- |
| certIndex | int16 | The index of the certificate in the chain. Negative indexes from the end of the chain (-1 is the last index, -2 second last etc.) | Yes      |
| target | string | Either `subject` or `issuer` of the [x509.Certificate](https://pkg.go.dev/crypto/x509#Certificate) | Yes      |
| field | string | One of the string or string slice fields from https://pkg.go.dev/crypto/x509/pkix#Name  | Yes      |
| headerKey | string | Extracted value is added to this request header key. | Yes      |

### Results
The CertExtractor is always success and returns no results.

## HeaderLookup

HeaderLookup checks [custom data](customdata.md) stored in etcd and put them into HTTP header.

Suppose you create a custom data kind of `client-info` and post a data key `client1` with the value:
```yaml
name: client1
id: 123
kind: vip
```

Then HeaderLookup with the following configuration adds `X-Id:123` and `X-Kind:vip` to HTTP request header.
```yaml
name: headerlookup-example-1
kind: HeaderLookup
etcdPrefix: client-info # get custom data kind
headerKey: client1      # get custom data name
headerSetters:
- etcdKey: id           # custom data value of id
  headerKey: X-Id
- etcdKey: kind         # custom data value of kind
  headerKey: X-Kind
```

You can also use `pathRegExp` to check different keys for different requests. When `pathRegExp` is defined, `pathRegExp` is used with `regexp.FindStringSubmatch` to identify a group from the path. The first captured group is appended to the etcd key in the following format: `{headerKey's value}-{regex group}`.

Suppose you create a custom data kind of `client-info` and post several data:
```yaml
name: client-abc
id: 123
kind: vip

name: client-def
id: 124
kind: vvip
```

Then HeaderLookup with the following configuration adds `X-Id:123` and `X-Kind:vip` for requests with path `/api/abc`, adds `X-Id:124` and `X-Kind:vvip` for requests with path `/api/def`.
```yaml
name: headerlookup-example-1
kind: HeaderLookup
etcdPrefix: client-info # get custom data kind
headerKey: client      # get custom data name
pathRegExp: "^/api/([a-z]+)"
headerSetters:
- etcdKey: id           # custom data value of id
  headerKey: X-Id
- etcdKey: kind         # custom data value of kind
  headerKey: X-Kind
```

### Configuration
| Name | Type | Description | Required |
|------|------|-------------|----------|
| etcdPrefix | string | Kind of custom data | Yes |
| headerKey | string | Name of custom data in given kind | Yes |
| pathRegExp | string | Reg used to get key from request path | No |
| headerSetters | [][headerlookup.HeaderSetterSpec](#headerlookup.HeaderSetterSpec) | Set custom data value to http header | Yes |
### Results

HeaderLookup has no results.

## ResultBuilder

ResultBuilder generates a string, which will be the result of the filter. This
filter exists to work with the [`jumpIf` mechanism](./controllers.md#pipeline)
for conditional jumping.

Currently, the result string can only be `result0` - `result9`, this will be
changed in the future to allow arbitrary result string.

For example, we can use the following configuration to check if the request
body contains an `image` field, and forward it to `proxy1` or `proxy2`
conditionally.

```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: resultBuilder
  jumpIf:
    result1: proxy1
    result2: proxy2
- filter: proxy1
- filter: END
- filter: proxy2

filters:
- name: resultBuilder
  kind: ResultBuilder
  template: |
    {{- if .requests.DEFAULT.JSONBody.image}}result1{{else}}result2{{end -}}
```

### Configuration

| Name            | Type   | Description                                   | Required |
|-----------------|--------|-----------------------------------------------|----------|
| template        | string | template to create result, please refer the [template](#template-of-builer-filters) for more information        | No       |
| leftDelim       | string | left action delimiter of the template, default is `{{`  | No       |
| rightDelim      | string | right action delimiter of the template, default is `}}` | No       |


### Results

| Value                                                                       | Description                                        |
| --------------------------------------------------------------------------- | -------------------------------------------------- |
| unknown                                                                     | The ResultBuilder generates an unknown result.     |
| buildErr                                                                    | Error happens when build the result.               |
| result0 <td rowspan="3">Results defined and returned by the template .</td> |
| ...                                                                         |
| result9                                                                     |

## DataBuilder

DataBuilder is used to manipulate and store data. The data from the previous
filter can be transformed and stored in the context so that the data can be
used in subsequent filters.

The example below shows how to use DataBuilder to store the request body in
the context.

```yaml
- name: requestBodyDataBuilder
  kind: DataBuilder
  dataKey: requestBody
  template: |
    {{.requests.DEFAULT.JSONBody | jsonEscape}}
```

### Configuration

| Name            | Type   | Description                                   | Required |
|-----------------|--------|-----------------------------------------------|----------|
| template        | string | template to create data, please refer the [template](#template-of-builer-filters) for more information        | Yes      |
| dataKey         | string | key to store data        | Yes      |
| leftDelim       | string | left action delimiter of the template, default is `{{`  | No       |
| rightDelim      | string | right action delimiter of the template, default is `}}` | No       |


### Results

| Value           | Description                                       |
|-----------------|---------------------------------------------------|
| buildErr        | Error happens when building the data              |

## OIDCAdaptor

OpenID Connect(OIDC) is an identity layer on top of the OAuth 2.0 protocol. It enables Clients to verify
the identity of the End-User based on the authentication performed by an Authorization Server, as well as
to obtain basic profile information about the End-User.

For identity platforms that implement standard OIDC specification like [Google Accounts](https://accounts.google.com)、[OKTA](https://www.okta.com/)、
[Auth0](https://auth0.com/)、[Authing](https://www.authing.cn/).  configure `discovery` endpoint as below example:

```yaml
name: demo-pipeline
kind: Pipeline
flow:
  - filter: oidc
    jumpIf: { oidcFiltered: END }
filters:
  - name: oidc
    kind: OIDCAdaptor
    cookieName: oidc-auth-cookie
    clientId: <Your ClientId>
    clientSecret: <Your clientSecret>
    discovery: https://accounts.google.com/.well-known/openid-configuration  #Replace your own discovery
    redirectURI: /oidc/callback
```

For third platforms that only implement OAuth2.0 like GitHub, users should configure `authorizationEndpoint`、
`tokenEndpoint`、`userinfoEndpoint` at the same time as below example:

```yaml
name: demo-pipeline
kind: Pipeline
flow:
  - filter: oidc
    jumpIf: { oidcFiltered: END }
filters:
  - name: oidc
    kind: OIDCAdaptor
    cookieName: oidc-auth-cookie
    clientId: <Your ClientId>
    clientSecret: <Your clientSecret>
    authorizationEndpoint: https://github.com/login/oauth/authorize
    tokenEndpoint: https://github.com/login/oauth/access_token
    userinfoEndpoint: https://api.github.com/user
    redirectURI: /oidc/callback
```

### Configuration

| Name                  | Type   | Description                                                                                                               | Required |
|-----------------------|--------|---------------------------------------------------------------------------------------------------------------------------|----------|
| clientId              | string | The OAuth2.0 app client id                                                                                                | Yes      |
| clientSecret          | string | The OAuth2.0 app client secret                                                                                            | Yes      |
| cookieName            | string | Used to check if necessary to launch OpenIDConnect flow                                                                   | No       |
| discovery             | string | Standard OpenID Connect discovery endpoint URL of the identity server                                                     | No       |
| authorizationEndpoint | string | OAuth2.0 authorization endpoint URL                                                                                       | No       |
| tokenEndpoint         | string | OAuth2.0 token endpoint URL                                                                                               | No       |
| userInfoEndpoint      | string | OAuth2.0 user info endpoint URL                                                                                           | No       |
| redirectURI           | string | The callback uri registered in identity server, for example: <br/>`https://example.com/oidc/callback` or `/oidc/callback` | Yes      |

### Results
| Value           | Description                            |
|-----------------|----------------------------------------|
| oidcFiltered    | The request is handled by OIDCAdaptor. |


After OIDCAdaptor handled, following OIDC related information can be obtained from Easegress HTTP request headers:

* **X-User-Info**: Base64 encoded OIDC End-User basic profile.
* **X-Origin-Request-URL**: End-User origin request URL before OpenID Connect or OAuth2.0 flow.
* **X-Id-Token**: The ID Token returned by OpenID Connect flow.
* **X-Access-Token**: The AccessToken returned by OpenId Connect or OAuth2.0 flow.



## OPAFilter
The [Open Policy Agent (OPA)](https://www.openpolicyagent.org/docs/latest/) is an open source, 
general-purpose policy engine that unifies policy enforcement across the stack. It provides a 
high-level declarative language, which can be used to define and enforce policies in 
Easegress API Gateway. Currently, there are 160+ built-in operators and functions we can use, 
for examples `net.cidr_contains` and `contains`.

```yaml
name: demo-pipeline
kind: Pipeline
flow:
  - filter: opa-filter
    jumpIf: { opaDenied: END }
filters:
  - name: opa-filter
    kind: OPAFilter
    defaultStatus: 403
    readBody: true
    includedHeaders: a,b,c
    policy: |
      package http
      default allow = false
      allow {
         input.request.method == "POST"
         input.request.scheme == "https"
         contains(input.request.path, "/")               
         net.cidr_contains("127.0.0.0/24",input.request.realIP)          
      }
```

The following table lists input request fields that can be used in an OPA policy to help enforce it.

| Name                     | Type   | Description                                                           | Example                              |
|--------------------------|--------|-----------------------------------------------------------------------|--------------------------------------|
| input.request.method     | string | The current http request method                                       | "POST"                               |
| input.request.path       | string | The current http request URL path                                     | "/a/b/c"                             |
| input.request.path_parts | array  | The current http request URL path parts                               | ["a","b","c"]                        |
| input.request.raw_query  | string | The current http request raw query                                    | "a=1&b=2&c=3"                        |
| input.request.query      | map    | The current http request query map                                    | {"a":1,"b":2,"c":3}                  |
| input.request.headers    | map    | The current http request header map targeted by<br/> includedHeaders  | {"Content-Type":"application/json"}  |
| input.request.scheme     | string | The current http request scheme                                       | "https"                              | 
| input.request.realIP     | string | The current http request client real IP                               | "127.0.0.1"                          |
| input.request.body       | string | The current http request body string data                             | {"data":"xxx"}                       |


### Configuration

| Name             | Type   | Description                                                                          | Required |
|------------------|--------|--------------------------------------------------------------------------------------|----------|
| defaultStatus    | int    | The default HTTP status code when request is denied by the OPA policy decision       | No       |
| readBody         | bool   | Whether to read request body as OPA policy data on condition                         | No       |
| includedHeaders  | string | Names of the HTTP headers to be included in `input.request.headers`, comma-separated | No       |
| policy           | string | The OPA policy written in the Rego declarative language                              | Yes      |

### Results
| Value     | Description                                   |
|-----------|-----------------------------------------------|
| opaDenied | The request is denied by OPA policy decision. |


## Redirector

The `Redirector` filter is used to do HTTP redirect. `Redirector` matches request url, do replacement, and return response with status code of `3xx` and put new path in response header with key of `Location`.

Here a simple example: 
```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "^/users/([0-9]+)"
  replacement: "http://example.com/display?user=$1"
```
In this example, request with path `/users/123` will redirect to `http://example.com/display?user=123`.
```
HTTP/1.1 301 Moved Permanently
Location: http://example.com/display?user=123
```

More details about spec:

We use [ReplaceAllString](https://pkg.go.dev/regexp#Regexp.ReplaceAllString) to do match and replace and put output into response header with key `Location`. By default, we use `URI` as input, but you can change input by control parameter of `matchPart`.

```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "^/users/([0-9]+)"
  # by default, value of matchPart is uri, supported values: uri, path, full.
  matchPart: "full" 
  replacement: "http://example.com/display?user=$1"
```

For request with URL of `https://example.com:8080/apis/v1/user?id=1`, URI part is `/apis/v1/user?id=1`, path part is `/apis/v1/user` and full part is `https://example.com:8080/apis/v1/user?id=1`.

By default, we return status code of `301` "Moved Permanently". To return status code of `302` "Found" or other `3xx`, change `statusCode` in yaml. 

```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "^/users/([0-9]+)"
  # default value of 301, supported values: 301, 302, 303, 304, 307, 308.
  statusCode: 302
  replacement: "http://example.com/display?user=$1"
```

Following are some common used examples: 
1. URI prefix redirect
```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "^(.*)$"
  matchPart: "uri"
  replacement: "/prefix$1"
```

```
input: https://example.com/path/to/api/?key1=123&key2=456
output: /prefix/path/to/api/?key1=123&key2=456
```

URI prefix redirect with schema and host:
```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "(^.*\/\/)([^\/]*)(.*)$"
  matchPart: "full"
  replacement: "${1}${2}/prefix$3"
```
```
input: https://example.com/path/to/api/?key1=123&key2=456
output: https://example.com/prefix/path/to/api/?key1=123&key2=456
```

2. Domain Redirect
```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "(^.*\/\/)([^\/]*)(.*$)"
  matchPart: "full"
  # use ${1} instead of $1 here.
  replacement: "${1}my.com${3}"
```
```
input: https://example.com/path/to/api/?key1=123&key2=456
output: https://my.com/path/to/api/?key1=123&key2=456
```

3. Path Redirect
```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "/path/to/(user)\.php\?id=(\d*)"
  matchPart: "uri"
  replacement: "/api/$1/$2"
```
```
input: https://example.com/path/to/user.php?id=123
output: /api/user/123
```

Path redirect with schema and host:
```yaml
name: demo-pipeline
kind: Pipeline
flow:
- filter: redirector
filters:
- name: redirector
  kind: Redirector
  match: "(^.*\/\/)([^\/]*)/path/to/(user)\.php\?id=(\d*)"
  matchPart: "full"
  replacement: "${1}${2}/api/$3/$4"
```
```
input: https://example.com/path/to/user.php?id=123
output: https://example.com/api/user/123
```


### Configuration
| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| match | string | Regular expression to match request path. The syntax of the regular expression is [RE2](https://golang.org/s/re2syntax) | Yes |
| matchPart | string | Parameter to decide which part of url used to do match, supported values: uri, full, path. Default value is uri. | No |
| replacement | string | Replacement when the match succeeds. Placeholders like `$1`, `$2` can be used to represent the sub-matches in `regexp` | Yes | 
| statusCode | int | Status code of response. Supported values: 301, 302, 303, 304, 307, 308. Default: 301. | No | 
### Results
| Value | Description |
| ----- | ----------- |
| redirected | The request has been redirected |

## GRPCProxy

The `GRPCProxy` filter is a proxy for gRPC backend service. It supports both unary RPCs and streaming RPCs.

Below is one of the simplest `GRPCProxy` configurations, it forwards incoming gRPC connections to `127.0.0.1:9095`.

```yaml
kind: GRPCProxy
name: grpc-proxy-example-1
pools:
- servers:
  - url: http://127.0.0.1:9095
```

Same as the `Proxy` filter:

* a `filter` can be configured on a pool.
* the servers of a pool can be configured dynamically via service discovery.
* when there are multiple servers in a pool, the pool can do a load balance between them.

Note that each gRPC client establishes a connection with Easegress. However,
Easegress may utilize a single connection when forwarding requests from various
clients to a gRPC server, due to its use of HTTP2. This action could potentially
disrupt some client or server applications. For instance, if the client
applications are structured to directly connect to the server, and both the
client and server have the ability to request a connection closure, then
problems may arise once Easegress is installed between them. If the server
wants to close the connection of one client, it closes the shared connection
with Easegress, thus affecting other clients.

### Configuration

| Name         | Type                                                   | Description                                                                 | Required |
| ------------ | ------------------------------------------------------ | --------------------------------------------------------------------------- | -------- |
| pools               | [grpcproxy.ServerPoolSpec](#grpcproxyserverpoolspec) | The pool without `filter` is considered the main pool, other pools with `filter` are considered candidate pools, and a `GRPCProxy` must contain exactly one main pool. When a `GRPCProxy` gets a request, it first goes through the candidate pools, and if one of the pool's filter matches the request, servers of this pool handle the request, otherwise, the request is passed to the main pool. | Yes      |
| timeout             | string                                       | The total time from easegress receive request to receive response, default is never timeout, only apply to unary calls.                                                                                                                                              | No       |
| borrowTimeout       | string                                       | Timeout of borrow a connection from pool. Default is never timeout.                   | No       |
| connectTimeout      | string                                       | Timeout until a new connection is fully established.  Default is never timeout.       | No       |
| maxIdleConnsPerHost | int                                          | For a address, the maximum of connections allowed to create. Default value is 1024 | No       |

### Results

| Value          | Description                  |
|----------------|------------------------------|
| internalError  | Encounters an internal error |
| clientError    | Client-side error            |
| serverError    | Server-side error            |

## Common Types

### pathadaptor.Spec

| Name         | Type                                                   | Description                                                                 | Required |
| ------------ | ------------------------------------------------------ | --------------------------------------------------------------------------- | -------- |
| replace      | string                                                 | Replaces request path with the value of this option when specified          | No       |
| addPrefix    | string                                                 | Prepend the value of this option to request path when specified             | No       |
| trimPrefix   | string                                                 | Trims the value of this option if request path start with it when specified | No       |
| regexpReplace | [pathadaptor.RegexpReplace](#pathadaptorRegexpReplace) | Revise request path with regular expression                                 | No       |

### pathadaptor.RegexpReplace

| Name    | Type   | Description                                                                                                             | Required |
| ------- | ------ | ----------------------------------------------------------------------------------------------------------------------- | -------- |
| regexp  | string | Regular expression to match request path. The syntax of the regular expression is [RE2](https://golang.org/s/re2syntax) | Yes      |
| replace | string | Replacement when the match succeeds. Placeholders like `$1`, `$2` can be used to represent the sub-matches in `regexp`  | Yes      |

### httpheader.AdaptSpec

Rules to revise request header.

| Name | Type              | Description                         | Required |
| ---- | ----------------- | ----------------------------------- | -------- |
| del  | []string          | Name of the headers to be removed   | No       |
| set  | map[string]string | Name & value of headers to be set   | No       |
| add  | map[string]string | Name & value of headers to be added | No       |

### proxy.ServerPoolSpec

| Name            | Type                                   | Description                                                                                                  | Required |
| --------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| spanName        | string                                 | Span name for tracing, if not specified, the `url` of the target server is used                              | No       |
| serverTags      | []string                               | Server selector tags, only servers have tags in this array are included in this pool                         | No       |
| servers         | [][proxy.Server](#proxyServer)         | An array of static servers. If omitted, `serviceName` and `serviceRegistry` must be provided, and vice versa | No       |
| serviceName     | string                                 | This option and `serviceRegistry` are for dynamic server discovery                                           | No       |
| serviceRegistry | string                                 | This option and `serviceName` are for dynamic server discovery                                               | No       |
| loadBalance     | [proxy.LoadBalance](#proxyLoadBalanceSpec) | Load balance options                                                                                         | Yes      |
| memoryCache     | [proxy.MemoryCacheSpec](#proxymemorycachespec)   | Options for response caching                                                                                 | No       |
| filter          | [proxy.RequestMatcherSpec](#proxyrequestmatcherspec)     | Filter options for candidate pools                                                                           | No       |
| serverMaxBodySize | int64 | Max size of response body, will use the option of the Proxy if not set. Responses with a body larger than this option are discarded.  When this option is set to `-1`, Easegress takes the response body as a stream and the body can be any size, but some features are not possible in this case, please refer [Stream](./stream.md) for more information. | No |
| timeout | string | Request calceled when timeout | No |
| retryPolicy | string | Retry policy name | No |
| circuitBreakerPolicy | string | CircuitBreaker policy name | No |
| failureCodes | []int | Proxy return result of failureCode when backend resposne's status code in failureCodes. The default value is 5xx | No |


### proxy.Server

| Name   | Type     | Description                                                                                                  | Required |
| ------ | -------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| url    | string   | Address of the server. The address should start with `http://` or `https://` (when used in the `WebSocketProxy`, it can also start with `ws://` and `wss://`), followed by the hostname or IP address of the server, and then optionally followed by `:{port number}`, for example: `https://www.megaease.com`, `http://10.10.10.10:8080`. When host name is used, the `Host` of a request sent to this server is always the hostname of the server, and therefore using a [RequestAdaptor](#requestadaptor) in the pipeline to modify it will not be possible; when IP address is used, the `Host` is the same as the original request, that can be modified by a [RequestAdaptor](#requestadaptor). See also `KeepHost`.         | Yes      |
| tags   | []string | Tags of this server, refer `serverTags` in [proxy.PoolSpec](#proxyPoolSpec)                                  | No       |
| weight | int      | When load balance policy is `weightedRandom`, this value is used to calculate the possibility of this server | No       |
| keepHost | bool      | If true, the `Host` is the same as the original request, no matter what is the value of `url`. Default value is `false`. | No       |

### proxy.LoadBalanceSpec

| Name          | Type   | Description                                                                                                 | Required |
| ------------- | ------ | ----------------------------------------------------------------------------------------------------------- | -------- |
| policy        | string | Load balance policy, valid values are `roundRobin`, `random`, `weightedRandom`, `ipHash`, `headerHash` and `forward`, the last one is only used in `GRPCProxy`  | Yes      |
| headerHashKey | string | When `policy` is `headerHash`, this option is the name of a header whose value is used for hash calculation | No       |
| stickySession | [proxy.StickySession](#proxyStickySessionSpec) | Sticky session spec                                                 | No       |
| healthCheck | [proxy.HealthCheck](#proxyHealthCheckSpec) | Health check spec, note that healthCheck is not needed if you are using service registry | No       |
| forwardKey | string | The value of this field is a header name of the incoming request, the value of this header is address of the target server (host:port), and the request will be sent to this address | No |

### proxy.StickySessionSpec

| Name          | Type   | Description                                                                                                 | Required |
| ------------- | ------ | ----------------------------------------------------------------------------------------------------------- | -------- |
| mode          | string | Mode of session stickiness, support `CookieConsistentHash`,`DurationBased`,`ApplicationBased`                                 | Yes      |
| appCookieName | string | Name of the application cookie, its value will be used as the session identifier for stickiness in `CookieConsistentHash` and `ApplicationBased` mode             | No      |
| lbCookieName | string | Name of the cookie generated by load balancer, its value will be used as the session identifier for stickiness in `DurationBased` and `ApplicationBased` mode, default is `EG_SESSION`             | No      |
| lbCookieExpire | string | Expire duration of the cookie generated by load balancer, its value will be used as the session expire time for stickiness in `DurationBased` and `ApplicationBased` mode, default is 2 hours             | No      |

### proxy.HealthCheckSpec

| Name          | Type   | Description                                                                                                 | Required |
| ------------- | ------ | ----------------------------------------------------------------------------------------------------------- | -------- |
| interval | string | Interval duration for health check, default is 60s | Yes |
| path | string | Path URL for server health check | No |
| timeout | string | Timeout duration for health check, default is 3s | No |
| fails | int | Consecutive fails count for assert fail, default is 1 | No |
| passes | int | Consecutive passes count for assert pass , default is 1 | No |

### proxy.MemoryCacheSpec

| Name          | Type     | Description                                                                    | Required |
| ------------- | -------- | ------------------------------------------------------------------------------ | -------- |
| codes         | []int    | HTTP status codes to be cached                                                 | Yes      |
| expiration    | string   | Expiration duration of cache entries                                           | Yes      |
| maxEntryBytes | uint32   | Maximum size of the response body, response with a larger body is never cached | Yes      |
| methods       | []string | HTTP request methods to be cached                                              | Yes      |

### proxy.RequestMatcherSpec

Polices:
- If the policy is empty or `general`, matcher match requests with `headers` and `urls`.
- If the policy is `ipHash`, the matcher match requests if their IP hash value is less than `permil``.
- If the policy is `headerHash`, the matcher match requests if their header hash value is less than `permil`, use the key of `headerHashKey`.
- If the policy is `random`, the matcher matches requests with probability `permil`/1000.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| policy | string | Policy used to match requests, support `general`, `ipHash`, `headerHash`, `random` | No |
| headers     | map[string][StringMatcher](#stringmatcher) | Request header filter options. The key of this map is header name, and the value of this map is header value match criteria | No       |
| urls        | [][proxy.MethodAndURLMatcher](#proxyMethodAndURLMatcher)                  | Request URL match criteria                                                                                                  | No       |
| permil | uint32 | the probability of requests been matched. Value between 0 to 1000 | No       |
| matchAllHeaders | bool | All rules in headers should be match | No |
| headerHashKey | string | Used by policy `headerHash`. | No |

### grpcproxy.ServerPoolSpec

| Name            | Type                                   | Description                                                                                                  | Required |
| --------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| spanName        | string                                 | Span name for tracing, if not specified, the `url` of the target server is used                              | No       |
| serverTags      | []string                               | Server selector tags, only servers have tags in this array are included in this pool                         | No       |
| servers         | [][proxy.Server](#proxyServer)         | An array of static servers. If omitted, `serviceName` and `serviceRegistry` must be provided, and vice versa | No       |
| serviceName     | string                                 | This option and `serviceRegistry` are for dynamic server discovery                                           | No       |
| serviceRegistry | string                                 | This option and `serviceName` are for dynamic server discovery                                               | No       |
| loadBalance     | [proxy.LoadBalance](#proxyLoadBalanceSpec) | Load balance options                                                                                         | Yes      |
| filter          | [grpcproxy.RequestMatcherSpec](#grpcproxyrequestmatcherspec)     | Filter options for candidate pools                                                                           | No       |
| circuitBreakerPolicy | string | CircuitBreaker policy name | No |


### grpcproxy.RequestMatcherSpec

Polices:
- If the policy is empty or `general`, matcher match requests with `headers`, `urls` and `methods`.
- If the policy is `ipHash`, the matcher match requests if their IP hash value is less than `permil``.
- If the policy is `headerHash`, the matcher match requests if their header hash value is less than `permil`, use the key of `headerHashKey`.
- If the policy is `random`, the matcher matches requests with probability `permil`/1000.

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| policy | string | Policy used to match requests, support `general`, `ipHash`, `headerHash`, `random` | No |
| headers     | map[string][StringMatcher](#stringmatcher) | Request header filter options. The key of this map is header name, and the value of this map is header value match criteria | No       |
| urls        | [][proxy.MethodAndURLMatcher](#proxyMethodAndURLMatcher)                  | Request URL match criteria                                                                                                  | No       |
| permil | uint32 | the probability of requests been matched. Value between 0 to 1000 | No       |
| matchAllHeaders | bool | All rules in headers should be match | No |
| headerHashKey | string | Used by policy `headerHash`. | No |
| methods | [][StringMatcher](#stringmatcher) | Method name filter options. | No |


### StringMatcher

The relationship between `exact`, `prefix`, and `regex` is `OR`.

| Name   | Type   | Description                                                                 | Required |
| ------ | ------ | --------------------------------------------------------------------------- | -------- |
| exact  | string | The string must be identical to the value of this field.                    | No       |
| prefix | string | The string must begin with the value of this field                          | No       |
| regex  | string | The string must the regular expression specified by the value of this field | No       |
| empty | bool | The string must be empty | No |

### proxy.MethodAndURLMatcher

The relationship between `methods` and `url` is `AND`.

| Name    | Type                                       | Description                                                      | Required |
| ------- | ------------------------------------------ | ---------------------------------------------------------------- | -------- |
| methods | []string                                   | HTTP method criteria, Default is an empty list means all methods | No       |
| url     | [StringMatcher](#stringmatcher) | Criteria to match a  URL                                          | Yes      |

### urlrule.URLRule

The relationship between `methods` and `url` is `AND`.

| Name      | Type                                       | Description                                                      | Required |
| --------- | ------------------------------------------ | ---------------------------------------------------------------- | -------- |
| methods   | []string                                   | HTTP method criteria, Default is an empty list means all methods | No       |
| url       | [StringMatcher](#StringMatcher) | Criteria to match a URL                                          | Yes      |
| policyRef | string                                     | Name of resilience policy for matched requests                   | No       |


### proxy.Compression

| Name      | Type | Description                                                                                   | Required |
| --------- | ---- | --------------------------------------------------------------------------------------------- | -------- |
| minLength | int  | Minimum response body size to be compressed, response with a smaller body is never compressed | Yes      |

### proxy.MTLS
| Name           | Type   | Description                    | Required |
| -------------- | ------ | ------------------------------ | -------- |
| certBase64     | string | Base64 encoded certificate     | Yes      |
| keyBase64      | string | Base64 encoded key             | Yes      |
| rootCertBase64 | string | Base64 encoded root certificate | Yes      |

### websocketproxy.WebSocketServerPoolSpec

| Name            | Type                                   | Description                                                                                                  | Required |
| --------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------- |
| serverTags      | []string                               | Server selector tags, only servers have tags in this array are included in this pool                         | No       |
| servers         | [][proxy.Server](#proxyServer)         | An array of static servers. If omitted, `serviceName` and `serviceRegistry` must be provided, and vice versa | No       |
| serviceName     | string                                 | This option and `serviceRegistry` are for dynamic server discovery                                           | No       |
| serviceRegistry | string                                 | This option and `serviceName` are for dynamic server discovery                                               | No       |
| serverMaxMsgSize | int                                   | Max server message size, default is 32768.                                                                   | No       |
| clientMaxMsgSize | int                                   | Max client message size, default is 32768.                                                                   | No       |
| loadBalance     | [proxy.LoadBalance](#proxyLoadBalanceSpec) | Load balance options                                                                                     | Yes      |
| filter          | [proxy.RequestMatcherSpec](#proxyrequestmatcherspec)     | Filter options for candidate pools                                                         | No       |

### mock.Rule

| Name       | Type              | Description                                                                                                                                         | Required |
| ---------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| code       | int               | HTTP status code of the mocked response                                                                                                             | Yes      |
| match      | [MatchRule](#mock.MatchRule) | Rule to match a request        | Yes      |
| delay      | string            | Delay duration, for the request processing time mocking                                                                                             | No       |
| headers    | map[string]string | Headers of the mocked response                                                                                                                      | No       |
| body       | string            | Body of the mocked response, default is an empty string                                                                                             | No       |

### mock.MatchRule

| Name       | Type              | Description                                                                                                                                         | Required |
| ---------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| path       | string            | Path match criteria, if request path is the value of this option, then the response of the request is mocked according to this rule                 | No       |
| pathPrefix | string            | Path prefix match criteria, if request path begins with the value of this option, then the response of the request is mocked according to this rule | No       |
| matchAllHeaders | bool          | Whether to match all headers | No       |
| headers    | map[string][StringMatcher](#stringmatcher) | Headers to match, key is a header name, value is the rule to match the header value | No |


### ratelimiter.Policy

| Name               | Type   | Description                                                                                                                                                       | Required |
| ------------------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| name               | string | Name of the policy. Must be unique in one RateLimiter configuration                                                                                               | Yes      |
| timeoutDuration    | string | Maximum duration a request waits for permission to pass through the RateLimiter. The request fails if it cannot get permission in this duration. Default is 100ms | No       |
| limitRefreshPeriod | string | The period of a limit refresh. After each period the RateLimiter sets its permissions count back to the `limitForPeriod` value. Default is 10ms                   | No       |
| limitForPeriod     | int    | The number of permissions available in one `limitRefreshPeriod`. Default is 50                                                                                    | No       |

### httpheader.ValueValidator

| Name   | Type     | Description                                                                                                                                                                      | Required |
| ------ | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| values | []string | An array of strings, if one of the header values of any header of the request is found in the array, the request is considered to pass the validation of current rule            | No       |
| regexp | string   | A regular expression, if one of the header values of any header of the request matches this regular expression, the request is considered to pass the validation of current rule | No       |

### validator.JWTValidatorSpec

| Name       | Type   | Description                                                                                                                                            | Required |
|------------|--------|--------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| cookieName | string | The name of a cookie, if this option is set and the cookie exists, its value is used as the token string, otherwise, the `Authorization` header is used | No       |
| algorithm  | string | The algorithm for validation:`HS256`,`HS384`,`HS512`,`RS256`,`RS384`,`RS512`,`ES256`,`ES384`,`ES512`,`EdDSA` are supported                             | Yes      |
 | publicKey  | string | The public key is used for `RS256`,`RS384`,`RS512`,`ES256`,`ES384`,`ES512` or `EdDSA` validation in hex encoding                                       | Yes      |
| secret     | string | The secret is for `HS256`,`HS384`,`HS512` validation  in hex encoding                                                                                  | Yes      |

### validator.BasicAuthValidatorSpec

| Name         | Type   | Description                                                                                                                                           | Required |
|--------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| mode         | string | The mode of basic authentication, valid values are `FILE`, `ETCD` and `LDAP`                                             | Yes      |
| userFile     | string | The user file used for `FILE` mode                                               | No       |
| etcdPrefix   | string | The etcd prefix used for `ETCD` mode                                               | No       |
| ldap         | [basicAuth.LDAPSpec](#basicAuthLDAPSpec)   | The LDAP configuration used for `LDAP` mode                 | No       |

### basicAuth.LDAPSpec

| Name         | Type   | Description                                                             | Required |
|--------------|--------|-------------------------------------------------------------------------|----------|
| host         | string | The host of the LDAP server                                             | Yes      |
| port         | int    | The port of the LDAP server                                             | Yes      |
| baseDN       | string | The base dn of the LDAP server, e.g. `ou=users,dc=example,dc=org`       | Yes      |
| uid          | string | The user attribute used to bind user, e.g. `cn`                         | Yes      |
| useSSL       | bool   | Whether to use SSL                                                      | No       |
| skipTLS      | bool   | Whether to skip `StartTLS`                                              | No       |
| insecure     | bool   | Whether to skip verifying LDAP server's certificate chain and host name | No       |
| serverName   | string | Server name used to verify certificate when `insecure` is `false`       | No       |
| certBase64   | string | Base64 encoded certificate                                              | No       |
| keyBase64    | string | Base64 encoded key                                                      | No       |

### signer.Spec

| Name        | Type                             | Description                                                               | Required |
| ----------- | -------------------------------- | ------------------------------------------------------------------------- | -------- |
| literal     | [signer.Literal](#signerLiteral) | Literal strings for customization, default value is used if omitted       | No       |
| excludeBody | bool                             | Exclude request body from the signature calculation, default is `false`   | No       |
| ttl         | string                           | Time to live of a signature, default is 0 means a signature never expires | No       |
| accessKeys  | map[string]string                | A map of access key id to access key secret                               | Yes      |
| accessKeyId | string | ID used to set credential | No |
| accessKeySecret | string | Value usd to set credential | No |
| ignoredHeaders | []string | Headers to be ignored | No |
| headerHoisting | signer.HeaderHoisting | HeaderHoisting defines which headers are allowed to be moved from header to query in presign: header with name has one of the allowed prefixes, but hasn't any disallowed prefixes and doesn't match any of disallowed names are allowed to be hoisted | No |

### signer.HeaderHoisting

| Name             | Type     | Description                   | Required |
|------------------|----------|-------------------------------|----------|
| allowedPrefix    | []string | Allowed prefix for headers    | No       |
| disallowedPrefix | []string | Disallowed prefix for headers | No       |
| disallowed       | []string | Disallowed headers            | No       |

### signer.Literal

| Name             | Type   | Description                                                                                                                                        | Required |
| ---------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| scopeSuffix      | string | The last part to build credential scope, default is `request`, in `Amazon Signature V4`, it is `aws4_request`                             | No       |
| algorithmName    | string | The query name of the signature algorithm in the request, default is `X-Algorithm`,  in `Amazon Signature V4`, it is `X-Amz-Algorithm`          | No       |
| algorithmValue   | string | The header/query value of the signature algorithm for the request, default is "HMAC-SHA256", in `Amazon Signature V4`, it is `AWS4-HMAC-SHA256` | No       |
| signedHeaders    | string | The header/query headers of the signed headers, default is `X-SignedHeaders`, in `Amazon Signature V4`, it is `X-Amz-SignedHeaders`             | No       |
| signature        | string | The query name of the signature, default is `X-Signature`, in `Amazon Signature V4`, it is `X-Amz-Signature`                                    | No       |
| date             | string | The header/query name of the request time, default is `X-Date`, in `Amazon Signature V4`, it is `X-Amz-Date`                                    | No       |
| expires          | string | The query name of expire duration, default is `X-Expires`, in `Amazon Signature V4`, it is `X-Amz-Date`                                         | No       |
| credential       | string | The query name of credential, default is `X-Credential`, in `Amazon Signature V4`, it is `X-Amz-Credential`                                     | No       |
| contentSha256    | string | The header name of body/payload hash, default is `X-Content-Sha256`, in `Amazon Signature V4`, it is `X-Amz-Content-Sha256`                     | No       |
| signingKeyPrefix | string | The prefix is prepended to access key secret when deriving the signing key, default is an empty string, in `Amazon Signature V4`, it is `AWS4`                | No       |

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

### kafka.Topic

| Name      | Type   | Description                                                              | Required |
| --------- | ------ | ------------------------------------------------------------------------ | -------- |
| default | string | Default topic for Kafka backend | Yes      |
| dynamic.header | string | The HTTP header that contains Kafka topic | Yes      |

### headertojson.HeaderMap

| Name      | Type   | Description                                                              | Required |
| --------- | ------ | ------------------------------------------------------------------------ | -------- |
| header | string | The HTTP header that contains JSON value   | Yes      |
| json    | string | The field name to put JSON value into HTTP body | Yes      |


### headerlookup.HeaderSetterSpec
| Name | Type | Description | Required |
|------|------|-------------|----------|
| etcdKey | string | Key used to get data | No |
| headerKey | string | Key used to set data into http header | No |

### requestadaptor.SignerSpec

This type is derived from  [signer.Spec](#signerspec), with the following
two more fields.

| Name | Type | Description | Required |
|------|------|-------------|----------|
| apiProvider | string | The RequestAdaptor pre-defines the [Literal](#signerliteral) and [HeaderHoisting](#signerheaderhoisting) configuration for some API providers, specify the provider name in this field to use one of them, only `aws4` is supported at present. | No |
| scopes | []string | Scopes of the input request | No |

### Template Of Builder Filters

The content of the `template` field in the builder filters' spec is a
a template defined in Golang [text/template](https://pkg.go.dev/text/template),
with extra functions from the [sprig](https://go-task.github.io/slim-sprig/)
package, and extra functions defined by Easegress:

* **addf**: calculate the sum of the input two numbers.
* **subf**: calculate the difference of the two input numbers.
* **mulf**: calculate the product of the two input numbers.
* **divf**: calculate the quotient of the two input numbers.
* **log**: write a log message to Easegress log, the first argument must be
  `debug`, `info`, `warn` or `error`, and the second argument is the message.
* **mergeObject**: merge two or more objects into one, the type of the input
  objects must be `map[string]interface{}`, and if one of their field is
  also an object, its type must also be `map[string]interface{}`.
* **jsonEscape**: escape a string so that it can be used as the key or value
  in JSON text.

Easegress injects existing requests/responses of the current context into
the template engine at runtime, so we can use `.requests.<namespace>.<field>`
or `.responses.<namespace>.<field>` to read the information out (the
available fields vary from the protocol of the request or response,
and please refer [Pipeline](controllers.md#pipeline) for what is `namespace`).
For example, if the request of the `DEFAULT` namespace is an HTTP one, we
can access its method via `.requests.DEFAULT.Method`.

Easegress also injects other data into the template engine, which can be
accessed with `.data.<name>`, for example, we can use `.data.PIPELINE` to
read the data defined in the pipeline spec.

The `template` should generate a string in YAML format, the schema of the
result YAML varies from filters and protocols.

#### HTTP Specific

* **Available fields of existing requests**

  All exported fields of the [http.Request](https://pkg.go.dev/net/http#Request).
  And `RawBody` is the body as bytes; `Body` is the body as string; `JSONBody`
  is the body as a JSON object; `YAMLBody` is the body as a YAML object.

* **Available fields of existing responses**

  All exported fields of the [http.Response](https://pkg.go.dev/net/http#Response).
  And `RawBody` is the body as bytes; `Body` is the body as string; `JSONBody`
  is the body as a JSON object; `YAMLBody` is the body as a YAML object.

* **Schema of result request**

  | Name | Type | Description | Required |
  |------|------|-------------|----------|
  | method | string | HTTP Method of the result request, default is `GET`.  | No |
  | url | string | URL of the result request, default is `/`. | No |
  | headers | map[string][]string | Headers of the result request. | No |
  | body | string | Body of the result request. | No |
  | formData | map[string]field | Body of the result request, in form data pattern. | No |

  Please note `body` takes higher priority than `formData`, and the schema of
  `field` in `formData` is:

  | Name     | Type   | Description         | Required |
  |----------|--------|---------------------|----------|
  | value    | string | value of the field. | No       |
  | fileName | string | the file name, if value is the content of a file. | No |

* **Schema of result response**

  | Name | Type | Description | Required |
  |------|------|-------------|----------|
  | statusCode | int | HTTP status code, default is 200.  | No |
  | headers | map[string][]string | Headers of the result request. | No |
  | body | string | Body of the result request. | No |

* **Schema of RequestAdaptor**

| Name | Type | Description | Required |
|------|------|-------------|----------|
| method     | string                                       | If provided, the method of the original request is replaced by the value of this option                                                                                                                             | No       |
| path       | [pathadaptor.Spec](#pathadaptorSpec)         | Rules to revise request path                                                                                                                                                                                        | No       |
| header     | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header                                                                                                                                                                                      | No       |
| body       | string                                       | If provided the body of the original request is replaced by the value of this option. | No       |
| host       | string                                       | If provided the host of the original request is replaced by the value of this option. | No       |

* **Schema of ResponseAdaptor**

| Name | Type | Description | Required |
|------|------|-------------|----------|
| header | [httpheader.AdaptSpec](#httpheaderAdaptSpec) | Rules to revise request header    | No       |
| body   | string   | If provided the body of the original request is replaced by the value of this option. | No       |
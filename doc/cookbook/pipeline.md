# Pipeline 

- [Pipeline](#pipeline)
  - [Background](#background)
  - [Examples](#examples)
    - [Sequences executing](#sequences-executing)
    - [JumpIf](#jumpif)
    - [Built-in Filter `END`](#built-in-filter-end)
    - [Alias](#alias)
    - [Namespace](#namespace)
  - [References](#references)


## Background

* Easegress provides many useful filters, such as validator, request builder, and so on.
* The `Pipeline` is an Easegress build-in mechanism for orchestrating filters to handle requests.
* It supports calling multiple backend services for one input request.
* It supports conditional forward direction jumping.
* The `HTTPServer` receives incoming traffic and routes to one dedicated `Pipeline` in Easegress according to the HTTP header, path matching, etc.


## Examples

### Sequences executing

* The basic model of Pipeline execution is a sequence. Filters are executed one by one in the order described by the `flow` field in Pipeline's spec.

```
        Request
           │
┌──────────┼──────────┐
│ Pipeline │          │
│          │          │
│  ┌───────┴───────┐  │
│  │requestAdaptor │  │
│  └───────┬───────┘  │
│          │          │
│          │          │
│  ┌───────▼───────┐  │
│  │     Proxy     │  │
│  └───────┬───────┘  │
│          │          │
└──────────┼──────────┘
           │
           ▼
        Response
```

```bash
$ echo '
name: pipeline-demo
kind: Pipeline
flow:
- filter: requestAdaptor
- filter: proxy
filters:
- name: requestAdaptor
  kind: RequestAdaptor
  header:
    set:
      X-Adapt-Key: goodplan
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
' | egctl create -f -
```
* The `pipeline-demo` above will execute the `requestAdaptor` filter first, then `proxy`. So the `proxy` filter can forward the request with the header `X-Adapt-Key` which was set by the `requestAdaptor`.

### JumpIf

* Easegress' filter returns a string message as the execution result. If it's empty, that means these filters handle the request without failure. Otherwise, Easegress will use these no-empty result strings as the basis for the `JumpIf` mechanism.
* The Pipeline supports `JumpIf` mechanism[2]. We use it to avoid some chosen filters' execution when something goes wrong.

```
            Request
               │
               │
┌──────────────┼─────────────────┐
│ Pipeline     │                 │
│              ▼                 │
│     ┌──────────────────┐       │
│     │    Validator     ├────┐  │
│     └────────┬─────────┘    │  │
│              │              │  │
│              ▼              │  │
│     ┌──────────────────┐    │  │
│     │  RequestAdaptor  │    │  │
│     └────────┬─────────┘    │  │
│              │          Invalid│
│              ▼              │  │
│     ┌──────────────────┐    │  │
│     │     Proxy        │    │  │
│     └────────┬─────────┘    │  │
│              │              │  │
│              ◄──────────────┘  │
└──────────────┼─────────────────┘
               │
               ▼
            Response
```

```bash
$ echo '
name: pipeline-demo
kind: Pipeline

flow:
- filter: validator
  jumpIf: { invalid: END }
- filter: requestAdaptor
- filter: proxy

filters:
- name: validator
  kind: Validator
  headers:
    Content-Type:
      values:
      - application/json
- name: requestAdaptor
  kind: RequestAdaptor
  header:
    set:
      X-Adapt-Key: goodplan
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
' | egctl apply -f -
```

* As we can see above, `pipeline-demo` will jump to the end of pipeline execution when `validator`'s execution result is `invalid`.

### Built-in Filter `END`

* `END` is a built-in filter, that's we can use without defining it. From the above example, we already see that `END` can be used as the target of `jumpIf`, and we can also use it directly in the flow.
  
```bash
$ echo '
name: pipeline-demo
kind: Pipeline

flow:
- filter: validator
  jumpIf: { invalid: buildFailureResponse }
- filter: requestAdaptor
- filter: proxy
- filter: END
- filter: buildFailureResponse

filters:
- name: validator
  kind: Validator
  headers:
    Content-Type:
      values:
      - application/json
- name: requestAdaptor
  kind: RequestAdaptor
  header:
    set:
      X-Adapt-Key: goodplan
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
- name: responseBuilder
  kind: ResponseBuilder
  template: |
    statusCode: 400
    body: the request is invalid.
' | egctl apply -f -
```

* By using the `END` filter, we are now possible to build a custom failure response.

### Alias

* We have assigned a name to a filter when we define it, but a filter can be used more than once in the flow, in this case, we can assign an alias to each appearance.


```bash
$ echo '
name: pipeline-demo
kind: Pipeline

flow:
- filter: validator
  jumpIf:
    invalid: mockRequestForProxy2
- filter: mockRequest
- filter: proxy1
- filter: END
- filter: mockRequest
  alias: mockRequestForProxy2
- filter: proxy2

filters:
- name: validator
  kind: Validator
  headers:
    Content-Type:
      values:
      - application/json
- name: mockRequest
  kind: RequestBuilder
  template: |
    method: GET
    url: /hello
- name: proxy1
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
- name: proxy2
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.2:9095
' | egctl apply -f -
```

### Namespace

* The pipeline can handle multiple requests/responses, requests/responses are grouped by namespaces, and each namespace can have at most one request and one response.
* We can set the `namespace` attribute of filters to let the filter know which request/response it should use.
* `RequestBuilder` and `ResponseBuilder` can access all requests and responses, they output the result request/response to the namespace they are assigned.
* The default namespace is `DEFAULT`.

``` bash
echo '
name: pipeline-api
kind: Pipeline

flow:
- filter: copyRequest
  namespace: demo1
- filter: copyRequest
  namespace: demo2
- filter: copyRequest
  namespace: demo3
- filter: proxy-demo1
  namespace: demo1
- filter: proxy-demo2
  namespace: demo2
- filter: proxy-demo3
  namespace: demo3
- filter: buildResponse

filters:
- name: copyRequest
  kind: RequestBuilder
  sourceNamespace: DEFAULT
- name: proxy-demo1
  kind: Proxy
  pools:
  - servers:
    - url: https://demo1
- name: proxy-demo2
  kind: Proxy
  pools:
  - servers:
    - url: https://demo2
- name: proxy-demo3
  kind: Proxy
  pools:
  - servers:
    - url: https://demo3
- name: buildResponse
  kind: ResponseBuilder
  template: |
    statusCode: 200
    body: [{{.responses.demo1.Body}}, {{.response.demo2.Body}}, {{.response.demo3.Body}}]
' | egctl create -f -
```

## References
1. https://en.wikipedia.org/wiki/Chain-of-responsibility_pattern
2. https://github.com/megaease/easegress/blob/main/doc/developer-guide.md#jumpif-mechanism-in-pipeline

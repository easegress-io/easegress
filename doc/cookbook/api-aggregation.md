# API Aggregation

- [API Aggregation](#api-aggregation)
  - [Background](#background)
  - [Example](#example)
    - [Scenario 1: Simple aggregation](#scenario-1-simple-aggregation)
    - [Scenario 2: Merge response body](#scenario-2-merge-response-body)
    - [Scenario 3: Handle Failures](#scenario-3-handle-failures)
  - [References](#references)

## Background

* API aggregation is a pattern to aggregate multiple individual requests into
  a single request. This pattern is useful when a client must make multiple
  calls to different backend systems to operate.[1]
* Easegress provides filters [RequestBuilder](../reference/filters.md#requestbuilder)
  & [ResponseBuilder](../reference/filters.md#responsebuilder) for this
  powerful feature.

## Example

### Scenario 1: Simple aggregation

1. Suppose we have three backend services called  `demo1`,  `demo2`, and  `demo3`.
   We want to call these three services and combine their response to one.
   `demo1` returns `{"mega":"ease"}` as the HTTP response body, `demo2`
   returns `{"hello":"world"}`,  and `demo3` returns `{"hello":"new world"}`.

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
    body: |
      [{{.responses.demo1.Body}}, {{.responses.demo2.Body}}, {{.responses.demo3.Body}}]
' | egctl create -f -
```

2. Creating an HTTPServer for forwarding the traffic to this pipeline.

``` bash
echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /api
      backend: pipeline-api ' | egctl create -f -
```

3. Send a request to the pipeline

``` bash

$ curl  -X GET  http://127.0.0.1:10080/api -v
[{"hello":"world"},{"hello":"new world"},{"mega":"ease"}]

```

### Scenario 2: Merge response body

* In #Scenario 1,  `demo2` and `demo3`'s responses share the same JSON key,
  we want to merge their response body by the JSON key together. If there
  are duplicated keys, we will use the last value.

1. Update the pipeline spec

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
    body: |
      {{mergeObject .responses.demo1.JSONBody .responses.demo2.JSONBody .responses.demo3.JSONBody | toRawJson}}
' | egctl apply -f -
```

2. Send a request to the pipeline

``` bash

$ curl  -X GET  http://127.0.0.1:10080/api -v
{"hello":"new world","mega":"ease"}

```

### Scenario 3: Handle Failures

In #Scenario 1, if one of the backend services encounters a failure, the pipeline
will result a wrong result. We can improve the pipeline to handle this kind of
failures. 

1. Update the pipeline spec

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
  jumpIf: { serverError: proxy-demo2 }
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
    statusCode: {{$code := 503}}{{range $resp := .responses}}{{if eq $resp.StatusCode 200}}{{$code = 200}}{{break}}{{end}}{{end}}{{$code}}
    body: |
      {{$first := true -}}
      [{{range $resp := .responses -}}
        {{if eq $resp.StatusCode 200 -}}
          {{if not $first}},{{end}}{{$resp.Body}}{{$first = false -}}
        {{- end}}
      {{- end}}]
' | egctl apply -f -
```

2. Stop service `demo1` and send a request to the pipeline

``` bash
$ curl  -X GET  http://127.0.0.1:10080/api -v
[{"hello":"world"},{"hello":"new world"}]

```

## References

1. https://docs.microsoft.com/en-us/azure/architecture/patterns/gateway-aggregation

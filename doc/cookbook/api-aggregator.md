# API Aggregator

- [API Aggregator](#api-aggregator)
  - [Background](#background)
  - [API aggregator](#api-aggregator-1)
  - [Example](#example)
    - [Scenario 1: Run an aggregator in pipeline](#scenario-1-run-an-aggregator-in-pipeline)
    - [Scenario 2: Merge response body](#scenario-2-merge-response-body)
    - [Scenario 3: Allow partial succeed](#scenario-3-allow-partial-succeed)
  - [References](#references)

## Background

* API aggregation is a pattern to aggregate multiple individual requests into a single request. This pattern is useful when a client must make multiple calls to different backend systems to operate.[1]
* Easegress provides a filter called `APIAggregator` in the pipeline for this powerful feature.

## API aggregator

* Reusing the existing pipelines for an API request.
* Easy to integrate with other filters such as RateLimiter.

## Example

### Scenario 1: Run an aggregator in pipeline

1. We have three pipelines in the default namespace, called  `pipeline-demo`,  `pipeline-demo1`, and  `pipeline-demo2`. We want to call these three pipelines and combine their response with one request.
`pipeline-demo` will return `{"mega":"ease"}` in HTTP response body.
`pipeline-demo1` will return `{"hello":"world"}`.
`pipeline-demo2` will return `{"hello":"new world"}`.

``` bash
echo '
name: pipeline-api
kind: HTTPPipeline
flow:
  - filter: agg
filters:
  - name: agg
    kind: APIAggregator
    pipelines:
      - name: pipeline-demo
      - name: pipeline-demo1
      - name: pipeline-demo2 '  | egctl object create
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
      backend: pipeline-api | egctl object create

```

3. Request the aggregator pipeline

``` bash

$ curl  -X GET  http://127.0.0.1:10080/api -v
[{"hello":"world"},{"hello":"new world"},{"mega":"ease"}]

```

### Scenario 2: Merge response body

* As in #Scenario 1,  `pipeline-demo1` and `pipeline-demo2`'s responses share the same JSON key, we want to merge their response body by the JSON key together. If the keys have conflicted, we will use the last value.

1. Update the pipeline with the aggregator

``` bash
echo '
name: pipeline-api
kind: HTTPPipeline
flow:
  - filter: agg
filters:
  - name: agg
    kind: APIAggregator
    mergeResponse: true   // turn on this switch
    pipelines:
      - name: pipeline-demo
      - name: pipeline-demo1
      - name: pipeline-demo2 '  | egctl object update
```

2. Request the aggregator pipeline

``` bash

$ curl  -X GET  http://127.0.0.1:10080/api -v
{"hello":"new world","mega":"ease"}

```

### Scenario 3: Allow partial succeed

* As In #Scenario 1, if the backend service meets some problem or can't provide service, this aggregator will
fail as blew:

``` bash
$ curl http://localhost:10080/api -v -X PUT
*   Trying ::1...
* TCP_NODELAY set
* Connected to localhost (::1) port 10080 (#0)
> PUT /api HTTP/1.1
> Host: localhost:10080
> User-Agent: curl/7.64.1
> Accept: */*
>
< HTTP/1.1 503 Service Unavailable
< X-Eg-Aggregator: failed-in-pipeline-demo
< Date: Wed, 28 Jul 2021 09:11:18 GMT
< Content-Length: 0
<
* Connection #0 to host localhost left intact
* Closing connection 0
```

* We can see an `X-Eg-Aggregator: failed-in-pipeline-demo` header in the response, that's the first met failure pipeline of this aggregator.

* In some scenarios, we want this aggregator to return the successful execution pipelines' result. We can achieve this purpose by the steps below

1. Update the aggregator's spec

``` bash
echo '
name: pipeline-api
kind: HTTPPipeline
flow:
  - filter: agg
filters:
  - name: agg
    kind: APIAggregator
    partialSucceed: true   // turn on this switch
    pipelines:
      - name:  pipeline-demo
      - name:  pipeline-demo1
      - name: pipeline-demo2 '  | egctl object update

```

2. Request the aggregator

``` bash
$ curl  -X GET  http://127.0.0.1:10080/api -v
[{"hello":"world"},{"hello":"new world"}]

```

## References

1. https://docs.microsoft.com/en-us/azure/architecture/patterns/gateway-aggregation

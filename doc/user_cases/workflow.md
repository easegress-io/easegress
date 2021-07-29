# Workflow

- [Workflow](#workflow)
  - [Background](#background)
  - [Sequence workflow with HTTPTextTemplate](#sequence-workflow-with-httptexttemplate)
  - [Examples](#examples)
    - [Scenario 1: Response forwarding](#scenario-1-response-forwarding)
  - [Scenario 2:  Request forming](#scenario-2--request-forming)
  - [References](#references)

## Background

* A workflow consists of an orchestrated and repeatable pattern of activity, enabled by the systematic organization of resources into processes that transform materials, provide services, or process information. It can be depicted as a sequence of operations, the work of a person or group, the work of an organization of staff, or one or more simple or complex mechanisms.[1]
* Easegress currently has a build-in sequence workflow in Pipeline. Further more, we also provide a template mechanism for more advance usage.

## Sequence workflow with HTTPTextTemplate

* Already executed filter's metadata can be the input for next filter.
* Orchestrating pipeline with APIAggregator, RequestAdaptor and ResponseAdaptor.


## Examples

### Scenario 1: Response forwarding

* In Easegress, a pipeline usubally reperesent a perticular HTTPServer(maybe with several backends), APIAggregator can forward request to a dedicate pipeline. And we can use HTTPTextTemplate syntax to extract the respones and make it to be the input fot next pipeline with Aggregator.

All the characters are included here:

| Name           | Kind         | Description                                                                | Status        |
| -------------- | ------------ | -------------------------------------------------------------------------- | ------------- |
| server-demo    | HTTPServer   | an HTTPServer for receiving traffic                                        | already exist |
| pipeline-demo  | HTTPPipeline | a pipeline for forwarding request to backend                               | already exist |
| pipeline-demo1 | HTTPPipeline | a pipeline for forwarding request to backend1                              | already exist |
| pipeline-agg   | HTTPPipeline | a pipeline to orchestrator pipeline-demo and pipeline-demo1                | new added     |
| agg-demo       | filter       | an APIAggregatorfilter inside pipeline-agg for representing pipeline-demo  | new added     |
| agg-demo1      | filter       | an APIAggregatorfilter inside pipeline-agg for representing pipeline-demo1 | new added     |
| req-adaptor    | filter       | an RequestAdaptor for turning agg-demo's response into agg-demo1's request | new added     |

1. We want to use the response from `pipeline-demo` to be the request for `pipeline-demo1`, here is the configuration

``` yaml

name: pipeline-agg
kind: HTTPPipeline
flow:
  - filter: agg-demo
  - filter: req-adaptor
  - filter: agg-demo1

filters:
  - piplines:
      - name: pipeline-demo
    kind: APIAggregator
    name: agg-demo
  - name: req-adaptor
    kind: RequestAdaptor
    method: ""
    path: null
    header:
      del: {}
      set: {}
      add: {}
    body: "[[filter.pipeline-demo.rsp.body]]"
  - pipelines:
      - name: pipeline-demo1
    kind: APIAggregator
    name: agg-demo1

```

Create this pipeline-agg by using `egctl create -f `

2. Expose `pipeline-agg`' to one route in `server-demo`'s `/orchestractor` path.

``` bash
echo '
kind: HTTPServer
name: server-demo

#...

rules:
#...
  - paths:
    - pathPrefix: /orchestractor
      backend: pipeline-agg' | egctl.sh object update

```

## Scenario 2:  Request forming

* Extending from #Scenario 1, we want to select one particular JSON filed in agg-demo's response and named it with other name to be the input of agg-demo, and at last we want to combine agg-demo's response and agg-demo1's together. Let's check it out:

This time, we need to introduce another filter, ResponseAdaptor, for the final response forming.

``` yaml

name: pipeline-agg
kind: HTTPPipeline
flow:
  - filter: agg-demo
  - filter: req-adaptor
  - filter: agg-demo1
  - filter: rsp-adaptor

filters:
  - piplines:
      - name: pipeline-demo
    kind: APIAggregator
    name: agg-demo
  - name: req-adaptor
    kind: RequestAdaptor
    method: ""
    path: null
    header:
      del: {}
      set: {}
      add: {}
    body: "[[filter.pipeline-demo.rsp.body.key]]"
  - pipelines:
      - name: pipeline-demo1
    kind: APIAggregator
    name: agg-demo1
  - name: rsp-adaptor
    kind: ResponseAdaptor
    header:
      del: {}
      set: {}
      add: {}
    body: "{\"key-from-agg-demo\": \"[[filter.agg-demo.rsp.body.key]]\",\"value-from-agg-demo1\":\"[[filter.agg-demo1.rsp.body.value]]\"}"

```

* `[[filter.pipeline-demo.rsp.body.key]]` in req-adaptor will extract `key` filed from agg-demo's JSON response(actually, it's the response from pipeline-demo)
* `{\"key-from-agg-demo\": \"[[filter.agg-demo.rsp.body.key]]\",\"value-from-agg-demo1\":\"[[filter.agg-demo1.rsp.body.value]]\"}` in rsp-adaptor will combine agg-demo's JSON body's `key` filed and agg-demo1's JSON body's `value` together in the final HTTP response body of pipeline-agg
* The template syntax above support GJSON[2] in the last filed.

## References

[1] https://en.wikipedia.org/wiki/Workflow
[2] https://github.com/tidwall/gjson

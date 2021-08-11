# Pipeline 

- [Pipeline](#pipeline)
  - [Background](#background)
  - [Examples](#examples)
    - [Sequences executing](#sequences-executing)
    - [JumpIf](#jumpif)
  - [References](#references)


## Background

* Easegress provides many useful filters, such as rate limiter, validator, request adaptor, and so on.
* The `Pipeline` is an Easegress build-in mechanism for orchestrating HTTP filters for handling requests. It uses the `Chain of Responsibility Pattern`[1] and also supports conditional forward direction jumping.
* The `HTTPServer` receives incoming traffic and routes to one dedicated `Pipeline` in Easegress according to the HTTP header, path matching, etc.


## Examples

### Sequences executing

* The basic model of Pipeline execution is a sequence. Filters will be executed step by step in the order described by the `flow` field in Pipeline's spec.

```
      HTTP request
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
      HTTP response
```

```bash
$ echo '
name: pipeline-demo
kind: HTTPPipeline
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
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin' | egctl object create
```
* The `pipeline-demo` above will execute `rateLimiter` filter first, then `proxy`. So the `proxy` filter can forward the request with the header `X-Adapt-Key` setting by `requestAdaptor`.

### JumpIf

* Easegress' filter returns a string message as execution result. If it's empty, that means these filters handle the request without failure. Otherwise, Easegress will use these no-empty result strings as the basis for the `JumpIf` mechanism.
* The Pipeline supports `JumpIf` mechanism[2]. We use it to avoid some chosen filters' execution when something goes wrong.

```
         HTTP request
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
         HTTP response
```

```bash
$ cat pipeline-demo.yaml
name: pipeline-demo
kind: HTTPPipeline
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
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin

$ egctl object update -f pipeline-demo.yaml
```

* As we can see above, `pipeline-demo` will jump to the end of pipeline execution when `validator`'s execution result is `invalid`.

## References
1. https://en.wikipedia.org/wiki/Chain-of-responsibility_pattern
2. https://github.com/megaease/easegress/blob/main/doc/developer-guide.md#jumpif-mechanism-in-pipeline

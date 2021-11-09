# Distributed Tracing

Easegress tracing is based on [OpenTracing API](https://opentracing.io/) and officially supports [Zipkin](https://zipkin.io/). We can enable tracing in `HTTPServer` by defining `tracing` entry. Tracing will create spans containing the pipeline name, tracing service name (`tracing.serviceName`), HTTP path and HTTP method. The matched pipeline will start a child span, and its internal filters will start children spans according to their implementation. For example, the `Proxy` filter has specific span implementation.

```yaml
kind: HTTPServer
name: http-server-example
port: 10080
tracing:
  serviceName: httpServerExample
  zipkin:
    hostport: 0.0.0.0:10080
    serverURL: http://localhost:9412/api/v2/spans
    sampleRate: 1
    sameSpan: true
    id128Bit: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: http-pipeline-example
```

## Custom tags
Custom tags can help to furher filter and debug tracing spans. Here's an example with custom tag `customTagKey` with value `customTagValue`:

```yaml
kind: HTTPServer
name: http-server-example
port: 10080
tracing:
  serviceName: httpServerExample
  tags:                             # add "tags" entry and tags as key-value pairs
    customTagKey: customTagValue
  zipkin:
    hostport: 0.0.0.0:10080
    serverURL: http://localhost:9412/api/v2/spans
    sampleRate: 1
    sameSpan: true
    id128Bit: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: http-pipeline-example
```

# Distributed Tracing

Easegress tracing is based on [Zipkin](https://zipkin.io/). We can enable tracing in Traffic Gates, for example, in `HTTPServer`, we can do this by defining the `tracing` entry. Tracing creates spans containing the tracing service name (`tracing.serviceName`) and other information. The matched pipeline will start a child span, and its internal filters will start children spans according to their implementation and configuration. For example, the `Proxy` filter has a specific span implementation.

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
      backend: pipeline-example
```

## Custom tags

Custom tags can help to further filter and debug tracing spans. Here's an example with custom tag `customTagKey` with value `customTagValue`:

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
      backend: pipeline-example
```

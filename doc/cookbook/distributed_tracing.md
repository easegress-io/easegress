# Distributed Tracing

We used [OpenTracing API](https://opentracing.io/) to build the tracing framework of Easegress. We officially support [Zipkin](https://zipkin.io/).  We can enable tracing in `HTTPServer`, it will start a span whose name is the same ahs the server. The matched pipeline will start a child span, and its internal filters will start children spans according to their implementation, for example, the `Proxy` did it.

```yaml
kind: HTTPServer
name: http-server-example
port: 10080
tracing:
  serviceName: httpServerExmple
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
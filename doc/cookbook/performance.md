# Performance

- [Performance](#performance)
  - [Basic: Load Balance](#basic-load-balance)
  - [Performance: Compression and Caching](#performance-compression-and-caching)
    - [Compression in filter `Proxy`](#compression-in-filter-proxy)
    - [Caching in filter `Proxy`](#caching-in-filter-proxy)
    - [Caching in `HTTPServer`](#caching-in-httpserver)
  - [References](#references)
    - [Proxy Compression](#proxy-compression)
    - [Proxy Caching](#proxy-caching)
    - [HTTPServer route rule caching](#httpserver-route-rule-caching)

Easegress supports compression and caching in Pipeline and HTTPServer for performance improvement as a reverse proxy.


## Basic: Load Balance

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

## Performance: Compression and Caching

### Compression in filter `Proxy`

* Easegress proxy filter supports `gzip` type compression. It can save the bandwidth between the client and Easegress and reduce the time cost.

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
#...
    compression:
      minLength: 1024
#...
```

* As the example above, we only need to value the `minLength` field to tell Easegress' proxy filter compressing response body whose size is bigger than `minLength`. Also, it will add the `gzip` header automatically if it's truly compressed.

* For the full yaml, see [here](#proxy-compression)

### Caching in filter `Proxy`

* Easegress proxy filter has a `pool` section for describing the traffic forwarding backends. And it also supports caching the response according to the HTTP Methods and the HTTP response code. **Recommend enabling this feature only in the routing static resources scenario**.

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    pools:
#...
      memoryCache:
        expiration: 10s
        maxEntryBytes: 4096
        codes:
        - 200
        - 201
        methods:
        - GET
        - HEAD
#...
```

* The example above will cache the response which size is smaller than 4096, and the response code is 200 or 201, with the HTTP methods Get and Head.

* For the full YAML, see [here](#proxy-caching)

### Caching in `HTTPServer`

* As a traffic gate of Easegress, HTTPServer also supports caching routing rules. It reduces the HTTPServer route searching cost. HTTPServer will use the request's Host, Method, and Path to form a key for the build-in LRU rule cache.

```yaml
kind: HTTPServer
name: http-server-example
port: 10080
https: false
#...
cacheSize: 10240
#...
```

* As the example above, all we need is to set the `cacheSize` to indicated the LRU cache's size. It will disuse the least used cache rules firstly.

* For the full YAML, see [here](#httpserver-route-rule-caching)

## References
### Proxy Compression

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
      compression:
        minLength: 1024
```

### Proxy Caching

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
      memoryCache:
        expiration: 10s
        maxEntryBytes: 4096
        codes:
        - 200
        - 201
        methods:
        - GET
        - HEAD
```

### HTTPServer route rule caching

``` yaml
kind: HTTPServer
name: http-server-example
port: 10080
https: false
http3: false
keyBase64:
certBase64:
keepAlive: true
keepAliveTimeout: 75s
maxConnection: 10240
cacheSize: 10240
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: http-pipeline-example
    - pathPrefix: /remote
      backend: remote-pipeline
      headers:
      - key: X-Activity-No
        values: [ "123456", "124456" ]
        backend: remote-pipeline1
      - key: X-Activity-No
        values: [ "224456" ]
        regexp: ^224.*$
        backend: remote-pipeline2
      - key: X-Activity-No
        regexp: ^324.*$
        backend: remote-pipeline2
    - path: /func
      backend: func-mirror
```
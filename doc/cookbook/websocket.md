# WebSocket

- [WebSocket](#websocket)
  - [Background](#background)
  - [Design](#design)
  - [Example](#example)
  - [References](#references)

## Background

- Reverse proxy is one of most popular features of Easegress and it's suitable
  for many production and development scenarios.

- In reversed proxy use cases, `WebSocket` is a widely used protocol for a
  full-duplex communication solution between client and server. WebSocket
  relies on TCP.[1]

- Many reverse proxy support `WebSocket`,e.g., NGINX[2], Traefik, and so on.

## Design

- The `WebSocketProxy` is a filter of Easegress, and can be put into a pipeline.
- Easegress uses `github.com/golang/x/net/websocket` to implement
  `WebSocketProxy` filter.

1. Spec

* Pipeline with a `WebSocketProxy` filter:
 
```yaml
name: websocket-pipeline
kind: Pipeline

flow:
- filter: wsproxy

filters:
- kind: WebSocketProxy
  name: wsproxy
  defaultOrigin: http://127.0.0.1/hello
  pools:
  - servers:
    - url: ws://127.0.0.1:12345
 ```

* HTTPServer to route traffic to the websocket-pipeline:
 
```yaml
name: demo-server
kind: HTTPServer
port: 8080
rules:
- paths:
  - path: /ws
    clientMaxBodySize: -1          # REQUIRED!
    backend: websocket-pipeline
```

Note: `clientMaxBodySize` must be `-1`.

2. Request sequence

    ```none
    +--------------+                +--------------+                +--------------+  
    |              |  1             |              |   2            |              | 
    |   client     +--------------->|  Easegress   +--------------->|  websocket   |
    |              |<---------------+              |<---------------+  backend     | 
    |              |  4             |              |   3            |              |
    +--------------+                +--------------+                +--------------+
    ```

3. Headers

We copy all headers from your HTTP request to websocket backend, except
ones used by websocket package to build connection. Based on [3], we also
add `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto` to http
headers that send to websocket backend.

> note: websocket use `Upgrade`, `Connection`, `Sec-Websocket-Key`,
  `Sec-Websocket-Version`, `Sec-Websocket-Extensions` and
  `Sec-Websocket-Protocol` in http headers to set connection.

## Example

1. Send request

    ```bash
    curl --include \
        --no-buffer \
        --header "Connection: Upgrade" \
        --header "Upgrade: websocket" \
        --header "Host: 127.0.0.1:10020" \
        --header "Sec-WebSocket-Key: your-key-here" \
        --header "Sec-WebSocket-Version: 13" \
        http://127.0.0.1:8080/
    ```

2. This request to Easegress will be forwarded to websocket backend
   `ws://127.0.0.1:12345`.

## References

1. <https://datatracker.ietf.org/doc/html/rfc6455>
2. <https://www.nginx.com/blog/websocket-nginx/>
3. <https://docs.oracle.com/en-us/iaas/Content/Balance/Reference/httpheaders.htm>

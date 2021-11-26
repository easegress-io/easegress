# WebSocket

- [WebSocket](#websocket)
  - [Background](#background)
  - [Design](#design)
  - [References](#references)

## Background

- Easegress is popular for reverse proxy usage in production and development scenarios.

- In reversed proxy use cases, `WebSocket` is a widely used protocol for a full-duplex communication solution between client and server. It relies on TCP.[1]
- Many reverse proxy support `WebSocket`,e.g., NGINX[2], Traefik, and so on.

## Design

- Bring `WebSocketServer` as an BusinessController to Easegress.

- Using `github.com/gorilla/websocket` as the `WebSocket` client implementation, since it has rich features supported and a quite active community.（15k star/2.5k fork) Comparing to the original `golang.org/x/net/websocket` package, it can `receive fragmented message` and `send close message`.[3]

1. Spec

    Example1: `http` and `ws`

    ```yaml
    kind: WebSocketServer
    name: websocketSvr
    https: false                  # client need to use http/https firstly for connection upgrade      
    certBase64:
    keyBase64:
    port: 10020                   # proxy servers listening port

    backend: ws://localhost:3001  # the reserved proxy target
                                  #  Easegress will exame the backend URL's scheme, If it starts with `wss`,
                                  #  then `wssCerBase64` and `wssKeyBase64` must not be empty

    wssCertBase64:                # wss backend certificate in base64 format
    wssKeyBase64:                 # wss backend key in base64 format
    ```

    Example2: `https` and `wss`

    ```yaml
    kind: WebSocketServer
    name: websocketSvr
    https: true     
    certBase64: your-cert-base64
    keyBase64: your-key-base64
    port: 10020
    backend: wss://localhost:3001
    wssCertBase64: your-cert-wss-base64
    wssKeyBase64: your-key-wss-base64
    ```

2. Request sequence

    ```none
    +--------------+                +--------------+                +--------------+  
    |              |  1             |              |   2            |              | 
    |   client     +--------------->|  Easegress   +--------------->|  websocket   |
    |              |<---------------+(WebSocketSvr)|<---------------+  backend     | 
    |              |  4             |              |   3            |              |
    +--------------+                +--------------+                +--------------+
    ```

3. Headers

    We copy all headers from your HTTP request to websocket backend, except ones used by `gorilla` package to build connection. Based on [4], we also add `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto` to http headers that send to websocket backend.

> note: `gorilla` use `Upgrade`, `Connection`, `Sec-Websocket-Key`, `Sec-Websocket-Version`, `Sec-Websocket-Extensions` and `Sec-Websocket-Protocol` in http headers to set connection.

## References

1. <https://datatracker.ietf.org/doc/html/rfc6455>
2. <https://www.nginx.com/blog/websocket-nginx/>
3. <https://github.com/gorilla/websocket>
4. <https://docs.oracle.com/en-us/iaas/Content/Balance/Reference/httpheaders.htm>
